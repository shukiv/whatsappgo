package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/model"
)

// menuOn opens the menu for one conversation from the list, without opening
// the conversation itself.
func menuOn(t *testing.T, fake *fakeDaemon, chat model.Chat) *app {
	t.Helper()
	current := driveFake(t, fake, chatsMsg([]model.Chat{chat}))
	fake.forget()
	return pump(t, current, press("m"))
}

// What the menu offers depends on where the conversation stands, and choosing
// one turns it the other way.
func TestTheMenuTurnsAConversationTheOtherWay(t *testing.T) {
	fake := newFakeDaemon()
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example", UnreadCount: 2})
	if current.chooser == nil {
		t.Fatal("the menu did not open")
	}
	labels := strings.Join(menuLabels(current), "\n")
	for _, wanted := range []string{"pin this conversation", "archive it", "mark it read", "delete the conversation"} {
		if !strings.Contains(labels, wanted) {
			t.Fatalf("the menu does not offer %q:\n%s", wanted, labels)
		}
	}
	current = pump(t, current, press("1"))
	pinned := fake.only(t, "chat.pin")
	if pinned.text("chat_jid") != alice || pinned.params["value"] != true {
		t.Fatalf("pinning went as %v", pinned.params)
	}

	// A conversation already pinned and read is offered the other way round.
	fake.forget()
	current = menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example", Pinned: true, Archived: true})
	labels = strings.Join(menuLabels(current), "\n")
	for _, wanted := range []string{"unpin this conversation", "take it out of the archive", "mark it unread"} {
		if !strings.Contains(labels, wanted) {
			t.Fatalf("the menu does not offer %q:\n%s", wanted, labels)
		}
	}
	current = pump(t, current, press("3"))
	archived := fake.only(t, "chat.archive")
	if archived.params["value"] != false {
		t.Fatalf("taking it out of the archive went as %v", archived.params)
	}

	// And unpinning really unpins: every one of these keys has to send the
	// state the conversation is going to, not the one it is in.
	fake.forget()
	current = menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example", Pinned: true})
	current = pump(t, current, press("1"))
	unpinned := fake.only(t, "chat.pin")
	if unpinned.params["value"] != false {
		t.Fatalf("unpinning went as %v", unpinned.params)
	}
}

// Muting offers what the desktop client's menu offers, and "always" is zero
// seconds, which the daemon reads as until it is undone - not as a week.
func TestMutingOffersTheSameSpansAndAlwaysMeansUntilUndone(t *testing.T) {
	fake := newFakeDaemon()
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, press("2"))
	if got := strings.Join(menuLabels(current), ","); got != "notifications on,8 hours,1 week,always" {
		t.Fatalf("muting offers %q", got)
	}
	current = pump(t, current, press("4"))
	quiet := fake.only(t, "chat.mute")
	if quiet.params["value"] != true || quiet.number("duration_seconds") != 0 {
		t.Fatalf("always went as %v", quiet.params)
	}

	fake.forget()
	current = menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, press("2"), press("2"))
	eight := fake.only(t, "chat.mute")
	if eight.number("duration_seconds") != 8*60*60 {
		t.Fatalf("eight hours went as %v", eight.params)
	}
}

// How long messages live starts on what the conversation is set to now, as the
// desktop client's dialog does.
func TestHowLongMessagesLiveStartsOnWhatIsSetNow(t *testing.T) {
	fake := newFakeDaemon()
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example", DisappearingSeconds: 604800})
	current = pump(t, current, chooseLabel(t, current, "how long messages live"))
	if current.chooser == nil || current.chooser.cursor != 2 {
		t.Fatalf("the prompt starts on %v", current.chooser)
	}
	current = pump(t, current, press("4"))
	set := fake.only(t, "chat.disappearing")
	if set.number("duration_seconds") != 7776000 {
		t.Fatalf("ninety days went as %v", set.params)
	}
}

// Emptying a conversation and deleting it both ask first, and both start on the
// answer that changes nothing.
func TestEmptyingAndDeletingAConversationAskFirst(t *testing.T) {
	fake := newFakeDaemon()
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, chooseLabel(t, current, "empty it of messages"))
	// What the prompt says is checked where it is said: on screen it is
	// wrapped to the pane, so which row a word lands on is not the point.
	if current.chooser.cursor != 0 || !strings.Contains(current.chooser.hint, "cannot be undone") {
		t.Fatalf("the prompt is %v", current.chooser)
	}
	if !strings.Contains(current.View(), "Empty this conversation") {
		t.Fatalf("the prompt is not on screen:\n%s", current.View())
	}
	current = pump(t, current, press("enter"))
	if emptied := fake.calls("chat.clear"); len(emptied) != 0 {
		t.Fatalf("the default answer emptied it: %v", emptied)
	}

	current = pump(t, current, press("m"))
	current = pump(t, current, chooseLabel(t, current, "delete the conversation"), press("j"), press("enter"))
	deleted := fake.only(t, "chat.delete")
	if deleted.text("chat_jid") != alice {
		t.Fatalf("the deletion went as %v", deleted.params)
	}
}

// A conversation that has been deleted cannot stay on screen.
func TestDeletingTheOpenConversationClosesIt(t *testing.T) {
	fake := newFakeDaemon().answer("messages.list", aliceMessages())
	current := driveFake(t, fake, aliceChat(), press("enter"), press("esc"))
	current = pump(t, current, press("m"))
	current = pump(t, current, chooseLabel(t, current, "delete the conversation"), press("j"), press("enter"))
	if current.open.JID != "" || len(current.messages) != 0 {
		t.Fatalf("the deleted conversation is still open: %q", current.open.JID)
	}
	if current.focus != paneChats {
		t.Fatal("the keyboard was left in a conversation that is gone")
	}
}

// What is in a conversation is read from the daemon rather than guessed at
// from the list row.
func TestWhatIsInAConversationIsRead(t *testing.T) {
	fake := newFakeDaemon().answer("chat.info", model.ChatInfo{
		Chat:       model.Chat{JID: alice, Title: "Alice Example", Pinned: true},
		Phone:      "+972500000001",
		LastSeen:   noon,
		MediaCount: 12, DocumentCount: 3, LinkCount: 7, SharedCount: 22,
	})
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, chooseLabel(t, current, "what is in this conversation"))
	if asked := fake.only(t, "chat.info"); asked.text("chat_jid") != alice {
		t.Fatalf("it was asked for as %v", asked.params)
	}
	view := current.View()
	for _, wanted := range []string{"+972500000001", "pictures and video: 12", "pinned to the top"} {
		if !strings.Contains(view, wanted) {
			t.Fatalf("what is in it does not mention %q:\n%s", wanted, view)
		}
	}
}

// What has been shared is asked for one kind at a time, in the daemon's own
// words for them.
func TestWhatHasBeenSharedIsAskedForByKind(t *testing.T) {
	fake := newFakeDaemon().answer("chat.shared", model.SharedMessagePage{
		Messages: []model.Message{{ID: "s1", Timestamp: noon, Kind: "document",
			MediaName: "lease.pdf", MediaSize: 245_760}},
		HasMore: true,
	})
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, chooseLabel(t, current, "what has been shared in it"))
	if got := strings.Join(menuLabels(current), ","); got != "media,documents,links,stickers" {
		t.Fatalf("the kinds offered are %q", got)
	}
	current = pump(t, current, press("2"))
	asked := fake.only(t, "chat.shared")
	if asked.text("category") != "documents" || asked.text("chat_jid") != alice {
		t.Fatalf("it was asked for as %v", asked.params)
	}
	if !strings.Contains(current.View(), "lease.pdf") || !strings.Contains(current.View(), "and more") {
		t.Fatalf("what was shared is not on screen:\n%s", current.View())
	}
}

// The lists a conversation is on are read, shown with a mark against the ones
// it is on, and turned the other way by choosing one.
func TestTheListsAConversationIsOnAreTurnedByChoosingOne(t *testing.T) {
	fake := newFakeDaemon().
		answer("labels.list", map[string]any{"labels": []model.Label{
			{ID: "1", Name: "Work"}, {ID: "2", Name: "Family"}}}).
		answer("chat.labels", map[string]any{"label_ids": []string{"2"}})
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, chooseLabel(t, current, "the lists it is on"))
	labels := menuLabels(current)
	if len(labels) != 2 || !strings.HasPrefix(labels[1], "✓") || strings.HasPrefix(labels[0], "✓") {
		t.Fatalf("the lists read %v", labels)
	}
	current = pump(t, current, press("1"))
	put := fake.only(t, "chat.label")
	if put.text("label_id") != "1" || put.params["value"] != true {
		t.Fatalf("putting it on a list went as %v", put.params)
	}
}

// Writing a conversation to a file asks where. The daemon will not replace a
// file unless it is told to, so a file already there comes back as a question
// rather than as a loss.
func TestWritingAConversationToAFileAsksBeforeReplacingOne(t *testing.T) {
	fake := newFakeDaemon().refuse("chat.export", errors.New("file exists")).
		answer("chat.export", map[string]any{"path": "/tmp/alice.txt"})
	current := menuOn(t, fake, model.Chat{JID: alice, Title: "Alice Example"})
	current = pump(t, current, chooseLabel(t, current, "write it to a file"))
	if current.prompt == nil {
		t.Fatal("nothing asked where to write it")
	}
	if !strings.HasSuffix(current.prompt.input.Value(), "Alice-Example.txt") {
		t.Fatalf("the file suggested is %q", current.prompt.input.Value())
	}

	current = pump(t, current, press("enter"))
	first := fake.calls("chat.export")
	if len(first) != 1 || first[0].params["replace"] != false {
		t.Fatalf("the first attempt went as %v", first)
	}
	// Refused because the file is there: it asks again, and says why.
	if current.prompt == nil || !strings.Contains(current.View(), "already there") {
		t.Fatalf("it did not ask about replacing the file:\n%s", current.View())
	}
	current = pump(t, current, press("enter"))
	second := fake.calls("chat.export")
	if len(second) != 2 || second[1].params["replace"] != true {
		t.Fatalf("the second attempt went as %v", second)
	}
	if !strings.Contains(current.View(), "/tmp/alice.txt") {
		t.Fatalf("where it was written is not on screen:\n%s", current.View())
	}
}

// Going back to a day asks the daemon which message that was, then reads older
// pages until that message is here and puts the cursor on it.
func TestGoingBackToADayReadsUntilThatMessageIsHere(t *testing.T) {
	first := aliceMessages()
	first.HasMore, first.NextBefore, first.NextBeforeID = true, noon, "m1"
	older := model.MessagePage{Messages: []model.Message{
		{ID: "m0", ChatJID: alice, SenderJID: alice, Timestamp: noon - 86_400_000,
			Kind: "text", Body: "the day before"},
	}}
	fake := newFakeDaemon().answer("messages.list", first, older).
		answer("messages.on_date", map[string]any{"chat_jid": alice, "message_id": "m0"})

	current := driveFake(t, fake, aliceChat(), press("enter"), press("esc"))
	fake.forget()
	current = pump(t, current, press("m"))
	current = pump(t, current, chooseLabel(t, current, "go back to a date"))
	day := time.UnixMilli(noon - 86_400_000).Format("2006-01-02")
	current = pump(t, current, typed(day), press("enter"))

	asked := fake.only(t, "messages.on_date")
	start, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if asked.number("start") != start.UnixMilli() || asked.number("end") != start.AddDate(0, 0, 1).UnixMilli()-1 {
		t.Fatalf("the day was asked for as %v", asked.params)
	}
	if message, ok := current.selectedMessage(); !ok || message.ID != "m0" {
		t.Fatalf("the cursor is on %q", message.ID)
	}
	if !strings.Contains(current.View(), "the day before") {
		t.Fatalf("that day is not on screen:\n%s", current.View())
	}
}

// Finding a conversation asks the daemon, and the list shows what came back
// until it is asked for again.
func TestFindingAConversationShowsWhatCameBack(t *testing.T) {
	fake := newFakeDaemon().answer("chats.search", []model.Chat{{JID: alice, Title: "Alice Example"}})
	current := driveFake(t, fake, chatsMsg([]model.Chat{
		{JID: "972500000002@s.whatsapp.net", Title: "Bob Example"}}))
	fake.forget()

	current = pump(t, current, press("/"), typed("alice"), press("enter"))
	asked := fake.only(t, "chats.search")
	if asked.text("query") != "alice" {
		t.Fatalf("the search went as %v", asked.params)
	}
	view := current.View()
	if !strings.Contains(view, "Alice Example") || strings.Contains(view, "Bob Example") {
		t.Fatalf("the list does not show what was found:\n%s", view)
	}
	if !strings.Contains(view, "alice") {
		t.Fatalf("the window does not say the list is a search:\n%s", view)
	}
}

// The archive is kept out of the list until it is asked for.
func TestTheArchiveIsAskedForOnItsOwn(t *testing.T) {
	fake := newFakeDaemon().answer("chats.list", []model.Chat{
		{JID: alice, Title: "Alice Example", Archived: true}})
	current := driveFake(t, fake, chatsMsg(nil))
	fake.forget()
	current = pump(t, current, press("A"))
	asked := fake.only(t, "chats.list")
	if asked.params["archived"] != true || asked.number("limit") != 200 {
		t.Fatalf("the archive was asked for as %v", asked.params)
	}
	if !strings.Contains(current.View(), "the archive") {
		t.Fatalf("the window does not say the list is the archive:\n%s", current.View())
	}
}

// Marking everything read reaches every conversation on the account and tells
// everybody who wrote, so it asks first.
func TestMarkingEverythingReadAsksFirst(t *testing.T) {
	fake := newFakeDaemon()
	current := driveFake(t, fake, aliceChat())
	fake.forget()
	current = pump(t, current, press("M"))
	if current.chooser == nil || current.chooser.cursor != 0 {
		t.Fatalf("the prompt is %v", current.chooser)
	}
	current = pump(t, current, press("enter"))
	if marked := fake.calls("chats.mark_all_read"); len(marked) != 0 {
		t.Fatalf("the default answer marked everything read: %v", marked)
	}
	current = pump(t, current, press("M"), press("j"), press("enter"))
	if marked := fake.calls("chats.mark_all_read"); len(marked) != 1 {
		t.Fatalf("marking everything read went as %v", marked)
	}
}

// The two numbers the desktop client keeps above its list are kept above this
// one.
func TestHowManyAreWaitingAndArchivedAreShown(t *testing.T) {
	fake := newFakeDaemon().
		answer("chats.unread_count", map[string]int{"count": 14}).
		answer("chats.archived_count", map[string]int{"count": 3})
	current := driveFake(t, fake, aliceChat(), do(newApp(nil, fake, "default").counts()))
	view := current.View()
	if !strings.Contains(view, "14 messages waiting") || !strings.Contains(view, "3 archived") {
		t.Fatalf("the window does not carry the counts:\n%s", view)
	}
}

// menuLabels is what the prompt on screen is offering.
func menuLabels(current *app) []string {
	if current.chooser == nil {
		return nil
	}
	found := make([]string, 0, len(current.chooser.items))
	for _, item := range current.chooser.items {
		found = append(found, item.label)
	}
	return found
}

// chooseLabel presses the row with that label on it, so a test says what it
// means rather than counting rows.
func chooseLabel(t *testing.T, current *app, label string) tea.Msg {
	t.Helper()
	for at, item := range current.chooser.items {
		if strings.Contains(item.label, label) {
			current.chooser.cursor = at
			return press("enter")
		}
	}
	t.Fatalf("no row says %q: %v", label, menuLabels(current))
	return nil
}
