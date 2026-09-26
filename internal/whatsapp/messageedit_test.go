package whatsapp

import (
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

// editEnvelope is the protocol envelope a correction is carried in: the key of
// the message being corrected, and the text that replaces it.
func editEnvelope(target, text string) *waE2E.Message {
	return &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
		Type:          waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
		Key:           &waCommon.MessageKey{ID: proto.String(target)},
		EditedMessage: &waE2E.Message{Conversation: proto.String(text)},
	}}
}

// An edit does not always arrive wrapped in editedMessage. A copy of one made
// on another of our own devices can come through as the bare envelope, which
// the library has nothing to unwrap and so does not mark as an edit. The
// stanza's own edit attribute is then the only thing that says what it is.
func TestEditWithoutItsWrapperLandsOnTheMessageItCorrects(t *testing.T) {
	chat := types.NewJID("123", types.DefaultUserServer)
	edit := &waEvents.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, IsFromMe: true},
			ID:            "edit-stanza",
			Timestamp:     time.UnixMilli(2000),
			Edit:          types.EditAttributeMessageEdit,
		},
		Message: editEnvelope("original", "the corrected text"),
	}
	m := messageFromEvent(edit)
	if m.ID != "original" {
		t.Fatalf("the correction arrived as a message of its own: %q", m.ID)
	}
	if m.Body != "the corrected text" || m.Kind != "text" {
		t.Fatalf("the correction did not carry its new text: %#v", m)
	}
	if !m.Edited {
		t.Fatal("the corrected message is not marked as edited")
	}
}

// The envelope on its own is enough. History carries no stanza attributes, so
// a correction that reaches us long after it was made has nothing but its own
// shape to go by.
func TestAnEnvelopeAloneIsStillRecognisedAsAnEdit(t *testing.T) {
	chat := types.NewJID("123", types.DefaultUserServer)
	edit := &waEvents.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat},
			ID:            "edit-stanza",
			Timestamp:     time.UnixMilli(2000),
		},
		Message: editEnvelope("original", "the corrected text"),
	}
	m := messageFromEvent(edit)
	if m.ID != "original" || m.Body != "the corrected text" || !m.Edited {
		t.Fatalf("an unmarked correction was read as a new message: %#v", m)
	}
}

// A newsletter is corrected differently again: the message is the new text
// itself, the id is already the original's, and an attribute of its own says
// an editor changed it.
func TestANewsletterCorrectionIsRecognisedByItsAttribute(t *testing.T) {
	chat := types.NewJID("123", types.NewsletterServer)
	edit := &waEvents.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat},
			ID:            "original",
			Timestamp:     time.UnixMilli(2000),
			Edit:          types.EditAttributeAdminEdit,
		},
		Message: &waE2E.Message{Conversation: proto.String("the corrected text")},
	}
	m := messageFromEvent(edit)
	if m.ID != "original" || m.Body != "the corrected text" || !m.Edited {
		t.Fatalf("a corrected newsletter message was read as a new one: %#v", m)
	}
}

// Nothing else may be mistaken for a correction, least of all an ordinary
// message, which would be marked as edited and would suppress its own alert.
func TestAnOrdinaryMessageIsNotTakenForACorrection(t *testing.T) {
	chat := types.NewJID("123", types.DefaultUserServer)
	plain := &waEvents.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat},
			ID:            "plain",
			Timestamp:     time.UnixMilli(2000),
		},
		Message: &waE2E.Message{Conversation: proto.String("hello")},
	}
	if messageIsEdit(plain) {
		t.Fatal("an ordinary message was taken for a correction")
	}
	// A message being taken back is not a correction either, and is handled
	// well before this.
	revoke := &waEvents.Message{
		Info:    plain.Info,
		Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_REVOKE.Enum()}},
	}
	if messageIsEdit(revoke) {
		t.Fatal("a message taken back was treated as a correction")
	}
	if messageIsEdit(&waEvents.Message{Info: plain.Info}) {
		t.Fatal("a message with nothing in it was treated as a correction")
	}
}

// End to end: an unwrapped correction replaces what the reader sees and keeps
// what the message said before.
func TestAnUnwrappedCorrectionReplacesTheStoredMessage(t *testing.T) {
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	chat := types.NewJID("123", types.DefaultUserServer)
	c := &Client{store: st, subs: make(map[uint64]func(gateway.Event))}

	original := model.Message{ID: "original", ChatJID: chat.String(), SenderJID: chat.String(), Kind: "text", Body: "what was written first", Timestamp: 1000, FromMe: true, Status: "sent"}
	if err := st.UpsertMessage(ctx, original, "", false); err != nil {
		t.Fatal(err)
	}

	c.handleMessage(&waEvents.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat, IsFromMe: true},
			ID:            "edit-stanza",
			Timestamp:     time.UnixMilli(2000),
			Edit:          types.EditAttributeMessageEdit,
		},
		Message: editEnvelope("original", "what it says now"),
	})

	page, err := st.ListMessages(ctx, chat.String(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 {
		t.Fatalf("the correction was stored as a message of its own: %#v", page.Messages)
	}
	if page.Messages[0].Body != "what it says now" {
		t.Fatalf("the reader is still seeing what was written first: %#v", page.Messages[0])
	}
	if !page.Messages[0].Edited {
		t.Fatalf("the message is not marked as edited: %#v", page.Messages[0])
	}
	revisions, err := st.ListMessageRevisions(ctx, chat.String(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].Body != "what was written first" {
		t.Fatalf("what the message said before was not kept: %#v", revisions)
	}
}
