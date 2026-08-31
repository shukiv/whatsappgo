package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/shuki/whatsappgo/internal/model"
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
	got, err := s.SearchMessages(ctx, "100%", 10)
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
	if len(chats) != 1 || chats[0].JID != lid {
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
