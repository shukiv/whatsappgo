package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/mediaformat"
	"github.com/shukiv/whatsappgo/internal/mediastore"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/notify"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func (c *Client) handleEvent(raw any) {
	switch evt := raw.(type) {
	case *waEvents.Connected:
		c.setStatus(func(s *model.ConnectionStatus) {
			s.State = "connected"
			s.Connected = true
			s.LoggedIn = c.wa.Store.ID != nil
			s.LastError = ""
			if c.wa.Store.ID != nil {
				s.UserJID = c.wa.Store.ID.String()
				s.UserName = c.wa.Store.PushName
			}
		})
		// Extracting previews only reads local data, so it runs on its own
		// rather than queueing behind directory and app-state synchronisation,
		// which wait on the network and can take minutes.
		go c.backfillThumbnails()
		go c.backfillLinkPreviews()
		// Reconnecting is routine on a flaky network, and a sweep can run for
		// hours. Without this guard a second sweep races the first: its page
		// requests are suppressed as duplicates, it waits for answers that
		// were never requested on its behalf, and it concludes conversations
		// are exhausted, marking them finished for good.
		if !c.beginConnectSweep() {
			return
		}
		go func() {
			defer c.endConnectSweep()
			_ = c.wa.SendPresence(context.Background(), types.PresenceAvailable)
			c.syncDirectory()
			if err := c.store.RecalculateUnreadCounts(context.Background()); err == nil {
				c.emit(gateway.Event{Name: "chat.updated"})
			}
			c.backfillCallLogs()
			c.backfillChatSettings()
			go c.backfillReactions()
			// Older messages and their attachments are collected side by
			// side. Queueing the files behind the whole history would leave
			// pictures missing for hours; both streams are slow enough on
			// their own that running them together is still gentle.
			ctx := c.baseCtx
			if ctx == nil {
				ctx = context.Background()
			}
			go c.collectMedia(ctx)
			c.collectHistory(ctx)
		}()
	case *waEvents.Disconnected:
		c.setStatus(func(s *model.ConnectionStatus) { s.State = "disconnected"; s.Connected = false })
	case *waEvents.LoggedOut:
		c.setStatus(func(s *model.ConnectionStatus) {
			s.State = "logged_out"
			s.Connected = false
			s.LoggedIn = false
			s.UserJID = ""
		})
	case *waEvents.PairSuccess:
		c.setStatus(func(s *model.ConnectionStatus) { s.State = "paired"; s.LoggedIn = true; s.UserJID = evt.ID.String() })
	case *waEvents.Message:
		c.handleMessage(evt)
	case *waEvents.Receipt:
		c.handleReceipt(evt)
	case *waEvents.MediaRetry:
		c.handleMediaRetry(evt)
	case *waEvents.ChatPresence:
		chatJID := evt.Chat.String()
		if c.store != nil {
			chatJID = c.store.CanonicalChatJID(context.Background(), chatJID)
		}
		c.emit(gateway.Event{Name: "chat.presence", Data: map[string]any{"chat_jid": chatJID, "sender_jid": evt.Sender.String(), "state": string(evt.State), "media": string(evt.Media)}})
	case *waEvents.Presence:
		jid := evt.From.String()
		if c.store != nil {
			jid = c.store.CanonicalChatJID(context.Background(), jid)
		}
		lastSeen := int64(0)
		if !evt.LastSeen.IsZero() {
			lastSeen = evt.LastSeen.UnixMilli()
		}
		if lastSeen > 0 && c.store != nil {
			_ = c.store.UpdateChatLastSeen(context.Background(), jid, lastSeen)
		}
		c.emit(gateway.Event{Name: "contact.presence", Data: map[string]any{"jid": jid, "unavailable": evt.Unavailable, "last_seen": lastSeen}})
	case *waEvents.HistorySync:
		go c.handleHistorySync(evt)
	case *waEvents.AppState:
		c.handleAppState(evt)
	case *waEvents.AppStateSyncComplete:
		c.handleAppStateSyncComplete(evt)
	case *waEvents.Pin:
		if evt.Action != nil {
			_ = c.store.UpdateChatPinnedAt(context.Background(), evt.JID.String(), evt.Action.GetPinned(), evt.Timestamp.UnixMilli())
			c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": evt.JID.String(), "pinned": evt.Action.GetPinned()}})
		}
	case *waEvents.Star:
		if evt.Action != nil {
			// A star set on the phone or another linked device only reaches us
			// as this app-state event, so without it the flag would be
			// write-only from this client's side.
			_ = c.store.SetMessageStarred(context.Background(), evt.ChatJID.String(), evt.MessageID, evt.Action.GetStarred())
			c.emit(gateway.Event{Name: "message.starred", Data: map[string]any{
				"chat_jid": evt.ChatJID.String(), "message_id": evt.MessageID, "starred": evt.Action.GetStarred(),
			}})
		}
	case *waEvents.MarkChatAsRead:
		if evt.Action != nil {
			// A full app-state snapshot contains the last explicit
			// mark-read/mark-unread action for a chat, even when the chat was
			// subsequently read normally. Replaying those old actions as current
			// state resurrects unread badges. Current cross-device changes arrive
			// as live app-state events or receipts (including offline replay).
			if evt.FromFullSync {
				return
			}
			if evt.Action.GetRead() {
				_ = c.store.MarkChatRead(context.Background(), evt.JID.String())
			} else {
				_ = c.store.MarkChatUnread(context.Background(), evt.JID.String())
			}
			c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": evt.JID.String(), "read": evt.Action.GetRead()}})
		}
	case *waEvents.Mute:
		if evt.Action != nil {
			_ = c.store.UpdateChatMuted(context.Background(), evt.JID.String(), normalizeMuteTime(uint64(evt.Action.GetMuteEndTimestamp())))
			c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": evt.JID.String()}})
		}
	case *waEvents.Archive:
		if evt.Action != nil {
			_ = c.store.UpdateChatArchived(context.Background(), evt.JID.String(), evt.Action.GetArchived())
			c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": evt.JID.String()}})
		}
	case *waEvents.JoinedGroup:
		_ = c.store.UpsertChat(context.Background(), model.Chat{JID: evt.JID.String(), Title: evt.Name, IsGroup: true})
		c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": evt.JID.String(), "title": evt.Name}})
	case *waEvents.GroupInfo:
		if evt.Name != nil {
			_ = c.store.UpdateChatTitle(context.Background(), evt.JID.String(), evt.Name.Name)
			c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": evt.JID.String(), "title": evt.Name.Name}})
		}
	}
}

// Live media downloads run a few at a time, each bounded, so a backlog
// delivered on reconnect cannot start hundreds of transfers at once.
const liveMediaTimeout = 5 * time.Minute

var liveMediaSlots = make(chan struct{}, 4)

const callLogBackfillMetadataKey = "call_logs_app_state_backfill_v1"
const reactionBackfillMetadataKey = "reactions_history_backfill_v1"

// reactionBackfillChats is how many conversations are asked for their recent
// page again, and reactionBackfillPause is the gap between those requests.
// Enough to cover the conversations anybody is reading, gentle enough that a
// linked device does not look like it is scraping its own history.
const reactionBackfillChats = 25
const reactionBackfillPause = 3 * time.Second
const chatSettingsBackfillMetadataKey = "chat_settings_app_state_backfill_v6"

// Call history lives in the regular app-state collection and chat settings in
// the low-priority one. A full sync is required once for installations that
// linked before either was imported, because reconnecting only fetches
// mutations newer than the cached collection version.
func (c *Client) backfillCallLogs() {
	c.backfillAppState(appstate.WAPatchRegular, callLogBackfillMetadataKey, "sync call history", "calls.synced")
}

func (c *Client) backfillChatSettings() {
	c.backfillAppState(appstate.WAPatchRegularLow, chatSettingsBackfillMetadataKey, "sync chat settings", "chat.updated")
}

func (c *Client) backfillAppState(name appstate.WAPatchName, metadataKey, label, doneEvent string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, done, err := c.store.Metadata(ctx, metadataKey); err != nil || done {
		return
	}
	// A full sync deletes the cached collection and applies every mutation the
	// server sends on the way through, so a collection that fails to verify
	// halfway leaves the settings it did apply behind - pins have gone missing
	// this way. Once the phone has been asked for a clean copy, wait for its
	// answer instead of tearing the collection down again on every connection.
	if c.awaitingAppStateRecovery(ctx, name) {
		return
	}
	fetch := c.fetchAppState
	if fetch == nil {
		fetch = c.wa.FetchAppState
	}
	if err := fetch(ctx, name, true, false); err != nil {
		if shouldDeferAppStateBackfill(err) {
			return
		}
		if isAppStateHashMismatch(err) {
			// The collection the server sent does not add up to the hash it
			// signed, and asking again produces the same bytes. Only the phone
			// can settle it, so ask it for a plain copy and say nothing: this
			// sync is one nobody asked for, and a banner about a hash across
			// somebody's conversation helps no one.
			c.recoverAppState(ctx, name, label)
			return
		}
		c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": label + ": " + err.Error()}})
		return
	}
	if err := c.store.SetMetadata(ctx, metadataKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return
	}
	c.emit(gateway.Event{Name: doneEvent})
}

// appStateRecoveryMetadataKey remembers when the phone was last asked for a
// collection, so a mismatch that the phone cannot fix does not send it a
// request on every connection.
func appStateRecoveryMetadataKey(name appstate.WAPatchName) string {
	return "app_state_recovery_" + string(name)
}

const appStateRecoveryInterval = 24 * time.Hour

// awaitingAppStateRecovery reports whether the phone has been asked for this
// collection recently enough that its answer is still worth waiting for.
func (c *Client) awaitingAppStateRecovery(ctx context.Context, name appstate.WAPatchName) bool {
	value, _, err := c.store.Metadata(ctx, appStateRecoveryMetadataKey(name))
	if err != nil || value == "" {
		return false
	}
	asked, parseErr := time.Parse(time.RFC3339, value)
	return parseErr == nil && time.Since(asked) < appStateRecoveryInterval
}

// recoverAppState asks the phone for an unencrypted copy of one app-state
// collection. The reply arrives as a peer message that whatsmeow applies on its
// own; it finishes as an AppStateSyncComplete, which is where the backfill is
// marked done.
func (c *Client) recoverAppState(ctx context.Context, name appstate.WAPatchName, label string) {
	if c.awaitingAppStateRecovery(ctx, name) {
		return
	}
	key := appStateRecoveryMetadataKey(name)
	send := c.sendPeerMessage
	if send == nil {
		send = c.wa.SendPeerMessage
	}
	if _, err := send(ctx, whatsmeow.BuildAppStateRecoveryRequest(name)); err != nil {
		log.Printf("%s: could not ask the phone for %s: %v", label, name, err)
		return
	}
	log.Printf("%s: %s did not verify; asked the phone for a fresh copy", label, name)
	_ = c.store.SetMetadata(ctx, key, time.Now().UTC().Format(time.RFC3339))
}

// backfillReactions asks the busiest conversations for their recent page once.
//
// Reactions arrive with history rather than as events, and they were dropped
// before this version, so the messages already stored carry none. Nothing that
// arrives later mentions them: without asking again, a conversation everybody
// reacted to would look untouched for as long as it is kept.
func (c *Client) backfillReactions() {
	ctx := c.baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	c.backfillReactionsWith(ctx, c.RefreshHistory, reactionBackfillChats, reactionBackfillPause)
}

func (c *Client) backfillReactionsWith(ctx context.Context,
	refresh func(context.Context, string, int) error, chats int, pause time.Duration) {
	if !c.reactionBackfilling.CompareAndSwap(false, true) {
		return
	}
	defer c.reactionBackfilling.Store(false)
	if _, done, err := c.store.Metadata(ctx, reactionBackfillMetadataKey); err != nil || done {
		return
	}
	conversations, err := c.store.ListChats(ctx, chats, 0, "")
	if err != nil {
		return
	}
	for _, chat := range conversations {
		// A conversation with nothing stored has no message to anchor a page
		// request to. The rest still get theirs.
		if err := refresh(ctx, chat.JID, 50); err != nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pause):
		}
	}
	_ = c.store.SetMetadata(ctx, reactionBackfillMetadataKey, time.Now().UTC().Format(time.RFC3339))
}

func shouldDeferAppStateBackfill(err error) bool {
	return errors.Is(err, appstate.ErrKeyNotFound)
}

// isAppStateHashMismatch reports the one failure a retry cannot help with: the
// mutations decoded, but the running hash does not match the one the server
// signed, so the collection has to come from the phone instead.
func isAppStateHashMismatch(err error) bool {
	return errors.Is(err, appstate.ErrMismatchingLTHash)
}

func (c *Client) handleAppStateSyncComplete(evt *waEvents.AppStateSyncComplete) {
	if evt == nil {
		return
	}
	ctx := context.Background()
	switch evt.Name {
	case appstate.WAPatchRegular:
		_ = c.store.SetMetadata(ctx, callLogBackfillMetadataKey, time.Now().UTC().Format(time.RFC3339))
		c.emit(gateway.Event{Name: "calls.synced"})
	case appstate.WAPatchRegularLow:
		_ = c.store.SetMetadata(ctx, chatSettingsBackfillMetadataKey, time.Now().UTC().Format(time.RFC3339))
		c.emit(gateway.Event{Name: "chat.updated"})
	}
}

func (c *Client) handleAppState(evt *waEvents.AppState) {
	if evt == nil || len(evt.Index) == 0 {
		return
	}
	switch evt.Index[0] {
	case appstate.IndexCallLog:
		record := evt.GetCallLogAction().GetCallLogRecord()
		call, ok := callLogFromRecord(record)
		if !ok {
			return
		}
		if err := c.store.UpsertCallLog(context.Background(), call); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "save call history: " + err.Error()}})
			return
		}
		c.emit(gateway.Event{Name: "call.upsert", Data: call})
	case appstate.IndexLabelEdit:
		action := evt.GetLabelEditAction()
		if action == nil || len(evt.Index) < 2 {
			return
		}
		label := model.Label{ID: evt.Index[1], Name: action.GetName(), Color: int(action.GetColor())}
		if err := c.store.UpsertLabel(context.Background(), label, action.GetDeleted()); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "save chat list: " + err.Error()}})
			return
		}
		c.emit(gateway.Event{Name: "labels.updated"})
	case appstate.IndexLabelAssociationChat:
		action := evt.GetLabelAssociationAction()
		// The index carries which list and which chat; the action only says
		// whether the chat is in it.
		if action == nil || len(evt.Index) < 3 {
			return
		}
		if err := c.store.SetChatLabeled(context.Background(), evt.Index[2], evt.Index[1], action.GetLabeled()); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "save chat list membership: " + err.Error()}})
			return
		}
		c.emit(gateway.Event{Name: "labels.updated"})
	case appstate.IndexFavorites:
		action := evt.GetFavoritesAction()
		if action == nil {
			return
		}
		favorites := make([]string, 0, len(action.GetFavorites()))
		for _, favorite := range action.GetFavorites() {
			if jid := strings.TrimSpace(favorite.GetID()); jid != "" {
				favorites = append(favorites, jid)
			}
		}
		if err := c.store.UpdateChatFavorites(context.Background(), favorites); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "save favorite chats: " + err.Error()}})
			return
		}
		c.emit(gateway.Event{Name: "chat.updated"})
	}
}

func (c *Client) syncDirectory() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if contacts, err := c.wa.Store.Contacts.GetAllContacts(ctx); err == nil {
		// Saved names affect what the user sees, so resolve them before the
		// potentially large alias-only pass. Go map iteration is random and the
		// directory operation has a timeout; a single mixed pass made startup
		// naming depend on iteration order.
		for pass := 0; pass < 2; pass++ {
			for jid, info := range contacts {
				name := authoritativeContactName(info)
				if (pass == 0) != (name != "") {
					continue
				}
				var lid types.JID
				if jid.Server == types.DefaultUserServer {
					lid, _ = c.wa.Store.LIDs.GetLIDForPN(ctx, jid)
				}
				if err := syncDirectoryContactInfo(ctx, c.store, jid, lid, info); err != nil {
					c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "merge contact history: " + err.Error()}})
				}
			}
		}
	}
	// History can contain a LID whose phone mapping is known locally even when
	// the contact table has never fetched that user's verified business name.
	// Ask WhatsApp for all such identities in one batch; GetUserInfo persists
	// verified names into the whatsmeow contact store, after which the normal
	// strength rules below can safely upgrade only numeric placeholders.
	c.enrichPlaceholderContacts(ctx)
	if groups, err := c.wa.GetJoinedGroups(ctx); err == nil {
		for _, group := range groups {
			_ = c.store.UpsertChat(ctx, model.Chat{JID: group.JID.String(), Title: group.Name, LastMessageAt: group.GroupCreated.UnixMilli(), IsGroup: true})
		}
	}
	c.emit(gateway.Event{Name: "directory.synced"})
}

// authoritativeContactName deliberately excludes PushName. LID and phone-JID
// entries can describe the same person, and the LID entry often contains only
// a push name. Letting it overwrite the phone entry's address-book name made
// titles change nondeterministically on every reconnect.
func authoritativeContactName(info types.ContactInfo) string {
	return firstNonEmpty(info.FullName, info.FirstName, info.BusinessName)
}

// syncDirectoryContact consolidates the legacy phone-number address and the
// privacy-preserving LID before naming the conversation. WhatsApp history may
// contain both forms for the same person; leaving them separate makes the UI
// appear to have lost history even though all rows are present in SQLite.
func syncDirectoryContact(ctx context.Context, st *localstore.Store, pn, lid types.JID, name string) error {
	canonical := pn
	if !lid.IsEmpty() && lid.String() != pn.String() {
		if err := st.LinkChatAliases(ctx, lid.String(), pn.String()); err != nil {
			return err
		}
		canonical = lid
	}
	// A directory-only contact does not appear in the normal chat list until it
	// has messages, but retaining its identity lets status stories resolve the
	// same saved name WhatsApp Web shows.
	return st.UpsertChat(ctx, model.Chat{JID: canonical.String(), Title: name})
}

func syncDirectoryContactInfo(ctx context.Context, st *localstore.Store, pn, lid types.JID, info types.ContactInfo) error {
	name := authoritativeContactName(info)
	if err := syncDirectoryContact(ctx, st, pn, lid, name); err != nil {
		return err
	}
	if name != "" || strings.TrimSpace(info.PushName) == "" {
		return nil
	}
	canonical := pn
	if !lid.IsEmpty() && lid.String() != pn.String() {
		canonical = lid
	}
	return st.UpdateChatTitleIfPlaceholder(ctx, canonical.String(), info.PushName)
}

func (c *Client) enrichPlaceholderContacts(ctx context.Context) {
	stored, err := c.store.ListPlaceholderContactJIDs(ctx, 100)
	if err != nil || len(stored) == 0 {
		return
	}
	type identity struct {
		pn  types.JID
		lid types.JID
	}
	identities := make([]identity, 0, len(stored))
	queries := make([]types.JID, 0, len(stored))
	seen := make(map[string]bool)
	for _, raw := range stored {
		jid, err := types.ParseJID(raw)
		if err != nil {
			continue
		}
		current := identity{pn: jid}
		switch jid.Server {
		case types.HiddenUserServer:
			current.lid = jid
			current.pn, err = c.wa.Store.LIDs.GetPNForLID(ctx, jid)
		case types.DefaultUserServer:
			current.lid, _ = c.wa.Store.LIDs.GetLIDForPN(ctx, jid)
		default:
			continue
		}
		if err != nil || current.pn.IsEmpty() || current.pn.Server != types.DefaultUserServer {
			continue
		}
		identities = append(identities, current)
		if !seen[current.pn.String()] {
			seen[current.pn.String()] = true
			queries = append(queries, current.pn)
		}
	}
	if len(queries) == 0 {
		return
	}
	if _, err := c.wa.GetUserInfo(ctx, queries); err != nil {
		return
	}
	for _, current := range identities {
		info, err := c.wa.Store.Contacts.GetContact(ctx, current.pn)
		if err != nil {
			continue
		}
		_ = syncDirectoryContactInfo(ctx, c.store, current.pn, current.lid, info)
	}
}

func (c *Client) handleMessage(evt *waEvents.Message) {
	if evt.Message == nil {
		return
	}
	if reaction := evt.Message.GetReactionMessage(); reaction != nil {
		r := model.Reaction{ChatJID: evt.Info.Chat.String(), MessageID: reaction.GetKey().GetID(), SenderJID: c.reactionSenderJID(evt.Info.Sender, evt.Info.IsFromMe), Emoji: reaction.GetText(), Timestamp: evt.Info.Timestamp.UnixMilli()}
		_ = c.recordReaction(context.Background(), r)
		return
	}
	if pin := evt.Message.GetPinInChatMessage(); pin != nil {
		target := pin.GetKey().GetID()
		if target == "" {
			return
		}
		pinned := pin.GetType() == waE2E.PinInChatMessage_PIN_FOR_ALL
		var err error
		if pinned {
			duration := time.Duration(evt.Message.GetMessageContextInfo().GetMessageAddOnDurationInSecs()) * time.Second
			if duration <= 0 {
				duration = 7 * 24 * time.Hour
			}
			err = c.store.SetMessagePinned(context.Background(), evt.Info.Chat.String(), target, evt.Info.Timestamp.Add(duration).UnixMilli())
		} else if pin.GetType() == waE2E.PinInChatMessage_UNPIN_FOR_ALL {
			err = c.store.ClearMessagePin(context.Background(), evt.Info.Chat.String())
		} else {
			return
		}
		if err == nil {
			c.emit(gateway.Event{Name: "message.pinned", Data: map[string]any{
				"chat_jid": evt.Info.Chat.String(), "message_id": target, "pinned": pinned,
			}})
		}
		return
	}
	if protocol := evt.Message.GetProtocolMessage(); protocol != nil && protocol.GetType() == waE2E.ProtocolMessage_REVOKE {
		target := protocol.GetKey().GetID()
		_ = c.store.MarkRevoked(context.Background(), evt.Info.Chat.String(), target)
		c.emit(gateway.Event{Name: "message.revoked", Data: map[string]string{"chat_jid": evt.Info.Chat.String(), "message_id": target}})
		return
	}
	msg := c.withSenderName(context.Background(), messageFromEvent(evt))
	if msg.ID == "" || msg.ChatJID == "" {
		return
	}
	msg = c.withCachedThumbnail(msg, evt.Message)
	msg = c.withCachedLinkPreview(msg, evt.Message)
	msg = c.withReplyPreview(context.Background(), msg)
	title := evt.Info.PushName
	if evt.Info.IsGroup {
		title = ""
	}
	if err := c.store.UpsertMessage(context.Background(), msg, title, !msg.FromMe); err != nil {
		c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "save message: " + err.Error()}})
		return
	}
	c.rememberMediaPayload(msg, evt.Message)
	c.emit(gateway.Event{Name: "message.upsert", Data: msg})
	if shouldNotifyMessage(msg) {
		chatInfo, _ := c.store.GetChat(context.Background(), msg.ChatJID)
		notifyTitle := notificationTitle(chatInfo, title, msg.ChatJID)
		muted := chatInfo.MutedUntil > time.Now().UnixMilli()
		if !muted {
			body := notificationBody(msg, evt.Info.IsGroup)
			// handled tells the desktop client whether this daemon already put
			// the message on screen. On Linux the freedesktop service does it
			// and the client must stay quiet; where no such service exists the
			// client presents the message itself.
			handled := "0"
			if c.notifier != nil && c.notifier.Presents() {
				handled = "1"
			}
			c.emit(gateway.Event{Name: "notification.received", Data: map[string]string{
				"chat_jid":    msg.ChatJID,
				"title":       notifyTitle,
				"body":        body,
				"avatar_path": chatInfo.AvatarPath,
				"handled":     handled,
			}})
			go func(notification notify.Message) {
				if notification.IconPath == "" {
					ctx, cancel := context.WithTimeout(c.baseCtx, 3*time.Second)
					defer cancel()
					if path, err := c.FetchAvatar(ctx, notification.ChatJID); err == nil && path != "" {
						notification.IconPath = path
						_ = c.store.UpdateChatAvatar(context.Background(), notification.ChatJID, path)
						c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": notification.ChatJID, "avatar_path": path}})
					}
				}
				if err := c.notifier.Notify(context.Background(), notification); err != nil {
					log.Printf("deliver desktop notification: %v", err)
				}
			}(notify.Message{ChatJID: msg.ChatJID, Title: notifyTitle, Body: body, IconPath: chatInfo.AvatarPath})
		}
	}
	if downloadable := downloadableFromMessage(evt.Message); downloadable != nil && !evt.IsViewOnce {
		go c.cacheMedia(msg, downloadable, evt.Message)
	}
}

func shouldNotifyMessage(msg model.Message) bool {
	return !msg.FromMe && msg.ChatJID != "status@broadcast"
}

// notificationTitle keeps native notifications consistent with the chat list.
// A message's PushName is self-published and may be stale (or simply unrelated
// to the name the user saved), so it is only a fallback for an unnamed chat.
func notificationTitle(chat model.Chat, pushName, chatJID string) string {
	if title := strings.TrimSpace(chat.Title); title != "" {
		return title
	}
	if title := strings.TrimSpace(pushName); title != "" {
		return title
	}
	return displayJID(chatJID)
}

// notificationBody summarises a message for a desktop notification: its text
// or caption when present, otherwise the capitalised message kind, prefixed
// with the sender's name inside groups.
func notificationBody(msg model.Message, isGroup bool) string {
	body := msg.Body
	if body == "" {
		body = capitalize(msg.Kind)
	}
	if isGroup && msg.SenderName != "" {
		body = msg.SenderName + ": " + body
	}
	return body
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	first, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(first)) + s[size:]
}

func (c *Client) handleReceipt(evt *waEvents.Receipt) {
	// Only the receipts that report progress may change a message's mark.
	// "server-error" and "inactive" also arrive here, and treating them as the
	// default put a delivered tick on messages that never got there.
	var status string
	switch evt.Type {
	case types.ReceiptTypeDelivered, types.ReceiptTypeSender:
		status = "delivered"
	case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
		status = "read"
	case types.ReceiptTypePlayed, types.ReceiptTypePlayedSelf:
		status = "played"
	default:
		return
	}
	ids := make([]string, len(evt.MessageIDs))
	for i, id := range evt.MessageIDs {
		ids[i] = string(id)
	}
	_ = c.store.UpdateReceipt(context.Background(), evt.Chat.String(), ids, status, evt.Timestamp.UnixMilli())
	_ = c.store.RecalculateChatUnread(context.Background(), evt.Chat.String())
	// A read-self receipt is generated when this account reads incoming
	// messages on another linked device while read receipts are disabled. It
	// is chat state, not merely delivery state for one outgoing message.
	if evt.Type == types.ReceiptTypeReadSelf {
		_ = c.store.MarkChatRead(context.Background(), evt.Chat.String())
	}
	c.emit(gateway.Event{Name: "message.receipt", Data: map[string]any{"chat_jid": evt.Chat.String(), "message_ids": ids, "status": status, "timestamp": evt.Timestamp.UnixMilli()}})
}

func (c *Client) handleHistorySync(evt *waEvents.HistorySync) {
	if evt.Data == nil {
		return
	}
	// Only bootstrap, recent, and full syncs describe conversation settings.
	// On-demand history pages carry messages without mute, pin, or archive
	// state; their absent fields must not be written back as cleared settings.
	storeChat := c.store.UpsertChat
	if historySyncCarriesChatSettings(evt.Data.GetSyncType()) {
		storeChat = c.store.ApplyChatSnapshot
	}
	total := 0
	chatJIDs := make([]string, 0, len(evt.Data.GetConversations()))
	for _, conversation := range evt.Data.GetConversations() {
		chatJID, err := types.ParseJID(conversation.GetID())
		if err != nil {
			continue
		}
		chatJIDs = append(chatJIDs, chatJID.String())
		title := conversation.GetName()
		if title == "" {
			title = conversation.GetDisplayName()
		}
		// A history conversation often carries the sender's push name, while
		// directory sync has the user's saved address-book name. Never let the
		// weaker history value replace an already resolved identity. A raw JID
		// placeholder is still upgraded normally.
		if existing, err := c.store.GetChat(context.Background(), chatJID.String()); err == nil &&
			strings.TrimSpace(existing.Title) != "" && existing.Title != displayJID(chatJID.String()) {
			title = ""
		}
		if err := storeChat(context.Background(), model.Chat{JID: chatJID.String(), Title: title, LastMessageAt: int64(conversation.GetLastMsgTimestamp()) * 1000, UnreadCount: int(conversation.GetUnreadCount()), MutedUntil: normalizeMuteTime(conversation.GetMuteEndTime()), Pinned: conversation.GetPinned() > 0, PinnedAt: int64(conversation.GetPinned()), Archived: conversation.GetArchived(), IsGroup: chatJID.Server == types.GroupServer}); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "save conversation: " + err.Error()}})
			continue
		}
		for _, historyMessage := range conversation.GetMessages() {
			parsed, err := c.wa.ParseWebMessage(chatJID, historyMessage.GetMessage())
			if err != nil {
				continue
			}
			msg := c.withSenderName(context.Background(), messageFromEvent(parsed))
			if msg.ID == "" {
				continue
			}
			msg = c.withCachedThumbnail(msg, parsed.Message)
			msg = c.withCachedLinkPreview(msg, parsed.Message)
			// History carries how far each of our own messages actually got.
			// Without this every synced message looks merely sent, so a
			// conversation full of read messages shows a single mark.
			if msg.FromMe {
				if delivered := deliveryFromWebStatus(historyMessage.GetMessage().GetStatus()); delivered != "" {
					msg.Status = delivered
				}
			}
			if err := c.store.UpsertMessage(context.Background(), msg, title, false); err == nil {
				c.rememberMediaPayload(msg, parsed.Message)
				// The upsert deliberately leaves the star alone so a
				// redelivered message keeps it. Only a set star is applied
				// here: an absent field and a cleared one look identical on
				// the wire, so honouring false would drop stars this device
				// already knows about. Unstarring arrives as a Star event.
				if historyMessage.GetMessage().GetStarred() {
					_ = c.store.SetMessageStarred(context.Background(), msg.ChatJID, msg.ID, true)
				}
				c.recordHistoryReactions(context.Background(), chatJID, msg,
					historyMessage.GetMessage().GetReactions())
				total++
			}
		}
	}
	for _, record := range evt.Data.GetCallLogRecords() {
		if call, ok := callLogFromRecord(record); ok {
			_ = c.store.UpsertCallLog(context.Background(), call)
		}
	}
	_ = c.store.RecalculateUnreadCounts(context.Background())
	c.emit(gateway.Event{Name: "history.synced", Data: map[string]any{"messages": total, "chat_jids": chatJIDs}})
}

// recordHistoryReactions saves the reactions a synced message already carried.
//
// A reaction is a message of its own, so one left before this device was
// linked is never delivered as an event: it arrives only here, attached to the
// message it was left on. Without this a conversation everybody replied to
// with an emoji comes back looking untouched.
func (c *Client) recordHistoryReactions(ctx context.Context, chatJID types.JID,
	msg model.Message, reactions []*waWeb.Reaction) {
	for _, reaction := range reactions {
		emoji := reaction.GetText()
		if emoji == "" {
			continue
		}
		sender := c.historyReactionSender(chatJID, reaction.GetKey())
		if sender == "" {
			continue
		}
		_ = c.store.UpsertReaction(ctx, model.Reaction{
			ChatJID:   chatJID.String(),
			MessageID: msg.ID,
			SenderJID: sender,
			Emoji:     emoji,
			Timestamp: reaction.GetSenderTimestampMS(),
		})
	}
}

// historyReactionSender works out who left a reaction that came with history.
// Its key describes the reaction message, not the message reacted to: in a
// group the participant is the reader who left it, and in a direct chat the
// only two candidates are this account and the person on the other end.
func (c *Client) historyReactionSender(chatJID types.JID, key *waCommon.MessageKey) string {
	if key.GetFromMe() {
		return c.reactionSenderJID(types.EmptyJID, true)
	}
	if participant := key.GetParticipant(); participant != "" {
		sender, err := types.ParseJID(participant)
		if err != nil {
			return ""
		}
		return sender.ToNonAD().String()
	}
	if chatJID.Server == types.GroupServer {
		// Somebody in the group left it and the history did not say who.
		return ""
	}
	return chatJID.ToNonAD().String()
}

func historySyncCarriesChatSettings(syncType waHistorySync.HistorySync_HistorySyncType) bool {
	switch syncType {
	case waHistorySync.HistorySync_INITIAL_BOOTSTRAP, waHistorySync.HistorySync_RECENT, waHistorySync.HistorySync_FULL:
		return true
	default:
		return false
	}
}

// deliveryFromWebStatus translates the delivery state stored with a history
// message. An empty result means the message carries nothing better than what
// is already known about it.
func deliveryFromWebStatus(status waWeb.WebMessageInfo_Status) string {
	switch status {
	case waWeb.WebMessageInfo_PLAYED:
		return "played"
	case waWeb.WebMessageInfo_READ:
		return "read"
	case waWeb.WebMessageInfo_DELIVERY_ACK:
		return "delivered"
	default:
		return ""
	}
}

func callLogFromRecord(record *waSyncAction.CallLogRecord) (model.CallLog, bool) {
	if record == nil || record.GetCallID() == "" {
		return model.CallLog{}, false
	}
	peer := record.GetGroupJID()
	if peer == "" {
		peer = record.GetCallCreatorJID()
	}
	if !record.GetIsIncoming() && len(record.GetParticipants()) > 0 {
		peer = record.GetParticipants()[0].GetUserJID()
	}
	timestamp := record.GetStartTime()
	if timestamp > 0 && timestamp < 1_000_000_000_000 {
		timestamp *= 1000
	}
	return model.CallLog{
		ID:        record.GetCallID(),
		PeerJID:   peer,
		Timestamp: timestamp,
		Duration:  record.GetDuration(),
		Incoming:  record.GetIsIncoming(),
		Video:     record.GetIsVideo(),
		Result:    strings.ToLower(record.GetCallResult().String()),
	}, true
}

func (c *Client) rememberMediaPayload(msg model.Message, raw *waE2E.Message) {
	if downloadableFromMessage(raw) == nil {
		return
	}
	payload, err := proto.Marshal(raw)
	if err == nil {
		_ = c.store.SaveMediaPayload(context.Background(), msg.ChatJID, msg.ID, payload)
	}
}

// cacheMedia downloads the file belonging to a message that just arrived.
//
// A reconnection can deliver a whole backlog at once, so these downloads are
// limited to a few at a time: without that they would all start together and
// undo the deliberate pacing the background collector keeps. They also follow
// the daemon's lifetime rather than running detached, and each one is bounded
// so a stalled transfer cannot hold a slot forever.
func (c *Client) cacheMedia(msg model.Message, media whatsmeow.DownloadableMessage, raw *waE2E.Message) {
	base := c.baseCtx
	if base == nil {
		base = context.Background()
	}
	select {
	case liveMediaSlots <- struct{}{}:
		defer func() { <-liveMediaSlots }()
	case <-base.Done():
		return
	}
	ctx, cancel := context.WithTimeout(base, liveMediaTimeout)
	defer cancel()
	_, _ = c.downloadMedia(ctx, msg, media, raw)
}

// cachePath is where a message's file is materialised for the desktop to read.
func (c *Client) cachePath(msg model.Message) string {
	return filepath.Join(c.mediaDir, msg.Kind, safeName(msg.ChatJID+"-"+msg.ID)+extensionForMIME(msg.MediaMIME))
}

func (c *Client) withDisplayableSticker(msg model.Message) model.Message {
	if msg.Kind != "sticker" || msg.MediaPath == "" {
		return msg
	}
	if path, err := mediaformat.StickerPNG(msg.MediaPath); err == nil {
		msg.MediaPath = path
	}
	return msg
}

func (c *Client) downloadMedia(ctx context.Context, msg model.Message, media whatsmeow.DownloadableMessage, raw *waE2E.Message) (model.Message, error) {
	dir := filepath.Join(c.mediaDir, msg.Kind)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return model.Message{}, err
	}
	path := c.cachePath(msg)
	tmp, err := os.CreateTemp(dir, "download-*")
	if err != nil {
		return model.Message{}, err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(tmpName)
		}
	}()
	download := c.downloadToFile
	if download == nil {
		download = c.wa.DownloadToFile
	}
	if err := download(ctx, media, tmp); err != nil {
		if raw == nil || !isExpiredMediaDownload(err) {
			return model.Message{}, err
		}
		requestPath := c.requestMediaRetryPath
		if requestPath == nil {
			requestPath = c.awaitMediaRetryPath
		}
		freshPath, retryErr := requestPath(ctx, msg, media)
		if retryErr != nil {
			return model.Message{}, fmt.Errorf("refresh expired media: %w", retryErr)
		}
		if !setMediaDirectPath(raw, freshPath) {
			return model.Message{}, errors.New("refresh expired media: unsupported message type")
		}
		payload, marshalErr := proto.Marshal(raw)
		if marshalErr != nil {
			return model.Message{}, fmt.Errorf("save refreshed media path: %w", marshalErr)
		}
		if saveErr := c.store.SaveMediaPayload(ctx, msg.ChatJID, msg.ID, payload); saveErr != nil {
			return model.Message{}, fmt.Errorf("save refreshed media path: %w", saveErr)
		}
		if resetErr := tmp.Truncate(0); resetErr != nil {
			return model.Message{}, resetErr
		}
		if _, resetErr := tmp.Seek(0, io.SeekStart); resetErr != nil {
			return model.Message{}, resetErr
		}
		downloadFresh := c.downloadWithPathToFile
		if downloadFresh == nil {
			downloadFresh = func(ctx context.Context, directPath string, media whatsmeow.DownloadableMessage, file whatsmeow.File) error {
				return c.wa.DownloadMediaWithPathToFile(ctx, directPath, media.GetFileEncSHA256(), media.GetFileSHA256(), media.GetMediaKey(), whatsmeow.GetMediaType(media), "", false, file)
			}
		}
		if retryErr := downloadFresh(ctx, freshPath, media, tmp); retryErr != nil {
			return model.Message{}, retryErr
		}
	}
	if err := tmp.Close(); err != nil {
		return model.Message{}, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return model.Message{}, err
	}
	ok = true
	msg.MediaPath = path
	if info, err := os.Stat(path); err == nil {
		msg.MediaSize = info.Size()
	}
	if c.media != nil {
		info := mediastore.Info{MIME: msg.MediaMIME, Name: msg.MediaName, Size: msg.MediaSize}
		if err := c.media.PutFile(ctx, msg.ChatJID, msg.ID, info, path); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "store attachment: " + err.Error()}})
		}
	}
	msg = c.withDisplayableSticker(msg)
	if err := c.store.UpsertMessage(ctx, msg, "", false); err != nil {
		return model.Message{}, err
	}
	c.emit(gateway.Event{Name: "message.upsert", Data: msg})
	return msg, nil
}

// withReplyPreview fills in what a reply shows about the message it answers.
// The stored copy is better than the one attached to the reply: it carries the
// sender's real name rather than a phone number, and it reflects an edit or a
// deletion that happened after the reply was written.
func (c *Client) withReplyPreview(ctx context.Context, m model.Message) model.Message {
	if m.ReplyTo == "" {
		return m
	}
	text, sender, fromMe, ok := c.store.ReplyPreview(ctx, m.ChatJID, m.ReplyTo)
	if !ok {
		return m
	}
	m.ReplyPreview = text
	m.ReplyFromMe = fromMe
	if sender != "" {
		m.ReplySender = sender
	}
	return m
}

// withSenderName fills in the name shown above an incoming group message.
// WhatsApp Web always labels those bubbles; history sync frequently omits the
// push name, which left every synced group message unlabelled. The address
// book is consulted next, and the bare number is used last so the label never
// comes out empty.
func (c *Client) withSenderName(ctx context.Context, msg model.Message) model.Message {
	if msg.FromMe || strings.TrimSpace(msg.SenderName) != "" {
		return msg
	}
	sender, err := types.ParseJID(msg.SenderJID)
	if err != nil || sender.User == "" {
		return msg
	}
	chat, chatErr := types.ParseJID(msg.ChatJID)
	if chatErr != nil || chat.Server != types.GroupServer {
		return msg
	}
	if info, err := c.wa.Store.Contacts.GetContact(ctx, sender); err == nil {
		for _, candidate := range []string{info.FullName, info.PushName, info.BusinessName} {
			if name := strings.TrimSpace(candidate); name != "" {
				msg.SenderName = name
				return msg
			}
		}
	}
	if known, err := c.store.GetChat(ctx, sender.ToNonAD().String()); err == nil {
		if title := strings.TrimSpace(known.Title); title != "" && title != displayJID(known.JID) {
			msg.SenderName = title
			return msg
		}
	}
	msg.SenderName = "+" + displayJID(sender.ToNonAD().String())
	return msg
}

func messageFromEvent(evt *waEvents.Message) model.Message {
	m := model.Message{ID: string(evt.Info.ID), ChatJID: evt.Info.Chat.String(), SenderJID: evt.Info.Sender.String(), SenderName: evt.Info.PushName, Timestamp: evt.Info.Timestamp.UnixMilli(), FromMe: evt.Info.IsFromMe, Status: "received", Edited: evt.IsEdit}
	if m.FromMe {
		m.Status = "sent"
	}
	if evt.IsEdit && evt.RawMessage != nil {
		if wrapper := evt.RawMessage.GetEditedMessage().GetMessage(); wrapper != nil {
			if protocol := wrapper.GetProtocolMessage(); protocol != nil && protocol.GetKey() != nil && protocol.GetKey().GetID() != "" {
				m.ID = protocol.GetKey().GetID()
			}
		}
	}
	msg := evt.Message
	if msg == nil {
		return m
	}
	switch {
	case msg.GetConversation() != "":
		m.Kind = "text"
		m.Body = msg.GetConversation()
	case msg.GetExtendedTextMessage() != nil:
		v := msg.GetExtendedTextMessage()
		m.Kind = "text"
		m.Body = v.GetText()
		applyContext(&m, v.GetContextInfo())
		applyLinkPreview(&m, v)
	case msg.GetImageMessage() != nil:
		v := msg.GetImageMessage()
		m.Kind = "image"
		m.Body = v.GetCaption()
		m.MediaMIME = v.GetMimetype()
		m.MediaSize = int64(v.GetFileLength())
		applyContext(&m, v.GetContextInfo())
	case msg.GetVideoMessage() != nil:
		v := msg.GetVideoMessage()
		m.Kind = "video"
		m.Body = v.GetCaption()
		m.MediaMIME = v.GetMimetype()
		m.MediaSize = int64(v.GetFileLength())
		applyContext(&m, v.GetContextInfo())
	case msg.GetAudioMessage() != nil:
		v := msg.GetAudioMessage()
		m.Kind = "audio"
		m.MediaMIME = v.GetMimetype()
		m.MediaSize = int64(v.GetFileLength())
		m.MediaDuration = int(v.GetSeconds())
		// The sender's client recorded these amplitude bars, so the waveform
		// shown is the real shape of the recording and needs no decoding here.
		for _, bar := range v.GetWaveform() {
			m.AudioWaveform = append(m.AudioWaveform, int(bar))
		}
		applyContext(&m, v.GetContextInfo())
	case msg.GetDocumentMessage() != nil:
		v := msg.GetDocumentMessage()
		m.Kind = "document"
		m.Body = v.GetCaption()
		m.MediaMIME = v.GetMimetype()
		m.MediaName = v.GetFileName()
		m.MediaSize = int64(v.GetFileLength())
		applyContext(&m, v.GetContextInfo())
	case msg.GetStickerMessage() != nil:
		v := msg.GetStickerMessage()
		m.Kind = "sticker"
		m.MediaMIME = v.GetMimetype()
		m.MediaSize = int64(v.GetFileLength())
		applyContext(&m, v.GetContextInfo())
	case msg.GetContactMessage() != nil:
		v := msg.GetContactMessage()
		m.Kind = "contact"
		m.ContactName = firstNonEmpty(v.GetDisplayName(), nameFromVCard(v.GetVcard()), "Contact")
		m.ContactPhone = phoneFromVCard(v.GetVcard())
		m.ContactCount = 1
		m.Body = m.ContactName
		applyContext(&m, v.GetContextInfo())
	case msg.GetContactsArrayMessage() != nil:
		v := msg.GetContactsArrayMessage()
		m.Kind = "contact"
		m.ContactCount = len(v.GetContacts())
		m.ContactName = firstNonEmpty(v.GetDisplayName(), "Contacts")
		if first := v.GetContacts(); len(first) > 0 {
			m.ContactName = firstNonEmpty(first[0].GetDisplayName(), nameFromVCard(first[0].GetVcard()), m.ContactName)
			m.ContactPhone = phoneFromVCard(first[0].GetVcard())
		}
		m.Body = m.ContactName
		applyContext(&m, v.GetContextInfo())
	case msg.GetLocationMessage() != nil:
		v := msg.GetLocationMessage()
		m.Kind = "location"
		m.Latitude = v.GetDegreesLatitude()
		m.Longitude = v.GetDegreesLongitude()
		m.Body = firstNonEmpty(v.GetName(), v.GetAddress(), "Location")
		applyContext(&m, v.GetContextInfo())
	case msg.GetLiveLocationMessage() != nil:
		v := msg.GetLiveLocationMessage()
		m.Kind = "location"
		m.Latitude = v.GetDegreesLatitude()
		m.Longitude = v.GetDegreesLongitude()
		m.Body = firstNonEmpty(v.GetCaption(), "Live location")
		applyContext(&m, v.GetContextInfo())
	case firstNonNilPoll(msg) != nil:
		m.Kind = "poll"
		m.Body = firstNonEmpty(firstNonNilPoll(msg).GetName(), "Poll")
	case msg.GetReactionMessage() != nil:
		v := msg.GetReactionMessage()
		m.Kind = "reaction"
		m.Body = v.GetText()
		m.ReplyTo = v.GetKey().GetID()
	case msg.GetProtocolMessage() != nil:
		// A protocol message is machinery: an ephemeral-timer change, a history
		// notification, a key share. Only a revocation is something a reader
		// sees. The rest used to be stored as an empty "system" message, which
		// drew a bubble with nothing in it but a clock.
		if msg.GetProtocolMessage().GetType() == waE2E.ProtocolMessage_REVOKE {
			m.Kind = "system"
			m.Revoked = true
			m.ReplyTo = msg.GetProtocolMessage().GetKey().GetID()
		}
	}
	if m.Kind == "" {
		// Key distribution and other transport-only envelopes aren't visible
		// chat messages. Keeping them created blank bubbles and generic previews.
		return model.Message{}
	}
	return m
}

// phoneFromVCard returns the first telephone number in a shared contact card.
// A vCard line looks like "TEL;type=CELL;waid=15551234567:+1 555 123 4567", so
// the number is whatever follows the last colon.
func phoneFromVCard(card string) string {
	for _, line := range strings.Split(card, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if !strings.HasPrefix(strings.ToUpper(line), "TEL") {
			continue
		}
		if colon := strings.LastIndex(line, ":"); colon >= 0 && colon+1 < len(line) {
			if number := strings.TrimSpace(line[colon+1:]); number != "" {
				return number
			}
		}
	}
	return ""
}

// nameFromVCard reads the formatted name a contact card carries.
func nameFromVCard(card string) string {
	for _, line := range strings.Split(card, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if !strings.HasPrefix(strings.ToUpper(line), "FN") {
			continue
		}
		if colon := strings.Index(line, ":"); colon >= 0 && colon+1 < len(line) {
			if name := strings.TrimSpace(line[colon+1:]); name != "" {
				return name
			}
		}
	}
	return ""
}

func firstNonNilPoll(msg *waE2E.Message) *waE2E.PollCreationMessage {
	for _, poll := range []*waE2E.PollCreationMessage{
		msg.GetPollCreationMessage(), msg.GetPollCreationMessageV2(), msg.GetPollCreationMessageV3(),
		msg.GetPollCreationMessageV5(), msg.GetPollCreationMessageV6(),
	} {
		if poll != nil {
			return poll
		}
	}
	return nil
}

// applyLinkPreview copies the preview the sender's client resolved when the
// message was written. Nothing is fetched here, so opening a conversation
// never contacts the sites that were linked.
func applyLinkPreview(m *model.Message, v *waE2E.ExtendedTextMessage) {
	url := strings.TrimSpace(v.GetMatchedText())
	if url == "" {
		return
	}
	m.LinkURL = url
	m.LinkTitle = strings.TrimSpace(v.GetTitle())
	m.LinkDescription = strings.TrimSpace(v.GetDescription())
}

// quotedPreview describes the copy of the quoted message WhatsApp attaches to
// a reply. It is the only description available when the quoted message itself
// is older than this device's history.
func quotedPreview(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	switch {
	case strings.TrimSpace(msg.GetConversation()) != "":
		return strings.TrimSpace(msg.GetConversation())
	case msg.GetExtendedTextMessage() != nil:
		return strings.TrimSpace(msg.GetExtendedTextMessage().GetText())
	case msg.GetImageMessage() != nil:
		return firstNonEmpty(strings.TrimSpace(msg.GetImageMessage().GetCaption()), "Image")
	case msg.GetVideoMessage() != nil:
		return firstNonEmpty(strings.TrimSpace(msg.GetVideoMessage().GetCaption()), "Video")
	case msg.GetAudioMessage() != nil:
		return "Audio"
	case msg.GetDocumentMessage() != nil:
		return firstNonEmpty(strings.TrimSpace(msg.GetDocumentMessage().GetFileName()), "Document")
	case msg.GetStickerMessage() != nil:
		return "Sticker"
	case msg.GetContactMessage() != nil:
		return firstNonEmpty(strings.TrimSpace(msg.GetContactMessage().GetDisplayName()), "Contact")
	case msg.GetLocationMessage() != nil:
		return firstNonEmpty(strings.TrimSpace(msg.GetLocationMessage().GetName()), "Location")
	}
	return ""
}

func applyContext(m *model.Message, ctx *waE2E.ContextInfo) {
	if ctx != nil {
		m.ReplyTo = ctx.GetStanzaID()
		// A reply carries its own copy of what it answers, which is what the
		// preview shows until - and unless - the quoted message itself is
		// found in this device's history.
		m.ReplyPreview = quotedPreview(ctx.GetQuotedMessage())
		if participant := ctx.GetParticipant(); participant != "" {
			m.ReplySender = displayJID(participant)
		}
		// A forward chain carries its length in the context. WhatsApp marks
		// the message as forwarded without a score for the first hop, so the
		// flag alone still has to count as one.
		m.ForwardingScore = int(ctx.GetForwardingScore())
		if ctx.GetIsForwarded() && m.ForwardingScore < 1 {
			m.ForwardingScore = 1
		}
	}
}
func downloadableFromMessage(msg *waE2E.Message) whatsmeow.DownloadableMessage {
	if msg == nil {
		return nil
	}
	switch {
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage()
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage()
	case msg.GetAudioMessage() != nil:
		return msg.GetAudioMessage()
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage()
	case msg.GetStickerMessage() != nil:
		return msg.GetStickerMessage()
	}
	return nil
}
func safeName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "@", "_", ":", "_")
	return r.Replace(s)
}
func extensionForMIME(m string) string {
	if exts, _ := mimeExtensions(m); len(exts) > 0 {
		return exts[0]
	}
	return ""
}
func mimeExtensions(m string) ([]string, error) { return mime.ExtensionsByType(m) }
func displayJID(jid string) string {
	if i := strings.IndexByte(jid, '@'); i > 0 {
		return jid[:i]
	}
	return jid
}

func normalizeMuteTime(value uint64) int64 {
	if value == 0 {
		return 0
	}
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	result := int64(value)
	if result < 1_000_000_000_000 {
		result *= 1000
	}
	return result
}
