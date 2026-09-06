package whatsapp

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// mutedForever is what WhatsApp stores when a conversation is muted with no
// end date. It is far enough ahead to mean "until this is undone".
const mutedForever = 100 * 365 * 24 * time.Hour

// Pinning, muting, archiving and marking a conversation read are app-state
// changes: they belong to the account, not to this device, so each one is sent
// to WhatsApp and applied locally so the list responds immediately.

func (c *Client) SetChatPinned(ctx context.Context, chatJID string, pinned bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildPin(target, pinned)); err != nil {
		return err
	}
	if err := c.store.UpdateChatPinned(ctx, chatJID, pinned); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "pinned": pinned}})
	return nil
}

// SetChatMuted mutes for the given duration. WhatsApp Web offers eight hours,
// one week and "always"; a duration of zero or less is that "always", which the
// protocol expresses as an end date far in the future rather than as a flag.
// DeleteChat removes the conversation from every linked device. WhatsApp keys
// the delete on the last message, so a chat with no messages left cannot be
// deleted through app state; the local rows still go, which is what the user
// asked for and what a later history sync would restore if the server still
// has it.
func (c *Client) DeleteChat(ctx context.Context, chatJID string) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	last, lastErr := c.store.NewestMessage(ctx, chatJID)
	if lastErr == nil && strings.TrimSpace(last.ID) != "" {
		key := &waCommon.MessageKey{
			RemoteJID: proto.String(target.String()),
			FromMe:    proto.Bool(last.FromMe),
			ID:        proto.String(last.ID),
		}
		patch := appstate.BuildDeleteChat(target, time.UnixMilli(last.Timestamp), key, false)
		if err := c.wa.SendAppState(ctx, patch); err != nil {
			return err
		}
	}
	if err := c.store.DeleteChat(ctx, chatJID); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.deleted", Data: map[string]any{"jid": chatJID}})
	return nil
}

// ExportChat writes a conversation to a text file, the way WhatsApp Web's
// "Export chat" does. Only the transcript: attachments stay where they are.
//
// The path comes from the person at the keyboard through their own file dialog,
// so it is theirs to choose; it is still required to be absolute, and the file
// is created with owner-only permissions because a transcript is as private as
// the conversation it came from.
func (c *Client) ExportChat(ctx context.Context, chatJID, path string) (string, error) {
	if strings.TrimSpace(chatJID) == "" {
		return "", errors.New("chat_jid is required")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("an absolute destination path is required")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	if err := c.store.ExportChatTranscript(ctx, chatJID, func(line string) error {
		_, writeErr := writer.WriteString(line)
		return writeErr
	}); err != nil {
		return "", err
	}
	if err := writer.Flush(); err != nil {
		return "", err
	}
	return path, nil
}

// SetChatDisappearing turns disappearing messages on or off for a conversation.
// WhatsApp offers 24 hours, 7 days and 90 days; zero turns the feature off.
func (c *Client) SetChatDisappearing(ctx context.Context, chatJID string, seconds int64) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if seconds < 0 {
		return errors.New("a disappearing-message timer cannot be negative")
	}
	if err := c.wa.SetDisappearingTimer(ctx, target, time.Duration(seconds)*time.Second, time.Now()); err != nil {
		return err
	}
	if err := c.store.SetChatDisappearing(ctx, chatJID, seconds); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "disappearing_seconds": seconds}})
	return nil
}

// SetChatFavorite adds or removes a chat from the favourites list.
//
// whatsmeow has no builder for this one. Favourites are not a per-chat flag in
// app state: the whole list travels as a single "favorites" mutation, so the
// current list is read back, edited and sent complete. Sending only the change
// would clear every other favourite on the phone.
func (c *Client) SetChatFavorite(ctx context.Context, chatJID string, favorite bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	current, err := c.store.FavoriteChatJIDs(ctx)
	if err != nil {
		return err
	}
	wanted := make([]string, 0, len(current)+1)
	for _, jid := range current {
		if jid == target.String() || jid == chatJID {
			continue
		}
		wanted = append(wanted, jid)
	}
	if favorite {
		wanted = append([]string{target.String()}, wanted...)
	}
	entries := make([]*waSyncAction.FavoritesAction_Favorite, 0, len(wanted))
	for _, jid := range wanted {
		entries = append(entries, &waSyncAction.FavoritesAction_Favorite{ID: proto.String(jid)})
	}
	patch := appstate.PatchInfo{
		Type: appstate.WAPatchRegular,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexFavorites},
			Version: 3,
			Value: &waSyncAction.SyncActionValue{
				FavoritesAction: &waSyncAction.FavoritesAction{Favorites: entries},
			},
		}},
	}
	if err := c.wa.SendAppState(ctx, patch); err != nil {
		return err
	}
	if err := c.store.UpdateChatFavorites(ctx, wanted); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "favorite": favorite}})
	return nil
}

// ClearChat empties a conversation on every linked device while keeping the
// chat itself, which is what WhatsApp Web's "Clear chat" does.
//
// Like the delete action, this is keyed on the last message, so a conversation
// with nothing in it has nothing to clear remotely; the local rows still go.
func (c *Client) ClearChat(ctx context.Context, chatJID string) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	last, lastErr := c.store.NewestMessage(ctx, chatJID)
	if lastErr == nil && strings.TrimSpace(last.ID) != "" {
		key := &waCommon.MessageKey{
			RemoteJID: proto.String(target.String()),
			FromMe:    proto.Bool(last.FromMe),
			ID:        proto.String(last.ID),
		}
		messageRange := &waSyncAction.SyncActionMessageRange{
			LastMessageTimestamp: proto.Int64(time.UnixMilli(last.Timestamp).Unix()),
			Messages: []*waSyncAction.SyncActionMessage{{
				Key:       key,
				Timestamp: proto.Int64(time.UnixMilli(last.Timestamp).Unix()),
			}},
		}
		// The index carries the chat, then a starred-message flag, then a
		// delete-media flag; whatsmeow reads the fourth slot as the media one
		// when the action comes back from another device.
		patch := appstate.PatchInfo{
			Type: appstate.WAPatchRegularHigh,
			Mutations: []appstate.MutationInfo{{
				Index:   []string{appstate.IndexClearChat, target.String(), "1", "0"},
				Version: 6,
				Value: &waSyncAction.SyncActionValue{
					ClearChatAction: &waSyncAction.ClearChatAction{MessageRange: messageRange},
				},
			}},
		}
		if err := c.wa.SendAppState(ctx, patch); err != nil {
			return err
		}
	}
	if err := c.store.ClearChatMessages(ctx, chatJID); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.cleared", Data: map[string]any{"jid": chatJID}})
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID}})
	return nil
}

func (c *Client) SetChatMuted(ctx context.Context, chatJID string, muted bool, duration time.Duration) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if duration <= 0 {
		duration = mutedForever
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildMute(target, muted, duration)); err != nil {
		return err
	}
	var until int64
	if muted {
		until = time.Now().Add(duration).UnixMilli()
	}
	if err := c.store.UpdateChatMuted(ctx, chatJID, until); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "muted": muted, "muted_until": until}})
	return nil
}

func (c *Client) SetChatArchived(ctx context.Context, chatJID string, archived bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	timestamp, key := c.lastMessageAnchor(ctx, chatJID)
	if err := c.wa.SendAppState(ctx, appstate.BuildArchive(target, archived, timestamp, key)); err != nil {
		return err
	}
	if err := c.store.UpdateChatArchived(ctx, chatJID, archived); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "archived": archived}})
	return nil
}

// MarkAllChatsRead clears every unread conversation, which is what WhatsApp
// Web's "Mark all as read" does. One failure does not abandon the rest: the
// count of chats actually cleared is returned so the caller can say what
// happened rather than reporting a blanket success.
// CreateGroup makes a new group and records it locally so the conversation is
// selectable straight away, instead of waiting for the group's own history
// sync to arrive.
// SetContactBlocked blocks or unblocks a contact. The block list belongs to the
// account, so WhatsApp is the source of truth: nothing is cached here beyond
// what BlockedContacts reads back.
// CreateLabel makes a new chat list. WhatsApp keys lists by a small integer, so
// the id comes from the store rather than being invented here.
func (c *Client) CreateLabel(ctx context.Context, name string, color int) (model.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Label{}, errors.New("a list needs a name")
	}
	id, err := c.store.NextLabelID(ctx)
	if err != nil {
		return model.Label{}, err
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildLabelEdit(id, name, int32(color), false)); err != nil {
		return model.Label{}, err
	}
	label := model.Label{ID: id, Name: name, Color: color}
	if err := c.store.UpsertLabel(ctx, label, false); err != nil {
		return model.Label{}, err
	}
	c.emit(gateway.Event{Name: "labels.updated"})
	return label, nil
}

// SetChatLabeled puts a conversation in a list, or takes it out.
func (c *Client) SetChatLabeled(ctx context.Context, chatJID, labelID string, labeled bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(labelID) == "" {
		return errors.New("label_id is required")
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildLabelChat(target, labelID, labeled)); err != nil {
		return err
	}
	if err := c.store.SetChatLabeled(ctx, chatJID, labelID, labeled); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "labels.updated"})
	return nil
}

func (c *Client) SetContactBlocked(ctx context.Context, chatJID string, blocked bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if target.Server == types.GroupServer {
		return errors.New("a group cannot be blocked")
	}
	action := waEvents.BlocklistChangeActionUnblock
	if blocked {
		action = waEvents.BlocklistChangeActionBlock
	}
	if _, err := c.wa.UpdateBlocklist(ctx, target.ToNonAD(), action); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "contact.blocked", Data: map[string]any{"jid": chatJID, "blocked": blocked}})
	return nil
}

// BlockedContacts reports who is currently blocked, so the menus can offer
// "Block" or "Unblock" rather than guessing.
func (c *Client) BlockedContacts(ctx context.Context) ([]string, error) {
	list, err := c.wa.GetBlocklist(ctx)
	if err != nil {
		return nil, err
	}
	blocked := make([]string, 0, len(list.JIDs))
	for _, jid := range list.JIDs {
		blocked = append(blocked, jid.String())
	}
	return blocked, nil
}

func (c *Client) CreateGroup(ctx context.Context, name string, participants []string) (model.Chat, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Chat{}, errors.New("a group needs a name")
	}
	if len(participants) == 0 {
		return model.Chat{}, errors.New("a group needs at least one other participant")
	}
	targets := make([]types.JID, 0, len(participants))
	for _, participant := range participants {
		jid, err := types.ParseJID(strings.TrimSpace(participant))
		if err != nil {
			return model.Chat{}, err
		}
		targets = append(targets, jid)
	}
	info, err := c.wa.CreateGroup(ctx, whatsmeow.ReqCreateGroup{Name: name, Participants: targets})
	if err != nil {
		return model.Chat{}, err
	}
	chat := model.Chat{JID: info.JID.String(), Title: name, IsGroup: true, LastMessageAt: time.Now().UnixMilli()}
	if err := c.store.UpsertChat(ctx, chat); err != nil {
		return model.Chat{}, err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chat.JID, "title": name}})
	return chat, nil
}

func (c *Client) MarkAllChatsRead(ctx context.Context) (int, error) {
	chats, err := c.store.ListChats(ctx, 1000, 0, "")
	if err != nil {
		return 0, err
	}
	archived, err := c.store.ListArchivedChats(ctx, 1000, 0, "")
	if err != nil {
		return 0, err
	}
	cleared := 0
	var firstErr error
	for _, chat := range append(chats, archived...) {
		if chat.UnreadCount <= 0 {
			continue
		}
		if err := c.SetChatRead(ctx, chat.JID, true); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		cleared++
	}
	return cleared, firstErr
}

func (c *Client) SetChatRead(ctx context.Context, chatJID string, read bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	timestamp, key := c.lastMessageAnchor(ctx, chatJID)
	if err := c.wa.SendAppState(ctx, appstate.BuildMarkChatAsRead(target, read, timestamp, key)); err != nil {
		return err
	}
	if read {
		if err := c.store.MarkChatRead(ctx, chatJID); err != nil {
			return err
		}
	} else if err := c.store.MarkChatUnread(ctx, chatJID); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]any{"jid": chatJID, "read": read}})
	return nil
}

// starSenderJID picks the sender that belongs in a star's app-state key.
//
// The key's last slot is the literal "0" whenever the message needs no separate
// sender: anything this device sent, and anything in a one-to-one chat, where
// the sender is already the chat. BuildStar writes that "0" when the sender and
// the chat share a user, so naming the chat itself is how the slot is emptied —
// an empty JID would serialise to "" instead, which WhatsApp does not read back
// as "no sender". Only a group message from someone else carries a real JID.
func starSenderJID(target types.JID, senderJID string, fromMe bool) (types.JID, error) {
	if fromMe || strings.TrimSpace(senderJID) == "" {
		return target, nil
	}
	return types.ParseJID(senderJID)
}

// SetMessageStarred stars or unstars one message for the whole account. The
// patch names the message by chat, sender and direction, because WhatsApp
// stores a star against the message key rather than against a local row.
func (c *Client) SetMessageStarred(ctx context.Context, chatJID, messageID, senderJID string, fromMe, starred bool) error {
	target, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	sender, err := starSenderJID(target, senderJID, fromMe)
	if err != nil {
		return err
	}
	patch := appstate.BuildStar(target, sender, types.MessageID(messageID), fromMe, starred)
	if err := c.wa.SendAppState(ctx, patch); err != nil {
		return err
	}
	if err := c.store.SetMessageStarred(ctx, chatJID, messageID, starred); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "message.starred", Data: map[string]any{
		"chat_jid": chatJID, "message_id": messageID, "starred": starred,
	}})
	return nil
}

// ForwardMessage re-sends an existing message into another conversation.
//
// WhatsApp has no "forward" operation on the wire: a forward is an ordinary
// send whose context is marked as forwarded, so the receiving client can label
// it. Media is re-sent from the copy already stored locally rather than
// re-downloaded, which also means a message whose file was never fetched
// cannot be forwarded.
func (c *Client) ForwardMessage(ctx context.Context, fromChatJID, messageID, toChatJID string) (model.Message, error) {
	if strings.TrimSpace(toChatJID) == "" {
		return model.Message{}, errors.New("to_chat_jid is required")
	}
	original, err := c.store.GetMessage(ctx, fromChatJID, messageID)
	if err != nil {
		return model.Message{}, err
	}
	if original.Revoked {
		return model.Message{}, errors.New("a deleted message cannot be forwarded")
	}
	if original.MediaPath == "" {
		switch original.Kind {
		case "image", "video", "audio", "document", "sticker":
			return model.Message{}, errors.New("download this attachment before forwarding it")
		}
	}
	if original.MediaPath != "" {
		if _, statErr := os.Stat(original.MediaPath); statErr != nil {
			return model.Message{}, errors.New("download this attachment before forwarding it")
		}
		return c.SendMedia(ctx, gateway.MediaRequest{
			ChatJID:         toChatJID,
			Path:            original.MediaPath,
			Caption:         original.Body,
			Document:        original.Kind == "document",
			ForwardingScore: original.ForwardingScore + 1,
		})
	}
	if strings.TrimSpace(original.Body) == "" {
		return model.Message{}, errors.New("this message has nothing to forward")
	}
	chat, err := types.ParseJID(toChatJID)
	if err != nil {
		return model.Message{}, err
	}
	payload := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String(original.Body),
		ContextInfo: &waE2E.ContextInfo{
			IsForwarded: proto.Bool(true),
			// WhatsApp shows "forwarded many times" once this passes its
			// threshold, so a chain keeps counting rather than restarting.
			ForwardingScore: proto.Uint32(uint32(original.ForwardingScore) + 1),
		},
	}}
	resp, err := c.wa.SendMessage(ctx, chat, payload)
	if err != nil {
		return model.Message{}, err
	}
	return model.Message{
		ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(), Kind: "text", Body: original.Body,
		FromMe: true, Status: "sent", ForwardingScore: original.ForwardingScore + 1,
	}, nil
}

// lastMessageAnchor describes the newest message of a conversation, which
// WhatsApp requires alongside an archive or read change.
func (c *Client) lastMessageAnchor(ctx context.Context, chatJID string) (time.Time, *waCommon.MessageKey) {
	newest, err := c.store.NewestMessage(ctx, chatJID)
	if err != nil || newest.ID == "" {
		return time.Now(), nil
	}
	return time.UnixMilli(newest.Timestamp), &waCommon.MessageKey{
		RemoteJID: proto.String(chatJID),
		FromMe:    proto.Bool(newest.FromMe),
		ID:        proto.String(newest.ID),
	}
}
