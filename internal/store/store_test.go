package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndPaginateMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := int64(1); i <= 3; i++ {
		err := s.UpsertMessage(ctx, model.Message{ID: string(rune('a' + i - 1)), ChatJID: "123@s.whatsapp.net", Timestamp: i * 1000, Kind: "text", Body: "hello", Status: "received"}, "Alice", true)
		if err != nil {
			t.Fatal(err)
		}
	}
	chats, err := s.ListChats(ctx, 100, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].UnreadCount != 3 || chats[0].LastMessageAt != 3000 {
		t.Fatalf("unexpected chat: %#v", chats)
	}
	page, err := s.ListMessages(ctx, "123@s.whatsapp.net", 4000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || len(page.Messages) != 2 || page.Messages[0].ID != "b" || page.Messages[1].ID != "c" {
		t.Fatalf("unexpected page: %#v", page)
	}
}

func TestDuplicateMessageDoesNotDuplicateRow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m := model.Message{ID: "m1", ChatJID: "g@g.us", Timestamp: 100, Kind: "text", Body: "first", Status: "received"}
	if err := s.UpsertMessage(ctx, m, "Group", true); err != nil {
		t.Fatal(err)
	}
	m.Body = "edited"
	m.Edited = true
	if err := s.UpsertMessage(ctx, m, "Group", false); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListMessages(ctx, "g@g.us", 200, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Body != "edited" || !page.Messages[0].Edited {
		t.Fatalf("unexpected messages: %#v", page.Messages)
	}
	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if chats[0].UnreadCount != 1 {
		t.Fatalf("duplicate incremented unread count: %d", chats[0].UnreadCount)
	}
}

func TestListMessagesHidesLegacyUnknownEnvelopes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, message := range []model.Message{
		{ID: "transport", ChatJID: "c@s.whatsapp.net", Timestamp: 1, Kind: "unknown", Status: "received"},
		{ID: "visible", ChatJID: "c@s.whatsapp.net", Timestamp: 2, Kind: "text", Body: "hello", Status: "received"},
	} {
		if err := s.UpsertMessage(ctx, message, "Contact", false); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListMessages(ctx, "c@s.whatsapp.net", 3, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].ID != "visible" {
		t.Fatalf("unexpected visible messages: %#v", page.Messages)
	}
}

func TestListMessagesHidesEmptySystemMessagesAlreadyStored(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, message := range []model.Message{
		// Written by a version that stored every protocol envelope. They have
		// no text, so they drew a bubble containing only a clock.
		{ID: "machinery", ChatJID: "c@s.whatsapp.net", Timestamp: 1, Kind: "system", Status: "received"},
		{ID: "gone", ChatJID: "c@s.whatsapp.net", Timestamp: 2, Kind: "system", Revoked: true, Status: "received"},
		{ID: "visible", ChatJID: "c@s.whatsapp.net", Timestamp: 3, Kind: "text", Body: "hello", Status: "received"},
	} {
		if err := s.UpsertMessage(ctx, message, "Contact", false); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListMessages(ctx, "c@s.whatsapp.net", 4, 50)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, message := range page.Messages {
		ids = append(ids, message.ID)
	}
	if len(ids) != 2 || ids[0] != "gone" || ids[1] != "visible" {
		t.Fatalf("an empty protocol envelope is still on screen: %v", ids)
	}
}

func TestListChatsHidesUnknownOnlyChatsAndUsesLatestVisibleMessage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, message := range []model.Message{
		{ID: "artifact", ChatJID: "artifact@s.whatsapp.net", Timestamp: 30, Kind: "unknown", Status: "received"},
		{ID: "visible", ChatJID: "mixed@s.whatsapp.net", Timestamp: 10, Kind: "text", Body: "real history", Status: "received"},
		{ID: "newer-artifact", ChatJID: "mixed@s.whatsapp.net", Timestamp: 20, Kind: "unknown", Status: "received"},
	} {
		if err := s.UpsertMessage(ctx, message, "Contact", false); err != nil {
			t.Fatal(err)
		}
	}

	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected one displayable chat, got %#v", chats)
	}
	if chats[0].JID != "mixed@s.whatsapp.net" || chats[0].LastMessageID != "visible" || chats[0].LastMessageAt != 10 || chats[0].LastMessagePreview != "real history" {
		t.Fatalf("chat did not use its latest displayable message: %#v", chats[0])
	}
}

func TestListChatsExcludesNonChatFeedsSystemOnlyAndArchivedConversations(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	fixtures := []struct {
		message model.Message
		title   string
	}{
		{model.Message{ID: "self-system", ChatJID: "573112522689@s.whatsapp.net", Timestamp: 50, Kind: "system", FromMe: true, Status: "sent"}, "+57 311 2522689"},
		{model.Message{ID: "status-image", ChatJID: "status@broadcast", Timestamp: 40, Kind: "image", Body: "status update", Status: "received"}, "status"},
		{model.Message{ID: "channel-post", ChatJID: "120363000000000000@newsletter", Timestamp: 30, Kind: "text", Body: "channel post", Status: "received"}, "Channel"},
		{model.Message{ID: "visible", ChatJID: "alice@s.whatsapp.net", Timestamp: 20, Kind: "text", Body: "hello", Status: "received"}, "Alice"},
	}
	for _, fixture := range fixtures {
		if err := s.UpsertMessage(ctx, fixture.message, fixture.title, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertChat(ctx, model.Chat{JID: "archived@s.whatsapp.net", Title: "Archived", LastMessageAt: 10, Archived: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "archived-message", ChatJID: "archived@s.whatsapp.net", Timestamp: 10, Kind: "text", Body: "old", Status: "received"}, "Archived", false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertChat(ctx, model.Chat{JID: "archived@s.whatsapp.net", Title: "Archived", LastMessageAt: 10, Archived: true}); err != nil {
		t.Fatal(err)
	}

	chats, err := s.ListChats(ctx, 20, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].JID != "alice@s.whatsapp.net" {
		t.Fatalf("expected only the real unarchived chat, got %#v", chats)
	}
}

func TestListMessagesIncludesReplyPreview(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, message := range []model.Message{
		{ID: "original", ChatJID: "c@s.whatsapp.net", SenderName: "Alice", Timestamp: 1, Kind: "text", Body: "quoted text", Status: "received"},
		{ID: "reply", ChatJID: "c@s.whatsapp.net", Timestamp: 2, Kind: "text", Body: "answer", ReplyTo: "original", FromMe: true, Status: "sent"},
	} {
		if err := s.UpsertMessage(ctx, message, "Alice", false); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListMessages(ctx, "c@s.whatsapp.net", 3, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[1].ReplyPreview != "quoted text" || page.Messages[1].ReplySender != "Alice" {
		t.Fatalf("reply preview missing: %#v", page.Messages)
	}
}

func TestReceiptStatusDoesNotRegress(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m := model.Message{ID: "m", ChatJID: "c@s.whatsapp.net", Timestamp: 1, Kind: "text", Body: "hi", FromMe: true, Status: "read"}
	if err := s.UpsertMessage(ctx, m, "C", false); err != nil {
		t.Fatal(err)
	}
	m.Status = "sent"
	if err := s.UpsertMessage(ctx, m, "", false); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListMessages(ctx, m.ChatJID, 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if page.Messages[0].Status != "read" {
		t.Fatalf("status regressed to %q", page.Messages[0].Status)
	}
}

func TestSearchEscapesWildcards(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, m := range []model.Message{{ID: "1", ChatJID: "c@s.whatsapp.net", Timestamp: 1, Kind: "text", Body: "100% ready"}, {ID: "2", ChatJID: "c@s.whatsapp.net", Timestamp: 2, Kind: "text", Body: "1000 ready"}} {
		if err := s.UpsertMessage(ctx, m, "C", false); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SearchMessages(ctx, "", "100%", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("unexpected results: %#v", got)
	}
}

func TestCallLogsAreUpsertedAndSortedNewestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.UpsertChat(ctx, model.Chat{JID: "bob@s.whatsapp.net", Title: "Bob", AvatarPath: "/tmp/bob.png"}); err != nil {
		t.Fatal(err)
	}
	for _, call := range []model.CallLog{
		{ID: "old", PeerJID: "alice@s.whatsapp.net", Timestamp: 10, Incoming: true, Result: "missed"},
		{ID: "new", PeerJID: "bob@s.whatsapp.net", Timestamp: 20, Video: true, Result: "connected"},
	} {
		if err := s.UpsertCallLog(ctx, call); err != nil {
			t.Fatal(err)
		}
	}
	logs, err := s.ListCallLogs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].ID != "new" || logs[1].ID != "old" {
		t.Fatalf("unexpected call log order: %#v", logs)
	}
	if logs[0].PeerName != "Bob" || logs[0].PeerAvatarPath != "/tmp/bob.png" {
		t.Fatalf("call log contact was not enriched: %#v", logs[0])
	}
}

func TestListChatsIncludesPinnedChatWithoutSyncedMessage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.UpsertChat(ctx, model.Chat{JID: "pinned@s.whatsapp.net", Title: "Pinned", LastMessageAt: 10, Pinned: true}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].JID != "pinned@s.whatsapp.net" || !chats[0].Pinned {
		t.Fatalf("pinned chat without synced messages was hidden: %#v", chats)
	}
}

func TestListChatsUsesWhatsAppPinOrderBeforeMessageTime(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "new-message@s.whatsapp.net", Title: "New message", LastMessageAt: 300, Pinned: true, PinnedAt: 100},
		{JID: "new-pin@s.whatsapp.net", Title: "New pin", LastMessageAt: 100, Pinned: true, PinnedAt: 300},
		{JID: "middle-pin@s.whatsapp.net", Title: "Middle pin", LastMessageAt: 200, Pinned: true, PinnedAt: 200},
	} {
		if err := s.ApplyChatSnapshot(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"new-pin@s.whatsapp.net", "middle-pin@s.whatsapp.net", "new-message@s.whatsapp.net"}
	if len(chats) != len(want) {
		t.Fatalf("got %d chats, want %d: %#v", len(chats), len(want), chats)
	}
	for i := range want {
		if chats[i].JID != want[i] {
			t.Fatalf("pin order[%d]=%q, want %q", i, chats[i].JID, want[i])
		}
	}
}

func TestUpdateChatFavoritesReplacesSynchronizedSet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "alice@s.whatsapp.net", Title: "Alice", LastMessageAt: 20},
		{JID: "family@g.us", Title: "Family", LastMessageAt: 10, IsGroup: true},
	} {
		if err := s.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpdateChatFavorites(ctx, []string{"alice@s.whatsapp.net"}); err != nil {
		t.Fatal(err)
	}
	alice, err := s.GetChat(ctx, "alice@s.whatsapp.net")
	if err != nil || !alice.Favorite {
		t.Fatalf("favorite was not persisted: chat=%#v err=%v", alice, err)
	}
	if err := s.UpdateChatFavorites(ctx, []string{"family@g.us"}); err != nil {
		t.Fatal(err)
	}
	alice, _ = s.GetChat(ctx, "alice@s.whatsapp.net")
	family, err := s.GetChat(ctx, "family@g.us")
	if err != nil || alice.Favorite || !family.Favorite {
		t.Fatalf("favorite set was not replaced atomically: alice=%#v family=%#v err=%v", alice, family, err)
	}
}

func TestLinkChatAliasesCombinesHistoryAndCanonicalizesFutureMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const lid = "201850896818405@lid"
	const phone = "573112522689@s.whatsapp.net"
	for i := int64(1); i <= 34; i++ {
		if err := s.UpsertMessage(ctx, model.Message{
			ID: fmt.Sprintf("phone-%02d", i), ChatJID: phone, Timestamp: i,
			Kind: "text", Body: "older", Status: "received",
		}, "Phone history", false); err != nil {
			t.Fatal(err)
		}
	}
	for i := int64(35); i <= 38; i++ {
		if err := s.UpsertMessage(ctx, model.Message{
			ID: fmt.Sprintf("lid-%02d", i), ChatJID: lid, Timestamp: i,
			Kind: "text", Body: "newer", Status: "received",
		}, "Current contact", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetLocalChatTitle(ctx, phone, "Local label"); err != nil {
		t.Fatal(err)
	}

	if err := s.LinkChatAliases(ctx, lid, phone); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListMessages(ctx, lid, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 38 || page.Messages[0].ID != "phone-01" || page.Messages[37].ID != "lid-38" {
		t.Fatalf("split history was not combined: count=%d first=%q last=%q", len(page.Messages), page.Messages[0].ID, page.Messages[len(page.Messages)-1].ID)
	}
	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].JID != lid || chats[0].Title != "Local label" {
		t.Fatalf("alias remained as a duplicate conversation: %#v", chats)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "future", ChatJID: phone, Timestamp: 39, Kind: "text", Body: "future", Status: "received"}, "", false); err != nil {
		t.Fatal(err)
	}
	page, err = s.ListMessages(ctx, lid, 0, 100)
	if err != nil || len(page.Messages) != 39 || page.Messages[38].ID != "future" {
		t.Fatalf("future alias message was not canonicalized: count=%d err=%v", len(page.Messages), err)
	}
}

func TestMediaPayloadRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	msg := model.Message{ID: "document-1", ChatJID: "alice@s.whatsapp.net", Timestamp: 10, Kind: "document", MediaName: "report.pdf", Status: "received"}
	if err := s.UpsertMessage(ctx, msg, "Alice", false); err != nil {
		t.Fatal(err)
	}
	want := []byte{0x01, 0x02, 0x03, 0xff}
	if err := s.SaveMediaPayload(ctx, msg.ChatJID, msg.ID, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.MediaPayload(ctx, msg.ChatJID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || string(got) != string(want) {
		t.Fatalf("unexpected media payload: ok=%v got=%x", ok, got)
	}
}

func TestUpsertChatPreservesConversationSettings(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "family@g.us"
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, Title: "Family", LastMessageAt: 10, UnreadCount: 2, MutedUntil: 9_999_999_999_999, Pinned: true, Archived: true, IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	// Directory sync, group-join events, and contact resolution only know a
	// conversation's identity. They must not reset settings that were
	// synchronised from WhatsApp.
	if err := s.UpsertChat(ctx, model.Chat{JID: jid, Title: "Family (renamed)", LastMessageAt: 5, IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Family (renamed)" || chat.LastMessageAt != 10 || chat.UnreadCount != 2 ||
		chat.MutedUntil != 9_999_999_999_999 || !chat.Pinned || !chat.Archived || !chat.IsGroup {
		t.Fatalf("identity upsert changed synchronised settings: %#v", chat)
	}
	if err := s.UpsertChat(ctx, model.Chat{JID: jid}); err != nil {
		t.Fatal(err)
	}
	chat, err = s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Family (renamed)" {
		t.Fatalf("empty title replaced the existing title: %q", chat.Title)
	}
}

func TestApplyChatSnapshotOverwritesConversationSettings(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, Title: "Alice", LastMessageAt: 10, MutedUntil: 5_000, Pinned: true, Archived: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatFavorites(ctx, []string{jid}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, LastMessageAt: 20, UnreadCount: 1}); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Alice" || chat.LastMessageAt != 20 || chat.UnreadCount != 1 || chat.MutedUntil != 0 || chat.Pinned || chat.Archived {
		t.Fatalf("snapshot did not replace conversation settings: %#v", chat)
	}
	if !chat.Favorite {
		t.Fatalf("snapshot cleared favorite, which WhatsApp synchronises separately: %#v", chat)
	}
}

func TestApplyCurrentChatSnapshotCanReduceUnreadCount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, Title: "Alice", LastMessageAt: 20, UnreadCount: 7}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, LastMessageAt: 20, UnreadCount: 0}); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 0 {
		t.Fatalf("current authoritative snapshot left %d unread messages", chat.UnreadCount)
	}

	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, LastMessageAt: 30, UnreadCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, LastMessageAt: 10, UnreadCount: 0}); err != nil {
		t.Fatal(err)
	}
	chat, err = s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 1 {
		t.Fatalf("stale snapshot replaced newer unread count: %#v", chat)
	}
}

func TestRecalculateUnreadUsesNewestIncomingReadBoundary(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	for _, msg := range []model.Message{
		{ID: "ancient-unmarked", ChatJID: jid, Timestamp: 100, Kind: "text", Body: "old", Status: "received"},
		{ID: "read-boundary", ChatJID: jid, Timestamp: 200, Kind: "text", Body: "read", Status: "read"},
		{ID: "outgoing", ChatJID: jid, Timestamp: 250, Kind: "text", Body: "mine", FromMe: true, Status: "read"},
		{ID: "new-unread", ChatJID: jid, Timestamp: 300, Kind: "text", Body: "new", Status: "received"},
	} {
		if err := s.UpsertMessage(ctx, msg, "Alice", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, LastMessageAt: 300, UnreadCount: 9}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecalculateUnreadCounts(ctx); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 1 {
		t.Fatalf("recalculated unread count = %d, want 1", chat.UnreadCount)
	}
}

func TestMarkChatReadClearsSnapshotWithoutLocalMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, UnreadCount: 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkChatRead(ctx, jid); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 0 {
		t.Fatalf("mark read left %d unread messages", chat.UnreadCount)
	}
}

func TestRepairHistoricalUnreadReplayRunsOnceForAffectedProfiles(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if _, err := s.db.ExecContext(ctx, `DELETE FROM metadata WHERE key=?`, unreadReplayRepairMetadataKey); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMetadata(ctx, "chat_settings_app_state_backfill_v5", "done"); err != nil {
		t.Fatal(err)
	}
	for _, msg := range []model.Message{
		{ID: "incoming", ChatJID: jid, Timestamp: 100, Kind: "text", Body: "hello", Status: "read"},
		{ID: "outgoing", ChatJID: jid, Timestamp: 200, Kind: "text", Body: "reply", FromMe: true, Status: "read"},
	} {
		if err := s.UpsertMessage(ctx, msg, "Alice", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApplyChatSnapshot(ctx, model.Chat{JID: jid, LastMessageAt: 200, UnreadCount: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE chats SET read_through_at=100 WHERE jid=?`, jid); err != nil {
		t.Fatal(err)
	}
	if err := s.repairHistoricalUnreadReplay(ctx); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 0 {
		t.Fatalf("repair left unread count %d", chat.UnreadCount)
	}
	var readThrough int64
	var incomingStatus, outgoingStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT read_through_at FROM chats WHERE jid=?`, jid).Scan(&readThrough); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM messages WHERE chat_jid=? AND id='incoming'`, jid).Scan(&incomingStatus); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM messages WHERE chat_jid=? AND id='outgoing'`, jid).Scan(&outgoingStatus); err != nil {
		t.Fatal(err)
	}
	if readThrough != 0 || incomingStatus != "received" || outgoingStatus != "read" {
		t.Fatalf("unexpected repaired state: boundary=%d incoming=%q outgoing=%q", readThrough, incomingStatus, outgoingStatus)
	}

	if err := s.MarkChatUnread(ctx, jid); err != nil {
		t.Fatal(err)
	}
	if err := s.repairHistoricalUnreadReplay(ctx); err != nil {
		t.Fatal(err)
	}
	chat, err = s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.UnreadCount != 1 {
		t.Fatalf("second repair cleared live unread count: %d", chat.UnreadCount)
	}
}

func TestIncomingPushNameDoesNotReplaceResolvedContactName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const jid = "alice@s.whatsapp.net"
	if err := s.UpsertChat(ctx, model.Chat{JID: jid, Title: "Adony Robles Lopez"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "message-1", ChatJID: jid, Timestamp: 10, Kind: "text", Body: "hello", Status: "received"}, "zone yalo", true); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, jid)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Adony Robles Lopez" {
		t.Fatalf("push name replaced resolved contact name: %q", chat.Title)
	}
}

func TestPinnedMessageAppearsInChatInfoAndCanBeCleared(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	const chatJID = "alice@s.whatsapp.net"
	const messageID = "pin-target"
	if err := s.UpsertMessage(ctx, model.Message{ID: messageID, ChatJID: chatJID, SenderJID: chatJID, Timestamp: 100, Kind: "text", Body: "Keep this", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	if err := s.SetMessagePinned(ctx, chatJID, messageID, expiresAt); err != nil {
		t.Fatal(err)
	}
	info, err := s.ChatInfo(ctx, chatJID)
	if err != nil {
		t.Fatal(err)
	}
	if info.PinnedMessage == nil || info.PinnedMessage.ID != messageID || info.PinnedMessage.Body != "Keep this" || info.PinnedUntil != expiresAt {
		t.Fatalf("unexpected pinned message: %#v", info)
	}
	if err := s.ClearMessagePin(ctx, chatJID); err != nil {
		t.Fatal(err)
	}
	info, err = s.ChatInfo(ctx, chatJID)
	if err != nil || info.PinnedMessage != nil {
		t.Fatalf("pin was not cleared: info=%#v err=%v", info, err)
	}
}

func TestReceiptMilestonesKeepTheirTimestamps(t *testing.T) {
	ctx := context.Background()
	s, err := Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	msg := model.Message{ID: "voice-1", ChatJID: "alice@lid", SenderJID: "me@lid", Timestamp: 1000, Kind: "audio", FromMe: true, Status: "sent"}
	if err := s.UpsertMessage(ctx, msg, "Alice", false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateReceipt(ctx, msg.ChatJID, []string{msg.ID}, "delivered", 2000); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateReceipt(ctx, msg.ChatJID, []string{msg.ID}, "played", 4000); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMessage(ctx, msg.ChatJID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DeliveredAt != 2000 || got.ReadAt != 4000 || got.PlayedAt != 4000 {
		t.Fatalf("receipt milestones were not retained: %#v", got)
	}
}

func TestListMessagesAttachesReactionsToCorrectMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	for i, id := range []string{"first", "second", "third"} {
		if err := s.UpsertMessage(ctx, model.Message{ID: id, ChatJID: chat, Timestamp: int64(i + 1), Kind: "text", Body: id, Status: "received"}, "Contact", false); err != nil {
			t.Fatal(err)
		}
	}
	for _, reaction := range []model.Reaction{
		{ChatJID: chat, MessageID: "first", SenderJID: "bob@s.whatsapp.net", Emoji: "👍", Timestamp: 20},
		{ChatJID: chat, MessageID: "first", SenderJID: "carol@s.whatsapp.net", Emoji: "❤️", Timestamp: 10},
		{ChatJID: chat, MessageID: "third", SenderJID: "bob@s.whatsapp.net", Emoji: "😂", Timestamp: 30},
	} {
		if err := s.UpsertReaction(ctx, reaction); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListMessages(ctx, chat, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 3 {
		t.Fatalf("expected three messages, got %#v", page.Messages)
	}
	first, second, third := page.Messages[0], page.Messages[1], page.Messages[2]
	if len(first.Reactions) != 2 || first.Reactions[0].SenderJID != "carol@s.whatsapp.net" || first.Reactions[1].SenderJID != "bob@s.whatsapp.net" {
		t.Fatalf("first message reactions wrong or unordered: %#v", first.Reactions)
	}
	if len(second.Reactions) != 0 {
		t.Fatalf("second message received foreign reactions: %#v", second.Reactions)
	}
	if len(third.Reactions) != 1 || third.Reactions[0].Emoji != "😂" || third.Reactions[0].MessageID != "third" {
		t.Fatalf("third message reactions wrong: %#v", third.Reactions)
	}
}

func TestUpsertMessageKeepsCachedMediaPaths(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "alice@s.whatsapp.net"
	cached := model.Message{ID: "photo-1", ChatJID: chat, Timestamp: 5, Kind: "image", Status: "received",
		MediaPath: "/cache/photo-1.jpg", MediaThumbnail: "/cache/thumbnails/photo-1.jpg"}
	if err := s.UpsertMessage(ctx, cached, "Alice", false); err != nil {
		t.Fatal(err)
	}
	// The same message arriving again, for example inside a history page,
	// carries no local paths. They must survive the write.
	repeated := model.Message{ID: "photo-1", ChatJID: chat, Timestamp: 5, Kind: "image", Status: "received"}
	if err := s.UpsertMessage(ctx, repeated, "Alice", false); err != nil {
		t.Fatal(err)
	}
	message, err := s.GetMessage(ctx, chat, "photo-1")
	if err != nil {
		t.Fatal(err)
	}
	if message.MediaPath != cached.MediaPath || message.MediaThumbnail != cached.MediaThumbnail {
		t.Fatalf("cached media was lost: path=%q thumbnail=%q", message.MediaPath, message.MediaThumbnail)
	}
	replaced := cached
	replaced.MediaPath = "/cache/photo-1-new.jpg"
	if err := s.UpsertMessage(ctx, replaced, "Alice", false); err != nil {
		t.Fatal(err)
	}
	message, err = s.GetMessage(ctx, chat, "photo-1")
	if err != nil {
		t.Fatal(err)
	}
	if message.MediaPath != "/cache/photo-1-new.jpg" {
		t.Fatalf("a newly downloaded file did not replace the old path: %q", message.MediaPath)
	}
}

func TestMessagesMissingThumbnailsPagesWithCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "alice@s.whatsapp.net"
	for _, id := range []string{"a", "b", "c"} {
		if err := s.UpsertMessage(ctx, model.Message{ID: id, ChatJID: chat, Timestamp: 1, Kind: "image", Status: "received"}, "Alice", false); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveMediaPayload(ctx, chat, id, []byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "text", ChatJID: chat, Timestamp: 2, Kind: "text", Body: "hi", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	first, err := s.MessagesMissingThumbnails(ctx, MediaCursor{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].MessageID != "a" || first[1].MessageID != "b" || string(first[0].Payload) != "\x01\x02\x03" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	next, err := s.MessagesMissingThumbnails(ctx, MediaCursor{ChatJID: chat, MessageID: first[1].MessageID}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].MessageID != "c" {
		t.Fatalf("cursor did not continue the scan: %#v", next)
	}
	if err := s.UpdateMediaThumbnail(ctx, chat, "c", "/cache/thumbnails/c.jpg"); err != nil {
		t.Fatal(err)
	}
	remaining, err := s.MessagesMissingThumbnails(ctx, MediaCursor{ChatJID: chat, MessageID: "b"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("message with a preview is still pending: %#v", remaining)
	}
}

func TestLinkPreviewSurvivesRoundTripAndRepeatedDelivery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "alice@s.whatsapp.net"
	withPreview := model.Message{ID: "link-1", ChatJID: chat, Timestamp: 5, Kind: "text", Status: "received",
		Body: "see https://example.com", LinkURL: "https://example.com", LinkTitle: "Example",
		LinkDescription: "A page", LinkThumbnail: "/cache/thumbnails/link-1.jpg"}
	if err := s.UpsertMessage(ctx, withPreview, "Alice", false); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListMessages(ctx, chat, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 {
		t.Fatalf("unexpected page: %#v", page.Messages)
	}
	got := page.Messages[0]
	if got.LinkURL != withPreview.LinkURL || got.LinkTitle != "Example" || got.LinkDescription != "A page" || got.LinkThumbnail == "" {
		t.Fatalf("link preview did not round-trip: %#v", got)
	}
	// The same message arriving again without preview fields must not clear it.
	repeated := model.Message{ID: "link-1", ChatJID: chat, Timestamp: 5, Kind: "text", Status: "received", Body: withPreview.Body}
	if err := s.UpsertMessage(ctx, repeated, "Alice", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, chat, "link-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.LinkURL == "" || stored.LinkTitle != "Example" || stored.LinkThumbnail == "" {
		t.Fatalf("repeated delivery cleared the link preview: %#v", stored)
	}
}

func TestChatInfoKeepsNewestLastSeenAcrossAliases(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	if err := s.UpsertChat(ctx, model.Chat{JID: "alice@lid", Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertChat(ctx, model.Chat{JID: "15551234567@s.whatsapp.net", Title: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatLastSeen(ctx, "15551234567@s.whatsapp.net", 1700000000000); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkChatAliases(ctx, "alice@lid", "15551234567@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatLastSeen(ctx, "alice@lid", 1600000000000); err != nil {
		t.Fatal(err)
	}
	info, err := s.ChatInfo(ctx, "alice@lid")
	if err != nil {
		t.Fatal(err)
	}
	if info.LastSeen != 1700000000000 {
		t.Fatalf("last seen = %d, want newest disclosed timestamp", info.LastSeen)
	}
}

func TestChatInfoAndSharedContentUseCanonicalHistory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const lid = "179452491378772@lid"
	const phoneJID = "573133878085@s.whatsapp.net"
	if err := s.UpsertChat(ctx, model.Chat{JID: lid, Title: "Shrums", AvatarPath: "/cache/avatar.jpg", MutedUntil: 9999999999999}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkChatAliases(ctx, lid, phoneJID); err != nil {
		t.Fatal(err)
	}
	messages := []model.Message{
		{ID: "photo", ChatJID: lid, Timestamp: 10, Kind: "image", MediaPath: "/cache/photo.jpg", Status: "received"},
		{ID: "video", ChatJID: lid, Timestamp: 9, Kind: "video", MediaThumbnail: "/cache/video.jpg", Status: "received"},
		{ID: "doc", ChatJID: lid, Timestamp: 8, Kind: "document", Body: "copy at https://example.net/invoice", MediaName: "invoice.pdf", Status: "received"},
		// Newer links must not displace the media thumbnails in the contact-info
		// preview strip. Links and documents have their own tabs in the full view.
		{ID: "link", ChatJID: lid, Timestamp: 12, Kind: "text", Body: "see https://example.com", LinkURL: "https://example.com", LinkTitle: "Example", Status: "received"},
		{ID: "plain-link", ChatJID: lid, Timestamp: 11, Kind: "text", Body: "https://example.org/path", Status: "received"},
		{ID: "revoked", ChatJID: lid, Timestamp: 5, Kind: "image", Revoked: true, Status: "received"},
	}
	for _, message := range messages {
		if err := s.UpsertMessage(ctx, message, "Shrums", false); err != nil {
			t.Fatal(err)
		}
	}
	info, err := s.ChatInfo(ctx, phoneJID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Chat.JID != lid || info.Phone != "573133878085" || info.MediaCount != 2 || info.DocumentCount != 1 || info.LinkCount != 3 || info.SharedCount != 5 {
		t.Fatalf("unexpected chat info: %#v", info)
	}
	if len(info.Preview) != 2 || info.Preview[0].ID != "photo" || info.Preview[1].ID != "video" {
		t.Fatalf("unexpected preview: %#v", info.Preview)
	}
	media, err := s.ListSharedMessages(ctx, phoneJID, "media", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(media.Messages) != 1 || media.Messages[0].ID != "photo" || !media.HasMore {
		t.Fatalf("unexpected media page: %#v", media)
	}
	documents, err := s.ListSharedMessages(ctx, lid, "documents", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents.Messages) != 1 || documents.Messages[0].MediaName != "invoice.pdf" {
		t.Fatalf("unexpected documents: %#v", documents)
	}
	links, err := s.ListSharedMessages(ctx, lid, "links", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Messages) != 3 || links.Messages[0].ID != "link" || links.Messages[1].ID != "plain-link" || links.Messages[2].ID != "doc" {
		t.Fatalf("unexpected links: %#v", links)
	}
}

func TestListChatsNamesConversationsFromTheirSender(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const business = "8772@lid"
	// History synchronisation left this conversation without a name, so the
	// stored title is just the identifier.
	if err := s.UpsertMessage(ctx, model.Message{ID: "b1", ChatJID: business, SenderName: "Bancolombia",
		Timestamp: 10, Kind: "text", Body: "hello", Status: "received"}, "", false); err != nil {
		t.Fatal(err)
	}
	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].Title != "Bancolombia" {
		t.Fatalf("conversation was not named after its sender: %#v", chats)
	}

	// A real stored name always wins over the sender's published name.
	if err := s.UpdateChatTitle(ctx, business, "Saved name"); err != nil {
		t.Fatal(err)
	}
	chats, err = s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if chats[0].Title != "Saved name" {
		t.Fatalf("stored name was overridden: %q", chats[0].Title)
	}

	// Nothing to fall back to leaves the conversation as it was.
	if err := s.UpsertMessage(ctx, model.Message{ID: "o1", ChatJID: "9999@lid", Timestamp: 5, Kind: "text",
		Body: "mine", FromMe: true, Status: "sent"}, "", false); err != nil {
		t.Fatal(err)
	}
	chats, err = s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, chat := range chats {
		if chat.JID == "9999@lid" && chat.Title != "9999" {
			t.Fatalf("unexpected fallback title: %q", chat.Title)
		}
	}
}

func TestWeakContactNameOnlyReplacesIdentifierPlaceholder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const placeholder = "179452491378772@lid"
	if err := s.UpsertChat(ctx, model.Chat{JID: placeholder}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatTitleIfPlaceholder(ctx, placeholder, "Bancolombia"); err != nil {
		t.Fatal(err)
	}
	chat, err := s.GetChat(ctx, placeholder)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Bancolombia" {
		t.Fatalf("numeric placeholder was not upgraded: %q", chat.Title)
	}

	const saved = "1234@lid"
	if err := s.UpsertChat(ctx, model.Chat{JID: saved, Title: "Saved contact"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatTitleIfPlaceholder(ctx, saved, "Weaker push name"); err != nil {
		t.Fatal(err)
	}
	chat, err = s.GetChat(ctx, saved)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Title != "Saved contact" {
		t.Fatalf("weak name replaced a saved name: %q", chat.Title)
	}
}

func TestListPlaceholderContactJIDsExcludesNamedAndGroupChats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "179452491378772@lid"},
		{JID: "573013536788@s.whatsapp.net"},
		{JID: "named@lid", Title: "Named person"},
		{JID: "120363000000000001@g.us", Title: "120363000000000001", IsGroup: true},
	} {
		if err := s.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	jids, err := s.ListPlaceholderContactJIDs(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(jids) != 2 || jids[0] != "179452491378772@lid" || jids[1] != "573013536788@s.whatsapp.net" {
		t.Fatalf("unexpected unresolved contacts: %#v", jids)
	}
}

func TestListMessagesHidesEnvelopesWithNoKind(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	// Older builds stored transport envelopes before they were recognised as
	// such. They have no kind and no body, and rendered as empty bubbles.
	for _, message := range []model.Message{
		{ID: "envelope", ChatJID: chat, Timestamp: 1, Kind: "", Status: "received"},
		{ID: "visible", ChatJID: chat, Timestamp: 2, Kind: "text", Body: "hello", Status: "received"},
	} {
		if err := s.UpsertMessage(ctx, message, "Contact", false); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListMessages(ctx, chat, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].ID != "visible" {
		t.Fatalf("empty envelope is still shown: %#v", page.Messages)
	}
	// History paging must not anchor on one either, or it would ask WhatsApp
	// for messages older than something that is not a message.
	oldest, err := s.OldestMessage(ctx, chat)
	if err != nil {
		t.Fatal(err)
	}
	if oldest.ID != "visible" {
		t.Fatalf("history anchored on an envelope: %#v", oldest)
	}
	chats, err := s.ListChats(ctx, 10, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].LastMessageID != "visible" {
		t.Fatalf("chat list previewed an envelope: %#v", chats)
	}
}

func TestSharedContactAndPlaceRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	contact := model.Message{ID: "c1", ChatJID: chat, Timestamp: 1, Kind: "contact", Status: "received",
		Body: "Adony Robles", ContactName: "Adony Robles", ContactPhone: "+57 311 2522689", ContactCount: 1}
	place := model.Message{ID: "l1", ChatJID: chat, Timestamp: 2, Kind: "location", Status: "received",
		Body: "Bogotá", Latitude: 4.60971, Longitude: -74.08175}
	for _, message := range []model.Message{contact, place} {
		if err := s.UpsertMessage(ctx, message, "C", false); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.ListMessages(ctx, chat, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("unexpected page: %#v", page.Messages)
	}
	if page.Messages[0].ContactName != "Adony Robles" || page.Messages[0].ContactPhone != "+57 311 2522689" ||
		page.Messages[0].ContactCount != 1 {
		t.Fatalf("contact details lost: %#v", page.Messages[0])
	}
	if page.Messages[1].Latitude == 0 || page.Messages[1].Longitude == 0 {
		t.Fatalf("coordinates lost: %#v", page.Messages[1])
	}
	// A repeat delivery without the details must not erase them.
	if err := s.UpsertMessage(ctx, model.Message{ID: "c1", ChatJID: chat, Timestamp: 1, Kind: "contact", Status: "received", Body: "Adony Robles"}, "C", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, chat, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.ContactPhone != "+57 311 2522689" {
		t.Fatalf("repeat delivery cleared the contact: %#v", stored)
	}
}

func TestMessagesMissingMediaScansNewestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	for _, message := range []model.Message{
		{ID: "old-photo", ChatJID: chat, Timestamp: 10, Kind: "image", Status: "received", MediaSize: 1000},
		{ID: "new-video", ChatJID: chat, Timestamp: 30, Kind: "video", Status: "received", MediaSize: 2000},
		{ID: "mid-doc", ChatJID: chat, Timestamp: 20, Kind: "document", Status: "received"},
		{ID: "already-here", ChatJID: chat, Timestamp: 40, Kind: "image", Status: "received", MediaPath: "/cache/x.jpg"},
		{ID: "just-text", ChatJID: chat, Timestamp: 50, Kind: "text", Body: "hi", Status: "received"},
	} {
		if err := s.UpsertMessage(ctx, message, "C", false); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := s.MessagesMissingMedia(ctx, MessageCursor{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected the three attachments with no file, got %#v", pending)
	}
	if pending[0].MessageID != "new-video" || pending[2].MessageID != "old-photo" {
		t.Fatalf("scan is not newest first: %#v", pending)
	}
	if pending[0].Kind != "video" || pending[0].Size != 2000 {
		t.Fatalf("attachment details missing: %#v", pending[0])
	}
	// Continuing from a cursor must not repeat what was already handled.
	next, err := s.MessagesMissingMedia(ctx, MessageCursor{Timestamp: pending[0].Timestamp, MessageID: pending[0].MessageID}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 || next[0].MessageID != "mid-doc" {
		t.Fatalf("cursor did not continue the scan: %#v", next)
	}
}

func TestUpdateChatMutedAndArchived(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	if err := s.UpsertChat(ctx, model.Chat{JID: chat, Title: "Contact"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatMuted(ctx, chat, 9_999_999_999_999); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChatArchived(ctx, chat, true); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetChat(ctx, chat)
	if err != nil {
		t.Fatal(err)
	}
	if stored.MutedUntil != 9_999_999_999_999 || !stored.Archived || stored.Title != "Contact" {
		t.Fatalf("mute and archive were not applied: %#v", stored)
	}
	if err := s.UpdateChatArchived(ctx, chat, false); err != nil {
		t.Fatal(err)
	}
	stored, _ = s.GetChat(ctx, chat)
	if stored.Archived {
		t.Fatal("unarchiving did not take effect")
	}
	if err := s.UpdateChatMuted(ctx, "", 1); err == nil {
		t.Fatal("an empty conversation id should be rejected")
	}
}

func TestUnreadMessageCountExcludesArchivedAndBroadcastChats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "visible@s.whatsapp.net", Title: "Visible", UnreadCount: 3},
		{JID: "archived@s.whatsapp.net", Title: "Archived", UnreadCount: 4, Archived: true},
		{JID: "status@broadcast", Title: "Status", UnreadCount: 5},
	} {
		if err := s.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.UnreadMessageCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("unread message count = %d, want 3", got)
	}
}

func TestDeletedMessageStaysDeletedWhenHistoryRedeliversIt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	original := model.Message{ID: "m1", ChatJID: chat, Timestamp: 10, Kind: "text",
		Body: "the original text", Status: "received"}
	if err := s.UpsertMessage(ctx, original, "C", false); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRevoked(ctx, chat, "m1"); err != nil {
		t.Fatal(err)
	}
	// History synchronisation redelivers the message exactly as it was sent,
	// with no knowledge that it was later deleted for everyone.
	if err := s.UpsertMessage(ctx, original, "C", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, chat, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Revoked {
		t.Fatal("a deleted message came back")
	}
	if stored.Body != "" || stored.Kind != "revoked" {
		t.Fatalf("deleted content was restored: kind=%q body=%q", stored.Kind, stored.Body)
	}
	page, err := s.ListMessages(ctx, chat, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Body != "" {
		t.Fatalf("deleted text is visible in the conversation: %#v", page.Messages)
	}
}

func TestPagingKeepsMessagesThatShareOneSecond(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	// WhatsApp timestamps have one-second resolution, so a burst of messages
	// carries a single value. Every one of them must still be reachable.
	for i := 0; i < 10; i++ {
		if err := s.UpsertMessage(ctx, model.Message{
			ID: fmt.Sprintf("burst-%02d", i), ChatJID: chat, Timestamp: 5000,
			Kind: "text", Body: fmt.Sprintf("message %d", i), Status: "received",
		}, "C", false); err != nil {
			t.Fatal(err)
		}
	}

	seen := map[string]bool{}
	cursor := int64(0)
	cursorID := ""
	for page := 0; page < 12; page++ {
		result, err := s.ListMessagesBefore(ctx, chat, cursor, cursorID, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Messages) == 0 {
			break
		}
		for _, message := range result.Messages {
			if seen[message.ID] {
				t.Fatalf("page %d returned %s twice", page, message.ID)
			}
			seen[message.ID] = true
		}
		if !result.HasMore {
			break
		}
		if result.NextBefore == cursor && result.NextBeforeID == cursorID {
			t.Fatalf("cursor did not advance past %d/%s", cursor, cursorID)
		}
		cursor, cursorID = result.NextBefore, result.NextBeforeID
	}
	if len(seen) != 10 {
		t.Fatalf("paging reached only %d of 10 messages sharing a timestamp", len(seen))
	}
}

func TestUpsertMessageKeepsMediaDetailsWhenRedelivered(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	full := model.Message{ID: "doc-1", ChatJID: chat, Timestamp: 5, Kind: "document", Status: "received",
		MediaMIME: "application/pdf", MediaName: "report.pdf", MediaSize: 4096}
	if err := s.UpsertMessage(ctx, full, "C", false); err != nil {
		t.Fatal(err)
	}
	// A history frame that omits the file details must not erase them.
	if err := s.UpsertMessage(ctx, model.Message{ID: "doc-1", ChatJID: chat, Timestamp: 5,
		Kind: "document", Status: "received"}, "C", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, chat, "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.MediaMIME != "application/pdf" || stored.MediaName != "report.pdf" || stored.MediaSize != 4096 {
		t.Fatalf("media details were lost: %#v", stored)
	}
}

func TestStarringKeepsTheMessageAndListsItAcrossChats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const first = "a@s.whatsapp.net"
	const second = "b@s.whatsapp.net"
	if err := s.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: first, Timestamp: 10,
		Kind: "text", Body: "keep me", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "m2", ChatJID: second, Timestamp: 20,
		Kind: "text", Body: "also keep", Status: "received"}, "Bob", false); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "m3", ChatJID: first, Timestamp: 30,
		Kind: "text", Body: "not starred", Status: "received"}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		chat, id string
	}{{first, "m1"}, {second, "m2"}} {
		if err := s.SetMessageStarred(ctx, target.chat, target.id, true); err != nil {
			t.Fatal(err)
		}
	}

	starred, err := s.ListStarredMessages(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred) != 2 {
		t.Fatalf("expected the two starred messages, got %d", len(starred))
	}
	// Newest first, and each row has to name the conversation it came from
	// because the list spans chats.
	if starred[0].ID != "m2" || starred[1].ID != "m1" {
		t.Fatalf("starred messages are not newest first: %s then %s", starred[0].ID, starred[1].ID)
	}
	for _, m := range starred {
		if !m.Starred {
			t.Fatalf("%s came back without its star", m.ID)
		}
		if m.ChatJID == "" {
			t.Fatalf("%s came back with no conversation", m.ID)
		}
	}

	// Unstarring removes it from the list without touching the message.
	if err := s.SetMessageStarred(ctx, first, "m1", false); err != nil {
		t.Fatal(err)
	}
	starred, err = s.ListStarredMessages(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred) != 1 || starred[0].ID != "m2" {
		t.Fatalf("unstarring did not update the list: %#v", starred)
	}
	kept, err := s.GetMessage(ctx, first, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if kept.Body != "keep me" || kept.Starred {
		t.Fatalf("unstarring damaged the message: %#v", kept)
	}
}

func TestStarSurvivesHistoryRedeliveringTheMessage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	original := model.Message{ID: "m1", ChatJID: chat, Timestamp: 10, Kind: "text",
		Body: "important", Status: "received"}
	if err := s.UpsertMessage(ctx, original, "C", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMessageStarred(ctx, chat, "m1", true); err != nil {
		t.Fatal(err)
	}
	// History synchronisation replays the same message constantly. A star is
	// account state, so a replay must not silently drop it.
	if err := s.UpsertMessage(ctx, original, "C", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, chat, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Starred {
		t.Fatal("a redelivered message lost its star")
	}
}

func TestForwardingScoreIsStoredAndSurvivesAScorelessRedelivery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "c@s.whatsapp.net"
	forwarded := model.Message{ID: "m1", ChatJID: chat, Timestamp: 10, Kind: "text",
		Body: "passed along", Status: "received", ForwardingScore: 3}
	if err := s.UpsertMessage(ctx, forwarded, "C", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, chat, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.ForwardingScore != 3 {
		t.Fatalf("forwarding score not stored: got %d, want 3", stored.ForwardingScore)
	}
	// A history page carries the body but not always the forward context, and
	// a zero there must not turn a forwarded message back into an original.
	plain := forwarded
	plain.ForwardingScore = 0
	if err := s.UpsertMessage(ctx, plain, "C", false); err != nil {
		t.Fatal(err)
	}
	if stored, err = s.GetMessage(ctx, chat, "m1"); err != nil {
		t.Fatal(err)
	}
	if stored.ForwardingScore != 3 {
		t.Fatalf("redelivery erased the forwarding score: got %d, want 3", stored.ForwardingScore)
	}
}

func TestDeleteChatRemovesTheChatAndEverythingHangingOffIt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "gone@s.whatsapp.net"
	const kept = "stays@s.whatsapp.net"
	for _, seed := range []struct {
		jid  string
		id   string
		body string
	}{{chat, "m1", "first"}, {chat, "m2", "second"}, {kept, "k1", "untouched"}} {
		msg := model.Message{ID: seed.id, ChatJID: seed.jid, Timestamp: 10, Kind: "text", Body: seed.body, Status: "received"}
		if err := s.UpsertMessage(ctx, msg, "Title", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveMediaPayload(ctx, chat, "m1", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMessagePinned(ctx, chat, "m1", time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteChat(ctx, chat); err != nil {
		t.Fatal(err)
	}

	if _, err := s.GetChat(ctx, chat); err == nil {
		t.Fatal("the chat row is still there after DeleteChat")
	}
	page, err := s.ListMessages(ctx, chat, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 0 {
		t.Fatalf("messages survived the delete: %d left", len(page.Messages))
	}
	if _, ok, err := s.MediaPayload(ctx, chat, "m1"); err != nil || ok {
		t.Fatalf("media payload survived the delete: ok=%v err=%v", ok, err)
	}
	// PinnedMessage reports "no pin" as an empty message, not as an error.
	if pinned, _, err := s.PinnedMessage(ctx, chat); err != nil || pinned.ID != "" {
		t.Fatalf("the pin survived the delete: id=%q err=%v", pinned.ID, err)
	}
	// A neighbouring conversation must be untouched: the delete is scoped by
	// chat, and a stray WHERE clause would take the whole table with it.
	other, err := s.ListMessages(ctx, kept, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Messages) != 1 {
		t.Fatalf("the other chat lost messages: %d left", len(other.Messages))
	}
}

func TestGroupSenderNameBackfillLabelsMessagesThatSyncedWithoutAPushName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const group = "team@g.us"
	const namedSender = "111@s.whatsapp.net"
	const knownChatSender = "222@s.whatsapp.net"
	const strangerSender = "333@s.whatsapp.net"

	seed := []model.Message{
		{ID: "a", ChatJID: group, SenderJID: namedSender, SenderName: "Dana", Timestamp: 10, Kind: "text", Body: "one", Status: "received"},
		{ID: "b", ChatJID: group, SenderJID: namedSender, Timestamp: 20, Kind: "text", Body: "two", Status: "received"},
		{ID: "c", ChatJID: group, SenderJID: knownChatSender, Timestamp: 30, Kind: "text", Body: "three", Status: "received"},
		{ID: "d", ChatJID: group, SenderJID: strangerSender, Timestamp: 40, Kind: "text", Body: "four", Status: "received"},
	}
	for _, msg := range seed {
		if err := s.UpsertMessage(ctx, msg, "Team", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertChat(ctx, model.Chat{JID: knownChatSender, Title: "Yossi"}); err != nil {
		t.Fatal(err)
	}
	// newTestStore already ran the migration, so re-arm it before asserting.
	if err := s.SetMetadata(ctx, groupSenderNameBackfillKey, ""); err != nil {
		t.Fatal(err)
	}

	if err := s.backfillGroupSenderNames(ctx); err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		id   string
		name string
	}{
		{"b", "Dana"},  // copied from the same sender's named message
		{"c", "Yossi"}, // copied from that sender's own conversation title
		{"d", ""},      // nothing known; the client renders the number
	} {
		got, err := s.GetMessage(ctx, group, want.id)
		if err != nil {
			t.Fatal(err)
		}
		if got.SenderName != want.name {
			t.Fatalf("message %s: sender name is %q, want %q", want.id, got.SenderName, want.name)
		}
	}
}

func TestChatListsAreStoredAndScopedToTheChatTheyName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const work = "1"
	const personal = "2"
	for _, label := range []model.Label{{ID: work, Name: "Work", Color: 3}, {ID: personal, Name: "Personal"}} {
		if err := s.UpsertLabel(ctx, label, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetChatLabeled(ctx, "a@s.whatsapp.net", work, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetChatLabeled(ctx, "b@s.whatsapp.net", personal, true); err != nil {
		t.Fatal(err)
	}

	labels, err := s.ListLabels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 || labels[0].Name != "Work" || labels[0].Color != 3 {
		t.Fatalf("unexpected label list: %+v", labels)
	}
	ids, err := s.ChatLabelIDs(ctx, "a@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != work {
		t.Fatalf("chat a is in %v, want only %q", ids, work)
	}

	// Deleting a list must take its memberships with it, or a chat keeps
	// pointing at a list that no longer exists.
	if err := s.UpsertLabel(ctx, model.Label{ID: work, Name: "Work"}, true); err != nil {
		t.Fatal(err)
	}
	if labels, err = s.ListLabels(ctx); err != nil {
		t.Fatal(err)
	} else if len(labels) != 1 || labels[0].ID != personal {
		t.Fatalf("deleted list survived: %+v", labels)
	}
	if ids, err = s.ChatLabelIDs(ctx, "a@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	} else if len(ids) != 0 {
		t.Fatalf("membership of a deleted list survived: %v", ids)
	}

	next, err := s.NextLabelID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The tombstone still occupies id 1, so the next free id is 3, not 2.
	if next != "3" {
		t.Fatalf("next label id is %q, want \"3\"", next)
	}
}

func TestSearchMessagesCarriesTheChatTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.UpsertChat(ctx, model.Chat{JID: "c@s.whatsapp.net", Title: "Karen Gomez"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "1", ChatJID: "c@s.whatsapp.net", Timestamp: 1, Kind: "text", Body: "Hola shuki"}, "", false); err != nil {
		t.Fatal(err)
	}
	got, err := s.SearchMessages(ctx, "", "hola", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one hit, got %d", len(got))
	}
	if got[0].ChatTitle != "Karen Gomez" {
		t.Fatalf("chat title missing from the result: %#v", got[0])
	}
}

func TestSearchContactsExcludesConversationsAlreadyListed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// One name the address book knows without a conversation, one with.
	for _, chat := range []model.Chat{
		{JID: "quiet@s.whatsapp.net", Title: "Hila Halamit"},
		{JID: "chatty@s.whatsapp.net", Title: "Hila Talker"},
	} {
		if err := s.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "1", ChatJID: "chatty@s.whatsapp.net", Timestamp: 1, Kind: "text", Body: "hi"}, "", false); err != nil {
		t.Fatal(err)
	}
	got, err := s.SearchContacts(ctx, "hila", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].JID != "quiet@s.whatsapp.net" {
		t.Fatalf("unexpected contacts: %#v", got)
	}
	if got[0].Name != "Hila Halamit" || got[0].Phone != "quiet" {
		t.Fatalf("contact fields not filled: %#v", got[0])
	}
}

func TestSearchMessagesLeavesStatusPostsOut(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, m := range []model.Message{
		{ID: "1", ChatJID: "c@s.whatsapp.net", Timestamp: 2, Kind: "text", Body: "Hola shuki"},
		{ID: "2", ChatJID: "status@broadcast", Timestamp: 1, Kind: "text", Body: "Hola everyone"},
	} {
		if err := s.UpsertMessage(ctx, m, "", false); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SearchMessages(ctx, "", "hola", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("a status post reached the sidebar results: %#v", got)
	}
}

func TestSearchContactsSkipsAliasesOfAConversation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// The same person under both address forms, with the conversation held on
	// the privacy-preserving identity.
	for _, chat := range []model.Chat{
		{JID: "111@lid", Title: "Hila Halamit"},
		{JID: "972500000000@s.whatsapp.net", Title: "Hila Halamit"},
	} {
		if err := s.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.LinkChatAliases(ctx, "111@lid", "972500000000@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMessage(ctx, model.Message{ID: "1", ChatJID: "111@lid", Timestamp: 1, Kind: "text", Body: "hi"}, "", false); err != nil {
		t.Fatal(err)
	}
	got, err := s.SearchContacts(ctx, "hila", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("the alias of an open conversation was offered as a contact: %#v", got)
	}
}

func TestSearchChatsReachesTheArchivedShelf(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "open@s.whatsapp.net", Title: "Atelie Open"},
		{JID: "filed@s.whatsapp.net", Title: "Atelie Filed", Archived: true},
	} {
		if err := s.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []model.Message{
		{ID: "1", ChatJID: "open@s.whatsapp.net", Timestamp: 1, Kind: "text", Body: "hi"},
		{ID: "2", ChatJID: "filed@s.whatsapp.net", Timestamp: 2, Kind: "text", Body: "hi"},
	} {
		if err := s.UpsertMessage(ctx, m, "", false); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SearchChats(ctx, "atelie", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("search did not span both shelves: %#v", got)
	}
	// The archived one has to say so, because the row is drawn beside chats
	// that are not archived.
	var archivedSeen bool
	for _, chat := range got {
		if chat.JID == "filed@s.whatsapp.net" {
			archivedSeen = chat.Archived
		}
	}
	if !archivedSeen {
		t.Fatalf("an archived result did not report itself archived: %#v", got)
	}
	// The plain list keeps the shelves apart.
	list, err := s.ListChats(ctx, 10, 0, "atelie")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].JID != "open@s.whatsapp.net" {
		t.Fatalf("the chat list leaked archived chats: %#v", list)
	}
}

func TestSearchMessagesScopesToOneChatAndFindsFilenames(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, m := range []model.Message{
		{ID: "1", ChatJID: "a@s.whatsapp.net", Timestamp: 3, Kind: "text", Body: "cuenta nequi?"},
		{ID: "2", ChatJID: "b@s.whatsapp.net", Timestamp: 2, Kind: "text", Body: "otra cuenta"},
		// A document carries no body; its filename is the only text it has.
		{ID: "3", ChatJID: "a@s.whatsapp.net", Timestamp: 1, Kind: "document", MediaName: "Extracto_Cuentas_3052.pdf"},
	} {
		if err := s.UpsertMessage(ctx, m, "", false); err != nil {
			t.Fatal(err)
		}
	}
	everywhere, err := s.SearchMessages(ctx, "", "cuenta", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(everywhere) != 3 {
		t.Fatalf("an unscoped search missed a match: %#v", everywhere)
	}
	scoped, err := s.SearchMessages(ctx, "a@s.whatsapp.net", "cuenta", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 2 {
		t.Fatalf("the scoped search did not stay in its chat: %#v", scoped)
	}
	for _, m := range scoped {
		if m.ChatJID != "a@s.whatsapp.net" {
			t.Fatalf("a result came from another chat: %#v", m)
		}
	}
	if scoped[1].MediaName != "Extracto_Cuentas_3052.pdf" {
		t.Fatalf("the document was not matched by its filename: %#v", scoped)
	}
}

func TestReplyPreviewDescribesTheQuotedMessage(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.UpsertMessage(ctx, model.Message{
		ID: "quoted", ChatJID: "chat@s.whatsapp.net", SenderJID: "alice@s.whatsapp.net",
		SenderName: "Alice", Timestamp: 1, Kind: "text", Body: "  the original  ",
	}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertMessage(ctx, model.Message{
		ID: "mine", ChatJID: "chat@s.whatsapp.net", Timestamp: 2, Kind: "image",
		FromMe: true,
	}, "Alice", false); err != nil {
		t.Fatal(err)
	}

	text, sender, fromMe, ok := st.ReplyPreview(ctx, "chat@s.whatsapp.net", "quoted")
	if !ok || text != "the original" || sender != "Alice" || fromMe {
		t.Fatalf("quoted text: %q sender %q fromMe %v ok %v", text, sender, fromMe, ok)
	}

	// A quoted picture has no words to show, so the kind is the preview.
	text, _, fromMe, ok = st.ReplyPreview(ctx, "chat@s.whatsapp.net", "mine")
	if !ok || text != "Image" || !fromMe {
		t.Fatalf("quoted image: %q fromMe %v ok %v", text, fromMe, ok)
	}

	if _, _, _, ok := st.ReplyPreview(ctx, "chat@s.whatsapp.net", "never-synced"); ok {
		t.Fatal("a message that is not in the database reported a preview")
	}
}

func TestReactionsCarryTheNameOfWhoLeftThem(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	const chat = "group@g.us"
	if err := s.UpsertChat(ctx, model.Chat{JID: chat, Title: "Family", IsGroup: true}); err != nil {
		t.Fatal(err)
	}
	// Bob is in the address book, so his own conversation names him. Alice is
	// only ever seen inside the group, where her messages carry her name.
	if err := s.UpsertChat(ctx, model.Chat{JID: "bob@s.whatsapp.net", Title: "Bob Levy", AvatarPath: "/tmp/bob.jpg"}); err != nil {
		t.Fatal(err)
	}
	for _, message := range []model.Message{
		{ID: "m1", ChatJID: chat, SenderJID: "alice@s.whatsapp.net", SenderName: "Alice Cohen", Kind: "text", Body: "morning", Timestamp: 10},
		{ID: "m2", ChatJID: chat, SenderJID: "dave@s.whatsapp.net", Kind: "text", Body: "hi", Timestamp: 20},
	} {
		if err := s.UpsertMessage(ctx, message, "", false); err != nil {
			t.Fatal(err)
		}
	}
	for _, reaction := range []model.Reaction{
		{ChatJID: chat, MessageID: "m1", SenderJID: "bob@s.whatsapp.net", Emoji: "👍", Timestamp: 30},
		{ChatJID: chat, MessageID: "m1", SenderJID: "alice@s.whatsapp.net", Emoji: "❤️", Timestamp: 40},
		{ChatJID: chat, MessageID: "m1", SenderJID: "dave@s.whatsapp.net", Emoji: "😂", Timestamp: 50},
	} {
		if err := s.UpsertReaction(ctx, reaction); err != nil {
			t.Fatal(err)
		}
	}

	page, err := s.ListMessages(ctx, chat, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	avatars := map[string]string{}
	for _, message := range page.Messages {
		for _, reaction := range message.Reactions {
			names[reaction.SenderJID] = reaction.SenderName
			avatars[reaction.SenderJID] = reaction.SenderAvatarPath
		}
	}
	if names["bob@s.whatsapp.net"] != "Bob Levy" {
		t.Fatalf("a saved contact reacted and came back as %q", names["bob@s.whatsapp.net"])
	}
	if names["alice@s.whatsapp.net"] != "Alice Cohen" {
		t.Fatalf("a group member reacted and came back as %q", names["alice@s.whatsapp.net"])
	}
	// Nobody knows Dave's name. The panel falls back to his number rather than
	// being handed something invented here.
	if names["dave@s.whatsapp.net"] != "" {
		t.Fatalf("an unknown number came back named %q", names["dave@s.whatsapp.net"])
	}
	if avatars["bob@s.whatsapp.net"] != "/tmp/bob.jpg" {
		t.Fatalf("the reaction did not carry the picture of who left it: %q", avatars["bob@s.whatsapp.net"])
	}
}

func TestReplayedHistoryDoesNotUndoAnEdit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	original := model.Message{ID: "m1", ChatJID: "c@s.whatsapp.net", Timestamp: 10, Kind: "text", Body: "original", Status: "received"}
	if err := s.UpsertMessage(ctx, original, "Contact", false); err != nil {
		t.Fatal(err)
	}
	if err := s.EditMessage(ctx, "c@s.whatsapp.net", "m1", "corrected"); err != nil {
		t.Fatal(err)
	}
	// History redelivers the message as it was first sent.
	if err := s.UpsertMessage(ctx, original, "Contact", false); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetMessage(ctx, "c@s.whatsapp.net", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Body != "corrected" || !stored.Edited {
		t.Fatalf("an older copy put the replaced text back: body=%q edited=%v", stored.Body, stored.Edited)
	}

	// A later edit still applies.
	if err := s.EditMessage(ctx, "c@s.whatsapp.net", "m1", "corrected twice"); err != nil {
		t.Fatal(err)
	}
	if stored, err = s.GetMessage(ctx, "c@s.whatsapp.net", "m1"); err != nil || stored.Body != "corrected twice" {
		t.Fatalf("a later edit did not apply: body=%q err=%v", stored.Body, err)
	}
}

func TestOlderReactionDoesNotReplaceTheCurrentOne(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: "c@s.whatsapp.net", Timestamp: 10, Kind: "text", Body: "hi", Status: "received"}, "Contact", false); err != nil {
		t.Fatal(err)
	}
	current := model.Reaction{ChatJID: "c@s.whatsapp.net", MessageID: "m1", SenderJID: "friend@s.whatsapp.net", Emoji: "❤️", Timestamp: 200}
	if err := s.UpsertReaction(ctx, current); err != nil {
		t.Fatal(err)
	}
	stale := model.Reaction{ChatJID: "c@s.whatsapp.net", MessageID: "m1", SenderJID: "friend@s.whatsapp.net", Emoji: "👍", Timestamp: 100}
	if err := s.UpsertReaction(ctx, stale); err != nil {
		t.Fatal(err)
	}
	detail, err := s.MessageDetail(ctx, "c@s.whatsapp.net", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Reactions) != 1 || detail.Reactions[0].Emoji != "❤️" {
		t.Fatalf("history replaced the current reaction: %#v", detail.Reactions)
	}

	// A stale removal must not clear a reaction left since.
	if err := s.UpsertReaction(ctx, model.Reaction{ChatJID: "c@s.whatsapp.net", MessageID: "m1", SenderJID: "friend@s.whatsapp.net", Timestamp: 100}); err != nil {
		t.Fatal(err)
	}
	if detail, err = s.MessageDetail(ctx, "c@s.whatsapp.net", "m1"); err != nil || len(detail.Reactions) != 1 {
		t.Fatalf("a stale removal took the current reaction away: %#v err=%v", detail.Reactions, err)
	}

	// Taking it back now does clear it.
	if err := s.UpsertReaction(ctx, model.Reaction{ChatJID: "c@s.whatsapp.net", MessageID: "m1", SenderJID: "friend@s.whatsapp.net", Timestamp: 300}); err != nil {
		t.Fatal(err)
	}
	if detail, err = s.MessageDetail(ctx, "c@s.whatsapp.net", "m1"); err != nil || len(detail.Reactions) != 0 {
		t.Fatalf("taking a reaction back left it on the message: %#v err=%v", detail.Reactions, err)
	}

	// A newer reaction still applies.
	if err := s.UpsertReaction(ctx, model.Reaction{ChatJID: "c@s.whatsapp.net", MessageID: "m1", SenderJID: "friend@s.whatsapp.net", Emoji: "😀", Timestamp: 400}); err != nil {
		t.Fatal(err)
	}
	if detail, err = s.MessageDetail(ctx, "c@s.whatsapp.net", "m1"); err != nil || len(detail.Reactions) != 1 || detail.Reactions[0].Emoji != "😀" {
		t.Fatalf("a newer reaction did not apply: %#v err=%v", detail.Reactions, err)
	}
}
