package store

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

func seedRevisionMessage(t *testing.T, s *Store, body string) (string, string) {
	t.Helper()
	const chat = "alice@s.whatsapp.net"
	const id = "m1"
	msg := model.Message{ID: id, ChatJID: chat, Timestamp: 1000, Kind: "text", Body: body, Status: "received"}
	if err := s.UpsertMessage(context.Background(), msg, "Alice", false); err != nil {
		t.Fatal(err)
	}
	return chat, id
}

// Correcting a message keeps what it said before, so the reader can still see
// the wording they answered.
func TestAnEditKeepsTheVersionItReplaces(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	chat, id := seedRevisionMessage(t, s, "27 mil")

	if err := s.EditMessage(ctx, chat, id, "25 mil"); err != nil {
		t.Fatal(err)
	}
	revisions, err := s.ListMessageRevisions(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 {
		t.Fatalf("the replaced version was not kept: %#v", revisions)
	}
	if revisions[0].Body != "27 mil" || revisions[0].Reason != "edited" {
		t.Fatalf("the kept version is not the one that was replaced: %#v", revisions[0])
	}
	current, err := s.GetMessage(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if current.Body != "25 mil" {
		t.Fatalf("the correction did not take: %q", current.Body)
	}
}

// Several corrections read as a sequence, oldest first.
func TestEveryVersionIsKeptInOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	chat, id := seedRevisionMessage(t, s, "first")

	for _, body := range []string{"second", "third", "fourth"} {
		if err := s.EditMessage(ctx, chat, id, body); err != nil {
			t.Fatal(err)
		}
	}
	revisions, err := s.ListMessageRevisions(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"first", "second", "third"}
	if len(revisions) != len(want) {
		t.Fatalf("expected %d earlier versions, got %#v", len(want), revisions)
	}
	for i, body := range want {
		if revisions[i].Body != body {
			t.Fatalf("version %d is %q, expected %q", i, revisions[i].Body, body)
		}
		if revisions[i].Revision != i {
			t.Fatalf("the versions are not numbered in order: %#v", revisions)
		}
	}
}

// Deleting for everyone wipes the body, so the recorded version is the only way
// back to what the message said.
func TestADeletedMessageKeepsWhatItSaid(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	chat, id := seedRevisionMessage(t, s, "the message that was deleted")

	if err := s.MarkRevoked(ctx, chat, id); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetMessage(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if current.Body != "" || !current.Revoked {
		t.Fatalf("the message was not deleted: %#v", current)
	}
	revisions, err := s.ListMessageRevisions(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 {
		t.Fatalf("a deleted message kept nothing: %#v", revisions)
	}
	if revisions[0].Body != "the message that was deleted" {
		t.Fatalf("the kept version is not what the message said: %#v", revisions[0])
	}
	if revisions[0].Reason != "deleted" {
		t.Fatalf("the history does not say the message was deleted: %#v", revisions[0])
	}
}

// A message that was corrected and then deleted keeps both: what it first said
// and what it said when it was deleted.
func TestAnEditedThenDeletedMessageKeepsBoth(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	chat, id := seedRevisionMessage(t, s, "first wording")

	if err := s.EditMessage(ctx, chat, id, "corrected wording"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRevoked(ctx, chat, id); err != nil {
		t.Fatal(err)
	}
	revisions, err := s.ListMessageRevisions(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected both versions, got %#v", revisions)
	}
	if revisions[0].Body != "first wording" || revisions[0].Reason != "edited" {
		t.Fatalf("the first version was not kept: %#v", revisions[0])
	}
	if revisions[1].Body != "corrected wording" || revisions[1].Reason != "deleted" {
		t.Fatalf("the version present at deletion was not kept: %#v", revisions[1])
	}
}

// History synchronisation can deliver the same correction twice. The same text
// must not become two versions.
func TestTheSameVersionIsNotKeptTwice(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	chat, id := seedRevisionMessage(t, s, "only once")

	for i := 0; i < 3; i++ {
		if err := s.RecordMessageRevision(ctx, chat, id, 1000); err != nil {
			t.Fatal(err)
		}
	}
	revisions, err := s.ListMessageRevisions(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 {
		t.Fatalf("the same version was kept more than once: %#v", revisions)
	}
}

// A message with nothing to keep - a photo with no caption, say - records
// nothing rather than a row of empty text.
func TestAMessageWithNoTextKeepsNothing(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	chat, id := seedRevisionMessage(t, s, "")

	if err := s.MarkRevoked(ctx, chat, id); err != nil {
		t.Fatal(err)
	}
	revisions, err := s.ListMessageRevisions(ctx, chat, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 0 {
		t.Fatalf("an empty message recorded a version: %#v", revisions)
	}
}
