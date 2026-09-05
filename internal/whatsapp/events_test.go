package whatsapp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"errors"

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
	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func TestMessageFromEventExtractsReplyAndMedia(t *testing.T) {
	evt := &waEvents.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer), Sender: types.NewJID("456", types.DefaultUserServer)}, ID: "m1", PushName: "Alice", Timestamp: time.Unix(100, 0)}, Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("look"), Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(42), ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("prior")}}}}
	m := messageFromEvent(evt)
	if m.Kind != "image" || m.Body != "look" || m.ReplyTo != "prior" || m.MediaSize != 42 {
		t.Fatalf("unexpected message: %#v", m)
	}
}

func TestMessageFromEventExtractsReaction(t *testing.T) {
	evt := &waEvents.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "reaction", Timestamp: time.Now()}, Message: &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: &waCommon.MessageKey{ID: proto.String("target")}, Text: proto.String("ok")}}}
	m := messageFromEvent(evt)
	if m.Kind != "reaction" || m.ReplyTo != "target" || m.Body != "ok" {
		t.Fatalf("unexpected reaction: %#v", m)
	}
}

func TestSendReactionStoresAndEmitsSuccessfulOwnReaction(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	chat := types.NewJID("123", types.DefaultUserServer)
	if err := st.UpsertMessage(ctx, model.Message{
		ID: "target", ChatJID: chat.String(), SenderJID: chat.String(),
		Timestamp: 1, Kind: "text", Body: "hello", Status: "received",
	}, "Alice", false); err != nil {
		t.Fatal(err)
	}

	c := &Client{
		store: st,
		subs:  make(map[uint64]func(gateway.Event)),
		sendReactionMessage: func(context.Context, types.JID, types.JID, types.MessageID, string) (whatsmeow.SendResponse, error) {
			return whatsmeow.SendResponse{
				Timestamp: time.UnixMilli(2),
				Sender:    types.NewJID("me", types.DefaultUserServer),
			}, nil
		},
	}
	var emitted gateway.Event
	c.Subscribe(func(evt gateway.Event) { emitted = evt })
	if err := c.SendReaction(ctx, chat.String(), "target", chat.String(), "👍"); err != nil {
		t.Fatal(err)
	}

	page, err := st.ListMessages(ctx, chat.String(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || len(page.Messages[0].Reactions) != 1 {
		t.Fatalf("successful own reaction was not attached to the message: %#v", page.Messages)
	}
	got := page.Messages[0].Reactions[0]
	if got.Emoji != "👍" || got.SenderJID != "me@s.whatsapp.net" {
		t.Fatalf("unexpected stored own reaction: %#v", got)
	}
	if emitted.Name != "message.reaction" {
		t.Fatalf("successful own reaction did not emit a refresh event: %#v", emitted)
	}

	// WhatsApp may later echo the reaction back to this linked device. The echo
	// must update the optimistic row, not count the same person's reaction twice.
	c.handleMessage(&waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{
			Chat: chat, Sender: types.NewJID("me", types.DefaultUserServer), IsFromMe: true,
		}, ID: "reaction-echo", Timestamp: time.UnixMilli(3)},
		Message: &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{ID: proto.String("target")}, Text: proto.String("👍"),
		}},
	})
	page, err = st.ListMessages(ctx, chat.String(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages[0].Reactions) != 1 {
		t.Fatalf("own reaction echo created a duplicate: %#v", page.Messages[0].Reactions)
	}
}

func TestPresenceEventUsesCanonicalChatAndKeepsUnknownLastSeenEmpty(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.UpsertChat(ctx, model.Chat{JID: "alice@lid", Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkChatAliases(ctx, "alice@lid", "15551234567@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event))}
	var emitted gateway.Event
	c.Subscribe(func(event gateway.Event) { emitted = event })
	c.handleEvent(&waEvents.Presence{From: types.NewJID("15551234567", types.DefaultUserServer), Unavailable: true})
	data := emitted.Data.(map[string]any)
	if emitted.Name != "contact.presence" || data["jid"] != "alice@lid" || data["last_seen"] != int64(0) {
		t.Fatalf("unexpected presence event: %#v", emitted)
	}
}

func TestPresenceEventPersistsDisclosedLastSeen(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.UpsertChat(ctx, model.Chat{JID: "alice@lid", Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkChatAliases(ctx, "alice@lid", "15551234567@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event))}
	c.handleEvent(&waEvents.Presence{
		From:        types.NewJID("15551234567", types.DefaultUserServer),
		Unavailable: true,
		LastSeen:    time.UnixMilli(1700000000000),
	})
	info, err := st.ChatInfo(ctx, "alice@lid")
	if err != nil {
		t.Fatal(err)
	}
	if info.LastSeen != 1700000000000 {
		t.Fatalf("last seen = %d, want persisted presence timestamp", info.LastSeen)
	}
}

func TestPinInChatEventUpdatesPinnedMessageProjection(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	chat := types.NewJID("123", types.DefaultUserServer)
	if err := st.UpsertMessage(ctx, model.Message{ID: "target", ChatJID: chat.String(), SenderJID: chat.String(), Timestamp: 1, Kind: "text", Body: "Pinned body", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	at := time.Now().Truncate(time.Second)
	c.handleMessage(&waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}, ID: "pin-action", Timestamp: at},
		Message: &waE2E.Message{
			MessageContextInfo: &waE2E.MessageContextInfo{MessageAddOnDurationInSecs: proto.Uint32(86400)},
			PinInChatMessage:   &waE2E.PinInChatMessage{Key: &waCommon.MessageKey{ID: proto.String("target")}, Type: waE2E.PinInChatMessage_PIN_FOR_ALL.Enum()},
		},
	})
	info, err := st.ChatInfo(ctx, chat.String())
	if err != nil || info.PinnedMessage == nil || info.PinnedMessage.ID != "target" || info.PinnedUntil != at.Add(24*time.Hour).UnixMilli() {
		t.Fatalf("pin event was not projected: info=%#v err=%v", info, err)
	}
	c.handleMessage(&waEvents.Message{
		Info:    types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}, ID: "unpin-action", Timestamp: at.Add(time.Minute)},
		Message: &waE2E.Message{PinInChatMessage: &waE2E.PinInChatMessage{Key: &waCommon.MessageKey{ID: proto.String("target")}, Type: waE2E.PinInChatMessage_UNPIN_FOR_ALL.Enum()}},
	})
	info, err = st.ChatInfo(ctx, chat.String())
	if err != nil || info.PinnedMessage != nil {
		t.Fatalf("unpin event was not projected: info=%#v err=%v", info, err)
	}
}

func TestMessageFromEventDropsTransportOnlyEnvelope(t *testing.T) {
	evt := &waEvents.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "transport", Timestamp: time.Now()}, Message: &waE2E.Message{SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{}}}
	if m := messageFromEvent(evt); m.ID != "" {
		t.Fatalf("transport-only envelope must not become a chat bubble: %#v", m)
	}
}

func TestProtocolMessageWithNothingToShowIsNotStored(t *testing.T) {
	chat := types.NewJID("123", types.DefaultUserServer)
	settings := &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}, ID: "ephemeral", Timestamp: time.Now()},
		Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_EPHEMERAL_SETTING.Enum(),
		}},
	}
	if m := messageFromEvent(settings); m.ID != "" || m.Kind != "" {
		t.Fatalf("a protocol message with nothing to read drew an empty bubble: %#v", m)
	}

	revoke := &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}, ID: "revoke", Timestamp: time.Now()},
		Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
			Key:  &waCommon.MessageKey{ID: proto.String("target")},
		}},
	}
	m := messageFromEvent(revoke)
	if m.Kind != "system" || !m.Revoked || m.ReplyTo != "target" {
		t.Fatalf("a revocation still has to be recognised: %#v", m)
	}
}

func TestCallLogFromRecordMapsAppStateRecord(t *testing.T) {
	record := &waSyncAction.CallLogRecord{
		CallID:         proto.String("call-1"),
		CallCreatorJID: proto.String("me@s.whatsapp.net"),
		Participants: []*waSyncAction.CallLogRecord_ParticipantInfo{{
			UserJID: proto.String("alice@s.whatsapp.net"),
		}},
		StartTime:  proto.Int64(1_700_000_000),
		Duration:   proto.Int64(42),
		IsIncoming: proto.Bool(false),
		IsVideo:    proto.Bool(true),
		CallResult: waSyncAction.CallLogRecord_CONNECTED.Enum(),
	}

	call, ok := callLogFromRecord(record)
	if !ok {
		t.Fatal("valid app-state call record was rejected")
	}
	if call.ID != "call-1" || call.PeerJID != "alice@s.whatsapp.net" ||
		call.Timestamp != 1_700_000_000_000 || call.Duration != 42 ||
		call.Incoming || !call.Video || call.Result != "connected" {
		t.Fatalf("unexpected call log: %#v", call)
	}
}

func TestPinAppStateEventUpdatesLocalChat(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := types.NewJID("123", types.DefaultUserServer)
	if err := st.UpsertChat(ctx, model.Chat{JID: jid.String(), Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	pinnedAt := time.Unix(1_700_000_000, 0)
	c.handleEvent(&waEvents.Pin{JID: jid, Timestamp: pinnedAt, Action: &waSyncAction.PinAction{Pinned: proto.Bool(true)}})
	chat, err := st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Pinned {
		t.Fatal("pin app-state event did not update the local chat")
	}
	if chat.PinnedAt != pinnedAt.UnixMilli() {
		t.Fatalf("pin timestamp = %d, want %d", chat.PinnedAt, pinnedAt.UnixMilli())
	}
}

func TestMarkChatAsReadAppStateEventUpdatesUnreadCount(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := types.NewJID("123", types.DefaultUserServer)
	if err := st.ApplyChatSnapshot(ctx, model.Chat{JID: jid.String(), Title: "Alice", UnreadCount: 4}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	c.handleEvent(&waEvents.MarkChatAsRead{JID: jid, Timestamp: time.Now(), Action: &waSyncAction.MarkChatAsReadAction{Read: proto.Bool(true)}})
	chat, err := st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 0 {
		t.Fatalf("mark-as-read event left %d unread messages", chat.UnreadCount)
	}

	c.handleEvent(&waEvents.MarkChatAsRead{JID: jid, Action: &waSyncAction.MarkChatAsReadAction{Read: proto.Bool(false)}})
	chat, err = st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 1 {
		t.Fatalf("mark-as-unread event set unread count to %d, want 1", chat.UnreadCount)
	}
}

func TestFullAppStateReplayDoesNotReplaceCurrentUnreadState(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := types.NewJID("123", types.DefaultUserServer)
	for _, msg := range []model.Message{
		{ID: "read", ChatJID: jid.String(), Timestamp: 1_000, Kind: "text", Body: "old", Status: "received"},
		{ID: "unread", ChatJID: jid.String(), Timestamp: 3_000, Kind: "text", Body: "new", Status: "received"},
	} {
		if err := st.UpsertMessage(ctx, msg, "Alice", true); err != nil {
			t.Fatal(err)
		}
	}
	c := &Client{store: st}
	for _, read := range []bool{true, false} {
		c.handleEvent(&waEvents.MarkChatAsRead{
			JID:          jid,
			Timestamp:    time.Unix(10, 0),
			FromFullSync: true,
			Action: &waSyncAction.MarkChatAsReadAction{
				Read:         proto.Bool(read),
				MessageRange: &waSyncAction.SyncActionMessageRange{LastMessageTimestamp: proto.Int64(1)},
			},
		})
	}
	chat, err := st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 2 {
		t.Fatalf("historical app-state replay changed unread count to %d, want 2", chat.UnreadCount)
	}
}

func TestSelfReadReceiptClearsChatUnreadCount(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := types.NewJID("123", types.DefaultUserServer)
	if err := st.ApplyChatSnapshot(ctx, model.Chat{JID: jid.String(), Title: "Alice", UnreadCount: 3}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	c.handleReceipt(&waEvents.Receipt{
		MessageSource: types.MessageSource{Chat: jid, IsFromMe: true},
		MessageIDs:    []types.MessageID{"message-1"},
		Timestamp:     time.Now(),
		Type:          types.ReceiptTypeReadSelf,
	})
	chat, err := st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 0 {
		t.Fatalf("self-read receipt left %d unread messages", chat.UnreadCount)
	}
}

func TestFavoritesAppStateEventUpdatesLocalChats(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := "123@s.whatsapp.net"
	if err := st.UpsertChat(ctx, model.Chat{JID: jid, Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	c.handleAppState(&waEvents.AppState{
		Index: []string{appstate.IndexFavorites},
		SyncActionValue: &waSyncAction.SyncActionValue{FavoritesAction: &waSyncAction.FavoritesAction{
			Favorites: []*waSyncAction.FavoritesAction_Favorite{{ID: proto.String(jid)}},
		}},
	})
	chat, err := st.GetChat(ctx, jid)
	if err != nil || !chat.Favorite {
		t.Fatalf("favorites app-state event was not persisted: chat=%#v err=%v", chat, err)
	}
}

func TestMissingAppStateKeyDefersBackfillWithoutUserError(t *testing.T) {
	err := fmt.Errorf("verify snapshot: %w", appstate.ErrKeyNotFound)
	if !shouldDeferAppStateBackfill(err) {
		t.Fatal("missing app-state key should defer backfill until key share arrives")
	}
}

func TestSyncDirectoryContactMergesPhoneAndLIDHistory(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	pn := types.NewJID("573112522689", types.DefaultUserServer)
	lid := types.NewJID("201850896818405", types.HiddenUserServer)
	for _, message := range []model.Message{
		{ID: "old-phone", ChatJID: pn.String(), Timestamp: 1, Kind: "text", Body: "older history", Status: "received"},
		{ID: "new-lid", ChatJID: lid.String(), Timestamp: 2, Kind: "text", Body: "newer history", Status: "received"},
	} {
		if err := st.UpsertMessage(ctx, message, "", false); err != nil {
			t.Fatal(err)
		}
	}

	if err := syncDirectoryContact(ctx, st, pn, lid, "Alice"); err != nil {
		t.Fatal(err)
	}
	page, err := st.ListMessages(ctx, lid.String(), 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[0].ID != "old-phone" || page.Messages[1].ID != "new-lid" {
		t.Fatalf("directory sync did not consolidate history: %#v", page.Messages)
	}
	chats, err := st.ListChats(ctx, 20, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].JID != lid.String() || chats[0].Title != "Alice" {
		t.Fatalf("expected one named canonical chat, got %#v", chats)
	}
}

func TestAuthoritativeContactNameDoesNotUsePushName(t *testing.T) {
	if got := authoritativeContactName(types.ContactInfo{PushName: "zone yalo"}); got != "" {
		t.Fatalf("push-only contact name was treated as authoritative: %q", got)
	}
	if got := authoritativeContactName(types.ContactInfo{FullName: "Adony Robles Lopez", PushName: "zone yalo"}); got != "Adony Robles Lopez" {
		t.Fatalf("full contact name was not preferred: %q", got)
	}
}

func TestDirectoryPushNameUpgradesPlaceholderButPreservesSavedName(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	placeholder := types.NewJID("179452491378772", types.HiddenUserServer)
	if err := st.UpsertChat(ctx, model.Chat{JID: placeholder.String()}); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectoryContactInfo(ctx, st, placeholder, types.JID{}, types.ContactInfo{PushName: "Bancolombia"}); err != nil {
		t.Fatal(err)
	}
	chat, err := st.GetChat(ctx, placeholder.String())
	if err != nil || chat.Title != "Bancolombia" {
		t.Fatalf("push name did not upgrade placeholder: chat=%#v err=%v", chat, err)
	}

	saved := types.NewJID("1234", types.HiddenUserServer)
	if err := st.UpsertChat(ctx, model.Chat{JID: saved.String(), Title: "Saved name"}); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectoryContactInfo(ctx, st, saved, types.JID{}, types.ContactInfo{PushName: "Weaker name"}); err != nil {
		t.Fatal(err)
	}
	chat, err = st.GetChat(ctx, saved.String())
	if err != nil || chat.Title != "Saved name" {
		t.Fatalf("push name replaced saved contact: chat=%#v err=%v", chat, err)
	}
}

func TestJoinedGroupEventPreservesSynchronisedChatSettings(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := types.NewJID("120363000000000001", types.GroupServer)
	if err := st.ApplyChatSnapshot(ctx, model.Chat{JID: jid.String(), Title: "Team", MutedUntil: 9_999_999_999_999, Pinned: true, IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	c.handleEvent(&waEvents.JoinedGroup{GroupInfo: types.GroupInfo{JID: jid, GroupName: types.GroupName{Name: "Team"}}})
	chat, err := st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Pinned || chat.MutedUntil != 9_999_999_999_999 {
		t.Fatalf("group metadata event reset synchronised settings: %#v", chat)
	}
}

func TestOnDemandHistorySyncPreservesConversationSettings(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := st.ApplyChatSnapshot(ctx, model.Chat{JID: jid, Title: "Alice", MutedUntil: 9_999_999_999_999, Pinned: true, Archived: true}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	// Paging older history returns conversations without their settings; the
	// absent fields must not be interpreted as "unpinned, unmuted, unarchived".
	c.handleHistorySync(&waEvents.HistorySync{Data: &waHistorySync.HistorySync{
		SyncType:      waHistorySync.HistorySync_ON_DEMAND.Enum(),
		Conversations: []*waHistorySync.Conversation{{ID: proto.String(jid)}},
	}})
	chat, err := st.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Pinned || !chat.Archived || chat.MutedUntil != 9_999_999_999_999 || chat.Title != "Alice" {
		t.Fatalf("on-demand history page reset conversation settings: %#v", chat)
	}
}

func TestInitialHistorySyncAppliesConversationSettings(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := st.UpsertChat(ctx, model.Chat{JID: jid}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	c.handleHistorySync(&waEvents.HistorySync{Data: &waHistorySync.HistorySync{
		SyncType: waHistorySync.HistorySync_INITIAL_BOOTSTRAP.Enum(),
		Conversations: []*waHistorySync.Conversation{{
			ID:          proto.String(jid),
			Name:        proto.String("Alice Example"),
			Pinned:      proto.Uint32(1),
			Archived:    proto.Bool(true),
			MuteEndTime: proto.Uint64(4_102_444_800),
			UnreadCount: proto.Uint32(3),
		}},
	}})
	chat, err := st.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Pinned || !chat.Archived || chat.MutedUntil != 4_102_444_800_000 || chat.UnreadCount != 3 || chat.Title != "Alice Example" {
		t.Fatalf("initial history sync did not apply conversation settings: %#v", chat)
	}
}

func TestHistorySyncDoesNotReplaceResolvedContactNameWithPushName(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := st.UpsertChat(ctx, model.Chat{JID: jid, Title: "Adony Robles Lopez"}); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	c.handleHistorySync(&waEvents.HistorySync{Data: &waHistorySync.HistorySync{
		SyncType: waHistorySync.HistorySync_RECENT.Enum(),
		Conversations: []*waHistorySync.Conversation{{
			ID:   proto.String(jid),
			Name: proto.String("zone yalo"),
		}},
	}})
	chat, err := st.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Adony Robles Lopez" {
		t.Fatalf("history push name replaced resolved contact name: %q", chat.Title)
	}
}

func TestNotificationBodyDescribesMessages(t *testing.T) {
	cases := []struct {
		name  string
		msg   model.Message
		group bool
		want  string
	}{
		{"text", model.Message{Kind: "text", Body: "hello"}, false, "hello"},
		{"media without caption", model.Message{Kind: "image"}, false, "Image"},
		{"group prefixes sender", model.Message{Kind: "document", SenderName: "Alice"}, true, "Alice: Document"},
		{"direct chat omits sender", model.Message{Kind: "text", Body: "hi", SenderName: "Alice"}, false, "hi"},
		{"unicode kind", model.Message{Kind: "élan"}, false, "Élan"},
		{"empty", model.Message{}, false, ""},
	}
	for _, tc := range cases {
		if got := notificationBody(tc.msg, tc.group); got != tc.want {
			t.Errorf("%s: notificationBody = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestShouldNotifyMessageExcludesStatusUpdates(t *testing.T) {
	cases := []struct {
		name string
		msg  model.Message
		want bool
	}{
		{"incoming chat message", model.Message{ChatJID: "alice@lid"}, true},
		{"outgoing chat message", model.Message{ChatJID: "alice@lid", FromMe: true}, false},
		{"incoming status update", model.Message{ChatJID: "status@broadcast"}, false},
	}
	for _, tc := range cases {
		if got := shouldNotifyMessage(tc.msg); got != tc.want {
			t.Errorf("%s: shouldNotifyMessage = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNotificationTitlePrefersResolvedChatName(t *testing.T) {
	cases := []struct {
		name      string
		chat      model.Chat
		pushName  string
		chatJID   string
		wantTitle string
	}{
		{"saved contact beats stale push name", model.Chat{Title: "יאיר"}, "YBG", "972500000000@s.whatsapp.net", "יאיר"},
		{"push name fills an unnamed contact", model.Chat{}, "YBG", "972500000000@s.whatsapp.net", "YBG"},
		{"jid is the final fallback", model.Chat{}, "", "972500000000@s.whatsapp.net", "972500000000"},
	}
	for _, tc := range cases {
		if got := notificationTitle(tc.chat, tc.pushName, tc.chatJID); got != tc.wantTitle {
			t.Errorf("%s: notificationTitle = %q, want %q", tc.name, got, tc.wantTitle)
		}
	}
}

func TestMessageFromEventExtractsLinkPreview(t *testing.T) {
	evt := &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "link-1", Timestamp: time.Unix(100, 0)},
		Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String("look at this https://example.com/watch"),
			MatchedText: proto.String("https://example.com/watch"),
			Title:       proto.String("  Example page  "),
			Description: proto.String("What the page is about."),
		}},
	}
	m := messageFromEvent(evt)
	if m.LinkURL != "https://example.com/watch" || m.LinkTitle != "Example page" || m.LinkDescription != "What the page is about." {
		t.Fatalf("link preview not extracted: %#v", m)
	}
	if m.Body != "look at this https://example.com/watch" {
		t.Fatalf("body was altered: %q", m.Body)
	}
}

func TestMessageFromEventWithoutLinkLeavesPreviewEmpty(t *testing.T) {
	evt := &waEvents.Message{
		Info:    types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "plain", Timestamp: time.Unix(100, 0)},
		Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("no links here")}},
	}
	if m := messageFromEvent(evt); m.LinkURL != "" || m.LinkTitle != "" {
		t.Fatalf("a message without a link got a preview: %#v", m)
	}
}

func TestCachedLinkPreviewStoresSenderPicture(t *testing.T) {
	c := &Client{mediaDir: t.TempDir()}
	msg := model.Message{ChatJID: "alice@s.whatsapp.net", ID: "link-2", Kind: "text", LinkURL: "https://example.com"}
	raw := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{JPEGThumbnail: jpegBytes(t)}}
	withPreview := c.withCachedLinkPreview(msg, raw)
	if withPreview.LinkThumbnail == "" {
		t.Fatal("link preview picture was not cached")
	}
	// A message with no link must never get a preview picture.
	plain := c.withCachedLinkPreview(model.Message{ChatJID: msg.ChatJID, ID: "plain"}, raw)
	if plain.LinkThumbnail != "" {
		t.Fatalf("preview cached for a message with no link: %q", plain.LinkThumbnail)
	}
}

func TestVCardParsing(t *testing.T) {
	const card = "BEGIN:VCARD\r\nVERSION:3.0\r\nN:Robles;Adony;;;\r\nFN:Adony Robles\r\n" +
		"TEL;type=CELL;type=VOICE;waid=573112522689:+57 311 2522689\r\nEND:VCARD"
	if got := phoneFromVCard(card); got != "+57 311 2522689" {
		t.Fatalf("phone = %q", got)
	}
	if got := nameFromVCard(card); got != "Adony Robles" {
		t.Fatalf("name = %q", got)
	}
	if phoneFromVCard("BEGIN:VCARD\r\nFN:No Number\r\nEND:VCARD") != "" {
		t.Fatal("a card without a number produced one")
	}
	if nameFromVCard("") != "" || phoneFromVCard("") != "" {
		t.Fatal("an empty card produced details")
	}
}

func TestMessageFromEventReadsSharedContact(t *testing.T) {
	evt := &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("1", types.DefaultUserServer)}, ID: "c1", Timestamp: time.Unix(1, 0)},
		Message: &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
			DisplayName: proto.String("Adony Robles"),
			Vcard:       proto.String("BEGIN:VCARD\r\nFN:Adony Robles\r\nTEL;waid=573112522689:+57 311 2522689\r\nEND:VCARD"),
		}},
	}
	m := messageFromEvent(evt)
	if m.Kind != "contact" || m.ContactName != "Adony Robles" || m.ContactPhone != "+57 311 2522689" || m.ContactCount != 1 {
		t.Fatalf("shared contact not read: %#v", m)
	}
}

func TestMessageFromEventReadsSharedPlace(t *testing.T) {
	evt := &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("1", types.DefaultUserServer)}, ID: "l1", Timestamp: time.Unix(1, 0)},
		Message: &waE2E.Message{LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(4.60971),
			DegreesLongitude: proto.Float64(-74.08175),
			Name:             proto.String("Bogotá"),
			Address:          proto.String("Colombia"),
		}},
	}
	m := messageFromEvent(evt)
	if m.Kind != "location" || m.Latitude == 0 || m.Longitude == 0 || m.Body != "Bogotá" {
		t.Fatalf("shared place not read: %#v", m)
	}
}

func TestLocationThumbnailIsCached(t *testing.T) {
	c := &Client{mediaDir: t.TempDir()}
	path := c.cacheThumbnail(model.Message{ChatJID: "c@s.whatsapp.net", ID: "l2"},
		&waE2E.Message{LocationMessage: &waE2E.LocationMessage{JPEGThumbnail: jpegBytes(t)}})
	if path == "" {
		t.Fatal("the map picture of a shared place was not cached")
	}
}

func TestDeliveryFromWebStatus(t *testing.T) {
	cases := map[waWeb.WebMessageInfo_Status]string{
		waWeb.WebMessageInfo_PLAYED:       "played",
		waWeb.WebMessageInfo_READ:         "read",
		waWeb.WebMessageInfo_DELIVERY_ACK: "delivered",
		// Anything the server has merely accepted, or that failed, tells us
		// nothing better than what the message already says.
		waWeb.WebMessageInfo_SERVER_ACK: "",
		waWeb.WebMessageInfo_PENDING:    "",
		waWeb.WebMessageInfo_ERROR:      "",
	}
	for status, want := range cases {
		if got := deliveryFromWebStatus(status); got != want {
			t.Errorf("status %v = %q, want %q", status, got, want)
		}
	}
}

func TestHistorySyncKeepsDeliveryState(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const chat = "alice@s.whatsapp.net"
	// A message already known to be read must not fall back to "sent" when
	// history redelivers it.
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: chat, Timestamp: 5, Kind: "text",
		Body: "hi", FromMe: true, Status: "read"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: chat, Timestamp: 5, Kind: "text",
		Body: "hi", FromMe: true, Status: "sent"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	stored, err := st.GetMessage(ctx, chat, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "read" {
		t.Fatalf("delivery state regressed to %q", stored.Status)
	}
}

func TestConnectSweepRunsOneAtATime(t *testing.T) {
	c := &Client{}
	if !c.beginConnectSweep() {
		t.Fatal("the first sweep was refused")
	}
	// A reconnect while a sweep is running must not start a second one: the
	// two would suppress each other's page requests and then conclude the
	// conversations had no more history.
	if c.beginConnectSweep() {
		t.Fatal("a second sweep started alongside the first")
	}
	c.endConnectSweep()
	if !c.beginConnectSweep() {
		t.Fatal("a sweep could not start after the previous one finished")
	}
	c.endConnectSweep()
}

func TestReceiptsThatReportNoProgressLeaveTheMarkAlone(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	chat := types.NewJID("123", types.DefaultUserServer)
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: chat.String(), Timestamp: 1,
		Kind: "text", Body: "hi", FromMe: true, Status: "sent"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}

	// A failed send and an inactive device say nothing about delivery.
	for _, receiptType := range []types.ReceiptType{types.ReceiptTypeServerError, types.ReceiptTypeInactive} {
		c.handleReceipt(&waEvents.Receipt{
			MessageSource: types.MessageSource{Chat: chat},
			MessageIDs:    []types.MessageID{"m1"},
			Type:          receiptType,
			Timestamp:     time.Unix(10, 0),
		})
		stored, err := st.GetMessage(ctx, chat.String(), "m1")
		if err != nil {
			t.Fatal(err)
		}
		if stored.Status != "sent" {
			t.Fatalf("%v marked the message %q", receiptType, stored.Status)
		}
	}

	// A real delivery receipt still moves it along.
	c.handleReceipt(&waEvents.Receipt{
		MessageSource: types.MessageSource{Chat: chat},
		MessageIDs:    []types.MessageID{"m1"},
		Type:          types.ReceiptTypeDelivered,
		Timestamp:     time.Unix(20, 0),
	})
	stored, err := st.GetMessage(ctx, chat.String(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "delivered" {
		t.Fatalf("a delivery receipt left the message %q", stored.Status)
	}
}

func TestStarAppStateEventUpdatesLocalMessage(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	jid := types.NewJID("123", types.DefaultUserServer)
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: jid.String(),
		Timestamp: 10, Kind: "text", Body: "keep me"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}
	// Starring on the phone reaches a linked device only as this app-state
	// event, so without it the flag would never arrive here.
	c.handleEvent(&waEvents.Star{ChatJID: jid, MessageID: "m1", IsFromMe: false,
		Timestamp: time.Unix(1_700_000_000, 0),
		Action:    &waSyncAction.StarAction{Starred: proto.Bool(true)}})
	stored, err := st.GetMessage(ctx, jid.String(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Starred {
		t.Fatal("star app-state event did not star the local message")
	}
	c.handleEvent(&waEvents.Star{ChatJID: jid, MessageID: "m1", IsFromMe: false,
		Timestamp: time.Unix(1_700_000_100, 0),
		Action:    &waSyncAction.StarAction{Starred: proto.Bool(false)}})
	if stored, err = st.GetMessage(ctx, jid.String(), "m1"); err != nil {
		t.Fatal(err)
	}
	if stored.Starred {
		t.Fatal("unstarring on another device left the message starred")
	}
}

func TestMessageFromEventReadsForwardingScore(t *testing.T) {
	newEvent := func(context *waE2E.ContextInfo) *waEvents.Message {
		return &waEvents.Message{
			Info: types.MessageInfo{MessageSource: types.MessageSource{
				Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "m1", Timestamp: time.Unix(100, 0)},
			Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String("passed along"), ContextInfo: context}},
		}
	}
	// The first hop of a chain is flagged without a score, and it still has to
	// show the "Forwarded" label.
	first := messageFromEvent(newEvent(&waE2E.ContextInfo{IsForwarded: proto.Bool(true)}))
	if first.ForwardingScore != 1 {
		t.Fatalf("flagged forward scored %d, want 1", first.ForwardingScore)
	}
	long := messageFromEvent(newEvent(&waE2E.ContextInfo{
		IsForwarded: proto.Bool(true), ForwardingScore: proto.Uint32(7)}))
	if long.ForwardingScore != 7 {
		t.Fatalf("forwarding score = %d, want 7", long.ForwardingScore)
	}
	plain := messageFromEvent(newEvent(nil))
	if plain.ForwardingScore != 0 {
		t.Fatalf("an ordinary message scored %d, want 0", plain.ForwardingScore)
	}
}

func TestStarPatchKeyLeavesTheSenderSlotEmptyExceptForGroupMessages(t *testing.T) {
	direct := types.NewJID("123", types.DefaultUserServer)
	group := types.NewJID("456", types.GroupServer)
	participant := types.NewJID("789", types.DefaultUserServer)
	// WhatsApp reads the literal "0" as "this key needs no sender". An empty
	// JID serialises to "" instead, and the server accepts that key silently
	// while the phone never matches it, so the star simply never appears.
	for _, tc := range []struct {
		name      string
		chat      types.JID
		senderJID string
		fromMe    bool
		want      string
	}{
		{"own message in a direct chat", direct, "", true, "0"},
		{"their message in a direct chat", direct, direct.String(), false, "0"},
		{"own message in a group", group, "", true, "0"},
		{"someone else's group message", group, participant.String(), false, participant.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender, err := starSenderJID(tc.chat, tc.senderJID, tc.fromMe)
			if err != nil {
				t.Fatal(err)
			}
			patch := appstate.BuildStar(tc.chat, sender, "m1", tc.fromMe, true)
			if len(patch.Mutations) != 1 {
				t.Fatalf("patch carries %d mutations, want 1", len(patch.Mutations))
			}
			index := patch.Mutations[0].Index
			if len(index) != 5 {
				t.Fatalf("index = %v, want 5 elements", index)
			}
			if index[4] != tc.want {
				t.Fatalf("sender slot = %q, want %q (index %v)", index[4], tc.want, index)
			}
		})
	}
}

func TestReplyCarriesAnExcerptOfTheQuotedMessage(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	chat := types.NewJID("123", types.DefaultUserServer)
	if err := st.UpsertMessage(ctx, model.Message{
		ID: "quoted", ChatJID: chat.String(), SenderJID: chat.String(), SenderName: "Alice",
		Timestamp: 1, Kind: "text", Body: "the original message", Status: "received",
	}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event))}

	reply := model.Message{ID: "answer", ChatJID: chat.String(), ReplyTo: "quoted"}
	filled := c.withReplyPreview(ctx, reply)
	if filled.ReplyPreview != "the original message" || filled.ReplySender != "Alice" || filled.ReplyFromMe {
		t.Fatalf("a stored quote should describe itself: %#v", filled)
	}

	// Nothing to look up: the reply keeps whatever the message carried, which
	// is the copy WhatsApp attaches to it.
	unknown := model.Message{ID: "answer", ChatJID: chat.String(), ReplyTo: "never-synced",
		ReplyPreview: "from the context", ReplySender: "456"}
	kept := c.withReplyPreview(ctx, unknown)
	if kept.ReplyPreview != "from the context" || kept.ReplySender != "456" {
		t.Fatalf("an unknown quote should keep the attached copy: %#v", kept)
	}
}

func TestMessageFromEventDescribesTheQuotedMessageItCarries(t *testing.T) {
	evt := &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{
			Chat:   types.NewJID("123", types.DefaultUserServer),
			Sender: types.NewJID("456", types.DefaultUserServer)},
			ID: "m1", Timestamp: time.Unix(100, 0)},
		Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("answering"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:      proto.String("prior"),
				Participant:   proto.String("789@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{Conversation: proto.String("  what was said before  ")},
			}}},
	}
	m := messageFromEvent(evt)
	if m.ReplyTo != "prior" || m.ReplyPreview != "what was said before" || m.ReplySender != "789" {
		t.Fatalf("unexpected reply context: %#v", m)
	}

	// A quoted picture has no words of its own.
	evt.Message.ExtendedTextMessage.ContextInfo.QuotedMessage = &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{Mimetype: proto.String("image/jpeg")}}
	if m := messageFromEvent(evt); m.ReplyPreview != "Image" {
		t.Fatalf("a quoted image should say so: %q", m.ReplyPreview)
	}
}

func TestHistorySyncKeepsTheReactionsAMessageAlreadyHad(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const chat = "972500000000-1600000000@g.us"
	c := &Client{store: st}

	// A reaction is its own message, so one left before this device was linked
	// arrives only here, attached to the message it was left on.
	c.handleHistorySync(&waEvents.HistorySync{Data: &waHistorySync.HistorySync{
		SyncType: waHistorySync.HistorySync_INITIAL_BOOTSTRAP.Enum(),
		Conversations: []*waHistorySync.Conversation{{
			ID: proto.String(chat),
			Messages: []*waHistorySync.HistorySyncMsg{{Message: &waWeb.WebMessageInfo{
				Key: &waCommon.MessageKey{
					ID:          proto.String("MSG1"),
					RemoteJID:   proto.String(chat),
					FromMe:      proto.Bool(false),
					Participant: proto.String("alice@s.whatsapp.net"),
				},
				MessageTimestamp: proto.Uint64(1_700_000_000),
				PushName:         proto.String("Alice"),
				Message:          &waE2E.Message{Conversation: proto.String("good morning")},
				Reactions: []*waWeb.Reaction{
					{
						Key: &waCommon.MessageKey{
							ID:          proto.String("REACT1"),
							RemoteJID:   proto.String(chat),
							Participant: proto.String("bob@s.whatsapp.net"),
						},
						Text:              proto.String("👍"),
						SenderTimestampMS: proto.Int64(1_700_000_005_000),
					},
					{
						Key: &waCommon.MessageKey{
							ID:          proto.String("REACT2"),
							RemoteJID:   proto.String(chat),
							Participant: proto.String("carol@s.whatsapp.net"),
						},
						Text:              proto.String("❤️"),
						SenderTimestampMS: proto.Int64(1_700_000_006_000),
					},
				},
			}}},
		}},
	}})

	page, err := st.ListMessages(ctx, chat, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 {
		t.Fatalf("the history message was not stored: %#v", page.Messages)
	}
	reactions := page.Messages[0].Reactions
	if len(reactions) != 2 {
		t.Fatalf("a message everybody reacted to came back with %d reactions", len(reactions))
	}
	byEmoji := map[string]string{}
	for _, reaction := range reactions {
		byEmoji[reaction.Emoji] = reaction.SenderJID
	}
	if byEmoji["👍"] != "bob@s.whatsapp.net" || byEmoji["❤️"] != "carol@s.whatsapp.net" {
		t.Fatalf("the reactions came back from the wrong people: %#v", reactions)
	}
	if reactions[0].Timestamp == 0 {
		t.Fatalf("a reaction came back without the time it was left: %#v", reactions[0])
	}
}

func TestReactionBackfillRefreshesTheBusiestChatsOnce(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for i, jid := range []string{"quiet@s.whatsapp.net", "busy@s.whatsapp.net", "busiest@s.whatsapp.net"} {
		if err := st.UpsertChat(ctx, model.Chat{JID: jid, LastMessageAt: int64(i+1) * 1000}); err != nil {
			t.Fatal(err)
		}
		if err := st.UpsertMessage(ctx, model.Message{ID: "m" + jid, ChatJID: jid, SenderJID: jid,
			Kind: "text", Body: "hello", Timestamp: int64(i+1) * 1000}, "", false); err != nil {
			t.Fatal(err)
		}
	}
	c := &Client{store: st}
	var refreshed []string
	refresh := func(_ context.Context, chatJID string, _ int) error {
		refreshed = append(refreshed, chatJID)
		return nil
	}

	// Reactions left before this version were dropped, and nothing arriving
	// later mentions them, so the recent page is asked for again.
	c.backfillReactionsWith(ctx, refresh, 2, 0)
	if len(refreshed) != 2 {
		t.Fatalf("asked for %d pages instead of the two busiest chats: %v", len(refreshed), refreshed)
	}
	if refreshed[0] != "busiest@s.whatsapp.net" || refreshed[1] != "busy@s.whatsapp.net" {
		t.Fatalf("the wrong conversations were refreshed: %v", refreshed)
	}

	// Reconnecting is routine. Asking WhatsApp for the same pages on every
	// connection would be a repeated request for history nobody is waiting for.
	refreshed = nil
	c.backfillReactionsWith(ctx, refresh, 2, 0)
	if len(refreshed) != 0 {
		t.Fatalf("the backfill ran a second time: %v", refreshed)
	}
}

func TestReactionBackfillDoesNotRunBesideItself(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const jid = "busy@s.whatsapp.net"
	if err := st.UpsertChat(ctx, model.Chat{JID: jid, LastMessageAt: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: jid, SenderJID: jid,
		Kind: "text", Body: "hello", Timestamp: 1000}, "", false); err != nil {
		t.Fatal(err)
	}
	c := &Client{store: st}

	// The backfill outlives the connection sweep that started it. A reconnect
	// while it is still running must not ask for the same pages again: the
	// metadata that stops it is only written once it is finished.
	var mu sync.Mutex
	var requests int
	started := make(chan struct{})
	release := make(chan struct{})
	refresh := func(context.Context, string, int) error {
		mu.Lock()
		requests++
		first := requests == 1
		mu.Unlock()
		if first {
			close(started)
			<-release
		}
		return nil
	}
	go c.backfillReactionsWith(ctx, refresh, 5, 0)
	<-started
	c.backfillReactionsWith(ctx, refresh, 5, 0)
	close(release)

	mu.Lock()
	defer mu.Unlock()
	if requests != 1 {
		t.Fatalf("a second backfill asked for %d pages while the first was running", requests)
	}
}

func TestAppStateHashMismatchAsksThePhoneAndStaysQuiet(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	requests := 0
	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event))}
	c.fetchAppState = func(context.Context, appstate.WAPatchName, bool, bool) error {
		return fmt.Errorf("failed to decode app state regular patches: failed to verify patch v156: %w",
			appstate.ErrMismatchingLTHash)
	}
	c.sendPeerMessage = func(_ context.Context, message *waE2E.Message) (whatsmeow.SendResponse, error) {
		requests++
		if message.GetProtocolMessage().GetPeerDataOperationRequestMessage().
			GetSyncdCollectionFatalRecoveryRequest().GetCollectionName() != string(appstate.WAPatchRegular) {
			t.Fatalf("the phone was asked for the wrong collection: %v", message)
		}
		return whatsmeow.SendResponse{}, nil
	}
	var shouted []gateway.Event
	c.Subscribe(func(event gateway.Event) {
		if event.Name == "daemon.error" {
			shouted = append(shouted, event)
		}
	})

	c.backfillCallLogs()
	if requests != 1 {
		t.Fatalf("a collection the server could not sign should be asked of the phone once, got %d", requests)
	}
	if len(shouted) != 0 {
		t.Fatalf("a background sync nobody asked for put an error across the window: %#v", shouted)
	}

	// A phone that has not answered yet must not be asked again on every
	// reconnection.
	c.backfillCallLogs()
	if requests != 1 {
		t.Fatalf("the phone was asked again within the day: %d requests", requests)
	}

	value, _, err := st.Metadata(context.Background(), appStateRecoveryMetadataKey(appstate.WAPatchRegular))
	if err != nil || value == "" {
		t.Fatalf("the request was not remembered: %q err=%v", value, err)
	}
}

func TestAppStateFailureThatIsNotAHashMismatchIsStillReported(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event))}
	c.fetchAppState = func(context.Context, appstate.WAPatchName, bool, bool) error {
		return errors.New("websocket disconnected")
	}
	c.sendPeerMessage = func(context.Context, *waE2E.Message) (whatsmeow.SendResponse, error) {
		t.Fatal("a plain failure must not ask the phone for a recovery copy")
		return whatsmeow.SendResponse{}, nil
	}
	var shouted []gateway.Event
	c.Subscribe(func(event gateway.Event) {
		if event.Name == "daemon.error" {
			shouted = append(shouted, event)
		}
	})
	c.backfillChatSettings()
	if len(shouted) != 1 {
		t.Fatalf("a failure that is not a hash mismatch was swallowed: %#v", shouted)
	}
}

func TestHashMismatchIsRecognisedThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("failed to decode app state %s patches: %w",
		appstate.WAPatchRegular, fmt.Errorf("failed to verify patch v156: %w", appstate.ErrMismatchingLTHash))
	if !isAppStateHashMismatch(wrapped) {
		t.Fatal("the mismatch the server reports has to be recognised through whatsmeow's wrapping")
	}
	if isAppStateHashMismatch(appstate.ErrKeyNotFound) {
		t.Fatal("a missing key is not a hash mismatch")
	}
}
