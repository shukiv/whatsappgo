package whatsapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/shuki/whatsappgo/internal/model"
	localstore "github.com/shuki/whatsappgo/internal/store"
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

func TestMessageFromEventDropsTransportOnlyEnvelope(t *testing.T) {
	evt := &waEvents.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("123", types.DefaultUserServer)}, ID: "transport", Timestamp: time.Now()}, Message: &waE2E.Message{SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{}}}
	if m := messageFromEvent(evt); m.ID != "" {
		t.Fatalf("transport-only envelope must not become a chat bubble: %#v", m)
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
	c.handleEvent(&waEvents.Pin{JID: jid, Action: &waSyncAction.PinAction{Pinned: proto.Bool(true)}})
	chat, err := st.GetChat(ctx, jid.String())
	if err != nil {
		t.Fatal(err)
	}
	if !chat.Pinned {
		t.Fatal("pin app-state event did not update the local chat")
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
	if err := st.UpsertChat(ctx, model.Chat{JID: jid, Title: "Alice"}); err != nil {
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
