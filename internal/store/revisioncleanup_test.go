package store

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

func chatWithAnEditedMessage(t *testing.T, name string) (*Store, context.Context) {
	t.Helper()
	st, err := Open("file:" + name + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.UpsertMessage(ctx, model.Message{
		ID: "m1", ChatJID: "alice@lid", SenderJID: "alice@lid", Timestamp: 1,
		Kind: "text", Body: "the first wording", Status: "received",
	}, "Alice", false); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordMessageRevision(ctx, "alice@lid", "m1", 2); err != nil {
		t.Fatal(err)
	}
	revisions, err := st.ListMessageRevisions(ctx, "alice@lid", "m1")
	if err != nil || len(revisions) == 0 {
		t.Fatalf("the earlier wording was not recorded: %v %#v", err, revisions)
	}
	return st, ctx
}

// Clearing a conversation has to take the earlier wording of an edited message
// with it. The revisions hang off the chat row rather than the message rows,
// and clearing keeps the chat row, so they used to survive a clear and leave
// the text of messages the reader had asked to be rid of.
func TestClearingAChatTakesEarlierWordingsWithIt(t *testing.T) {
	st, ctx := chatWithAnEditedMessage(t, t.Name())
	if err := st.ClearChatMessages(ctx, "alice@lid"); err != nil {
		t.Fatal(err)
	}
	revisions, err := st.ListMessageRevisions(ctx, "alice@lid", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 0 {
		t.Fatalf("a cleared conversation still holds earlier wordings: %#v", revisions)
	}
}

func TestDeletingAChatTakesEarlierWordingsWithIt(t *testing.T) {
	st, ctx := chatWithAnEditedMessage(t, t.Name())
	if err := st.DeleteChat(ctx, "alice@lid"); err != nil {
		t.Fatal(err)
	}
	revisions, err := st.ListMessageRevisions(ctx, "alice@lid", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 0 {
		t.Fatalf("a deleted conversation still holds earlier wordings: %#v", revisions)
	}
}

// The choices a question offered hang off the conversation, like the earlier
// wordings above, so they go the same way when it is cleared or deleted.
func TestClearingAChatForgetsTheChoicesItsQuestionsOffered(t *testing.T) {
	st, ctx := chatWithAnEditedMessage(t, t.Name())
	if err := st.RecordPromptChoices(ctx, "alice@lid", "m1", []string{"yes", "no"}); err != nil {
		t.Fatal(err)
	}
	offered, err := st.PromptOffersChoice(ctx, "alice@lid", "m1", "yes")
	if err != nil || !offered {
		t.Fatalf("the choices were not recorded: %v %v", err, offered)
	}
	if err := st.ClearChatMessages(ctx, "alice@lid"); err != nil {
		t.Fatal(err)
	}
	offered, err = st.PromptOffersChoice(ctx, "alice@lid", "m1", "yes")
	if err != nil {
		t.Fatal(err)
	}
	if offered {
		t.Fatal("a cleared conversation still vouches for a question's choices")
	}
}

func TestAQuestionNeverRecordedVouchesForNothing(t *testing.T) {
	st, ctx := chatWithAnEditedMessage(t, t.Name())
	offered, err := st.PromptOffersChoice(ctx, "alice@lid", "m1", "yes")
	if err != nil {
		t.Fatal(err)
	}
	if offered {
		t.Fatal("a question whose choices were never recorded vouched for one")
	}
}

// Two identities for one person are merged by moving the messages across. A
// question that lost its choices on the way would keep its text and lose its
// buttons, which reads as a question nobody can answer.
func TestAMergedChatKeepsItsQuestions(t *testing.T) {
	st, err := Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	const alias = "11111@s.whatsapp.net"
	const canonical = "22222@lid"
	question := model.Message{ID: "q1", ChatJID: alias, Kind: "text", Body: "Approve?",
		Timestamp: 1, Prompt: &model.MessagePrompt{
			Footer:  "from the agent",
			Choices: []model.MessageChoice{{ID: "yes", Title: "Approve"}},
		}}
	if err := st.UpsertMessage(ctx, question, "", false); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkChatAliases(ctx, canonical, alias); err != nil {
		t.Fatal(err)
	}
	moved, err := st.GetMessage(ctx, canonical, "q1")
	if err != nil {
		t.Fatalf("the question did not survive the merge: %v", err)
	}
	if moved.Prompt == nil {
		t.Fatal("the merged question kept its text and lost its choices")
	}
	if len(moved.Prompt.Choices) != 1 || moved.Prompt.Choices[0].ID != "yes" {
		t.Fatalf("choices after the merge = %+v", moved.Prompt.Choices)
	}
	if moved.Prompt.Footer != "from the agent" {
		t.Fatalf("footer after the merge = %q", moved.Prompt.Footer)
	}
}
