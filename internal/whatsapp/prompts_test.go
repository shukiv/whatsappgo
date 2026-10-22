package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// The shape this project sends, and the one a phone was shown on 2026-10-21:
// an approval question with two quick replies. It drew on the phone and was
// dropped here, so the reader is tested against the payload the sender builds
// rather than against a shape written out by hand, which would only prove the
// two agree with each other if somebody kept them in step.
func TestAQuestionWithButtonsIsRead(t *testing.T) {
	prompt, err := model.NewButtonPrompt("Approve?", "from the agent", []model.PromptButton{
		{ID: "yes", Title: "Approve"}, {ID: "no", Title: "Cancel"},
	})
	if err != nil {
		t.Fatalf("building the question: %v", err)
	}
	payload, _, err := buttonPromptPayload(prompt, nil, time.Now())
	if err != nil {
		t.Fatalf("building the payload: %v", err)
	}
	text, read, ok := promptFromMessage(payload)
	if !ok {
		t.Fatal("a question with buttons was not recognised as one")
	}
	if text != "Approve?" {
		t.Fatalf("question text = %q", text)
	}
	if read.Footer != "from the agent" {
		t.Fatalf("footer = %q", read.Footer)
	}
	if read.Label != "" {
		t.Fatalf("buttons shown under the question need no label, got %q", read.Label)
	}
	if len(read.Choices) != 2 {
		t.Fatalf("choices = %d, want 2", len(read.Choices))
	}
	if read.Choices[0].ID != "yes" || read.Choices[0].Title != "Approve" {
		t.Fatalf("first choice = %+v", read.Choices[0])
	}
	if read.Choices[1].ID != "no" || read.Choices[1].Title != "Cancel" {
		t.Fatalf("second choice = %+v", read.Choices[1])
	}
}

func TestAQuestionWithAListIsRead(t *testing.T) {
	prompt, err := model.NewListPrompt("Which day?", "", "Choose a day", []model.PromptSection{{
		Title: "This week",
		Rows: []model.PromptRow{
			{ID: "mon", Title: "Monday", Description: "Morning only"},
			{ID: "tue", Title: "Tuesday"},
		},
	}})
	if err != nil {
		t.Fatalf("building the list: %v", err)
	}
	payload, _, err := listPromptPayload(prompt, nil, time.Now())
	if err != nil {
		t.Fatalf("building the payload: %v", err)
	}
	text, read, ok := promptFromMessage(payload)
	if !ok {
		t.Fatal("a question with a list was not recognised as one")
	}
	if text != "Which day?" {
		t.Fatalf("question text = %q", text)
	}
	// The label is what says the choices sit behind a button rather than under
	// the question, which is the only difference the drawing needs.
	if read.Label != "Choose a day" {
		t.Fatalf("list label = %q", read.Label)
	}
	if len(read.Choices) != 2 {
		t.Fatalf("choices = %d, want 2", len(read.Choices))
	}
	if read.Choices[0].ID != "mon" || read.Choices[0].Description != "Morning only" {
		t.Fatalf("first row = %+v", read.Choices[0])
	}
}

// The legacy shapes cannot be sent by a linked device, but one can be sent to
// it, so each is read from a message built here.
func TestTheLegacyButtonShapeIsRead(t *testing.T) {
	msg := &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{
		ContentText: proto.String("Old question"),
		FooterText:  proto.String("footer"),
		Buttons: []*waE2E.ButtonsMessage_Button{
			{ButtonID: proto.String("a"), ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("First")}},
			{ButtonID: proto.String("b"), ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("Second")}},
		},
	}}
	text, read, ok := promptFromMessage(msg)
	if !ok || text != "Old question" || len(read.Choices) != 2 {
		t.Fatalf("ok=%v text=%q read=%+v", ok, text, read)
	}
	if read.Choices[1].ID != "b" || read.Choices[1].Title != "Second" {
		t.Fatalf("second choice = %+v", read.Choices[1])
	}
}

func TestTheLegacyListShapeIsRead(t *testing.T) {
	msg := &waE2E.Message{ListMessage: &waE2E.ListMessage{
		Description: proto.String("Pick one"),
		ButtonText:  proto.String("Open"),
		Sections: []*waE2E.ListMessage_Section{{
			Title: proto.String("All"),
			Rows: []*waE2E.ListMessage_Row{
				{RowID: proto.String("r1"), Title: proto.String("One"), Description: proto.String("the first")},
			},
		}},
	}}
	text, read, ok := promptFromMessage(msg)
	if !ok || text != "Pick one" || read.Label != "Open" {
		t.Fatalf("ok=%v text=%q read=%+v", ok, text, read)
	}
	if len(read.Choices) != 1 || read.Choices[0].Description != "the first" {
		t.Fatalf("choices = %+v", read.Choices)
	}
}

// A template's other buttons open a link or dial a number. Drawing one as a
// choice would offer to send something that answers nothing.
func TestATemplateOffersOnlyItsQuickReplies(t *testing.T) {
	msg := &waE2E.Message{TemplateMessage: &waE2E.TemplateMessage{
		Format: &waE2E.TemplateMessage_HydratedFourRowTemplate_{
			HydratedFourRowTemplate: &waE2E.TemplateMessage_HydratedFourRowTemplate{
				HydratedContentText: proto.String("Template"),
				HydratedButtons: []*waE2E.HydratedTemplateButton{
					{HydratedButton: &waE2E.HydratedTemplateButton_QuickReplyButton{
						QuickReplyButton: &waE2E.HydratedTemplateButton_HydratedQuickReplyButton{
							ID: proto.String("q"), DisplayText: proto.String("Reply"),
						}}},
					{HydratedButton: &waE2E.HydratedTemplateButton_UrlButton{
						UrlButton: &waE2E.HydratedTemplateButton_HydratedURLButton{
							URL: proto.String("https://example.com"), DisplayText: proto.String("Open site"),
						}}},
				},
			},
		},
	}}
	_, read, ok := promptFromMessage(msg)
	if !ok {
		t.Fatal("a template with a quick reply was not recognised as a question")
	}
	if len(read.Choices) != 1 || read.Choices[0].ID != "q" {
		t.Fatalf("a link button was offered as an answer: %+v", read.Choices)
	}
}

// An ordinary message must not come back as a question, or every text in the
// conversation would grow a row of controls.
func TestAnOrdinaryMessageIsNotAQuestion(t *testing.T) {
	if _, _, ok := promptFromMessage(&waE2E.Message{Conversation: proto.String("hello")}); ok {
		t.Fatal("a plain text message was read as a question")
	}
}

// A question whose choices are all unreadable is not a question. Offering it
// would put an empty row of controls under the text.
func TestAQuestionWithNoReadableChoicesIsNotOne(t *testing.T) {
	msg := &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{
		Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Question")},
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{{
					Name: proto.String(quickReplyButton), ButtonParamsJSON: proto.String("{not json"),
				}},
			},
		},
	}}
	if _, _, ok := promptFromMessage(msg); ok {
		t.Fatal("a question whose only choice would not parse was offered as one")
	}
}

// One unreadable choice must not cost the reader the others.
func TestOneUnreadableChoiceLeavesTheRest(t *testing.T) {
	msg := &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{
		Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Question")},
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
					{Name: proto.String(quickReplyButton), ButtonParamsJSON: proto.String("{not json")},
					{Name: proto.String(quickReplyButton), ButtonParamsJSON: proto.String(`{"id":"ok","display_text":"OK"}`)},
				},
			},
		},
	}}
	_, read, ok := promptFromMessage(msg)
	if !ok || len(read.Choices) != 1 || read.Choices[0].ID != "ok" {
		t.Fatalf("ok=%v read=%+v", ok, read)
	}
}

// The choice travels to whoever asked, so a caller must not be able to put one
// there that the question never offered. Each case stops before the socket,
// which is why the connection is checked last.
func TestAnAnswerMustBeOneTheQuestionOffered(t *testing.T) {
	for _, tc := range []struct {
		name     string
		question model.Message
		choiceID string
		want     string
	}{
		{
			name: "a choice that was never offered",
			question: model.Message{ID: "q1", ChatJID: "1@s.whatsapp.net", Kind: "text", Body: "Approve?",
				Prompt: &model.MessagePrompt{Choices: []model.MessageChoice{{ID: "yes", Title: "Approve"}}}},
			choiceID: "something-else",
			want:     "that question did not offer that choice",
		},
		{
			name:     "a message that asked nothing",
			question: model.Message{ID: "q1", ChatJID: "1@s.whatsapp.net", Kind: "text", Body: "hello"},
			choiceID: "yes",
			want:     "that message asked nothing that can be answered",
		},
		{
			name: "a question this account asked itself",
			question: model.Message{ID: "q1", ChatJID: "1@s.whatsapp.net", Kind: "text", Body: "Approve?", FromMe: true,
				Prompt: &model.MessagePrompt{Choices: []model.MessageChoice{{ID: "yes", Title: "Approve"}}}},
			choiceID: "yes",
			want:     "this account asked that question",
		},
		{
			// Reaching the socket is how this case says the checks passed:
			// there is no connection in a test, and that error comes last.
			name: "a choice the question did offer",
			question: model.Message{ID: "q1", ChatJID: "1@s.whatsapp.net", Kind: "text", Body: "Approve?",
				Prompt: &model.MessagePrompt{Choices: []model.MessageChoice{{ID: "yes", Title: "Approve"}}}},
			choiceID: "yes",
			want:     "WhatsApp is disconnected",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A database each: one shared between cases lets a later case pass
			// on an earlier one's stored question.
			c, st, _ := receivingClient(t)
			if err := st.UpsertMessage(context.Background(), tc.question, "", false); err != nil {
				t.Fatal(err)
			}
			_, err := c.SendPromptAnswer(context.Background(), gateway.PromptAnswerRequest{
				ChatJID: tc.question.ChatJID, MessageID: tc.question.ID, ChoiceID: tc.choiceID})
			if err == nil {
				t.Fatalf("answering was allowed; want %q", tc.want)
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

// A question that is not in this conversation cannot be answered, or a caller
// could name a message id from somewhere else entirely.
func TestAnAnswerNeedsAQuestionInThatConversation(t *testing.T) {
	c, _, _ := receivingClient(t)
	_, err := c.SendPromptAnswer(context.Background(), gateway.PromptAnswerRequest{
		ChatJID: "1@s.whatsapp.net", MessageID: "missing", ChoiceID: "yes"})
	if err == nil || err.Error() != "that question is not in this conversation" {
		t.Fatalf("error = %v", err)
	}
}

// Which refusals mean the stanza was wrong, and which mean something else
// entirely. Falling back on the wrong one would send the answer twice over a
// hiccup that a retry of the same shape would have survived.
func TestOnlyARefusedStanzaFallsBackToText(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "no error at all", err: nil, want: false},
		{name: "479, an invalid stanza", err: fmt.Errorf("%w %d", whatsmeow.ErrServerReturnedError, 479), want: true},
		{name: "405, a shape this device may not send", err: fmt.Errorf("%w %d", whatsmeow.ErrServerReturnedError, 405), want: true},
		{name: "503, the service being away", err: fmt.Errorf("%w %d", whatsmeow.ErrServerReturnedError, 503), want: false},
		{name: "a server error with no code", err: whatsmeow.ErrServerReturnedError, want: false},
		{name: "something that never reached the server", err: errors.New("dial unix: connection refused"), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stanzaRefused(tc.err); got != tc.want {
				t.Fatalf("stanzaRefused(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
