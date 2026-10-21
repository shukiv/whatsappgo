package model

import (
	"strings"
	"testing"
)

func TestAPromptKeepsItsButtonsInOrder(t *testing.T) {
	prompt, err := NewButtonPrompt("  Approve the transfer?  ", " ref 4821 ", []PromptButton{
		{ID: " yes ", Title: " Approve "}, {ID: "no", Title: "Reject"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Text != "Approve the transfer?" || prompt.Footer != "ref 4821" {
		t.Fatalf("prompt = %#v", prompt)
	}
	if len(prompt.Buttons) != 2 || prompt.Buttons[0] != (PromptButton{ID: "yes", Title: "Approve"}) || prompt.Buttons[1].ID != "no" {
		t.Fatalf("buttons = %#v", prompt.Buttons)
	}
}

func TestAPromptIsRefusedWhenAChoiceCouldNotBeActedOn(t *testing.T) {
	ok := []PromptButton{{ID: "yes", Title: "Approve"}}
	for _, tc := range []struct {
		name    string
		text    string
		footer  string
		buttons []PromptButton
	}{
		{"no text", "   ", "", ok},
		{"text too long", strings.Repeat("x", 1025), "", ok},
		// A prompt whose text carries a line separator could be made to look
		// like it ended and something else began.
		{"text with a control character", "Approve?\u0007", "", ok},
		{"footer too long", "Approve?", strings.Repeat("x", 61), ok},
		{"footer with a newline", "Approve?", "ref\n4821", ok},
		{"no buttons", "Approve?", "", nil},
		{"too many buttons", "Approve?", "", []PromptButton{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}, {ID: "c", Title: "C"}, {ID: "d", Title: "D"}}},
		// Two buttons answering with the same identifier would leave the caller
		// unable to tell which one the reader pressed.
		{"repeated identifier", "Approve?", "", []PromptButton{{ID: "yes", Title: "Approve"}, {ID: " yes", Title: "Reject"}}},
		// WhatsApp requires the titles to differ too, and a reader cannot tell
		// two identical answers apart either.
		{"repeated title", "Approve?", "", []PromptButton{{ID: "yes", Title: "Approve"}, {ID: "no", Title: " Approve"}}},
		{"no identifier", "Approve?", "", []PromptButton{{ID: "  ", Title: "Approve"}}},
		{"identifier too long", "Approve?", "", []PromptButton{{ID: strings.Repeat("i", 257), Title: "Approve"}}},
		{"identifier with a newline", "Approve?", "", []PromptButton{{ID: "yes\nno", Title: "Approve"}}},
		{"no title", "Approve?", "", []PromptButton{{ID: "yes", Title: " "}}},
		{"title too long", "Approve?", "", []PromptButton{{ID: "yes", Title: strings.Repeat("t", 21)}}},
		{"title with a newline", "Approve?", "", []PromptButton{{ID: "yes", Title: "Approve\nnow"}}},
	} {
		if _, err := NewButtonPrompt(tc.text, tc.footer, tc.buttons); err == nil {
			t.Fatalf("%s was accepted", tc.name)
		}
	}
}

// A body may legitimately wrap, so a newline there is kept rather than refused.
func TestAPromptKeepsLineBreaksInItsQuestion(t *testing.T) {
	prompt, err := NewButtonPrompt("Approve the transfer?\n\nref 4821", "", []PromptButton{{ID: "yes", Title: "Approve"}})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Text != "Approve the transfer?\n\nref 4821" {
		t.Fatalf("text = %q", prompt.Text)
	}
}
