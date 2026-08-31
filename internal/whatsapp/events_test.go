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
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

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
