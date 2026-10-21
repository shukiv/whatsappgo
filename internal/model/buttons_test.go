package model

import (
	"strconv"
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

func TestAListKeepsItsChoicesInOrder(t *testing.T) {
	prompt, err := NewListPrompt(" Which day suits you? ", " this week ", " Choose a day ", []PromptSection{
		{Title: " This week ", Rows: []PromptRow{
			{ID: " mon ", Title: " Monday ", Description: " Morning only "},
			{ID: "tue", Title: "Tuesday"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Text != "Which day suits you?" || prompt.Footer != "this week" || prompt.ButtonText != "Choose a day" {
		t.Fatalf("prompt = %#v", prompt)
	}
	if len(prompt.Sections) != 1 || prompt.Sections[0].Title != "This week" || len(prompt.Sections[0].Rows) != 2 {
		t.Fatalf("sections = %#v", prompt.Sections)
	}
	if prompt.Sections[0].Rows[0] != (PromptRow{ID: "mon", Title: "Monday", Description: "Morning only"}) {
		t.Fatalf("first choice = %#v", prompt.Sections[0].Rows[0])
	}
}

func TestAListIsRefusedWhenAChoiceCouldNotBeActedOn(t *testing.T) {
	ok := []PromptSection{{Rows: []PromptRow{{ID: "mon", Title: "Monday"}}}}
	eleven := []PromptSection{{Rows: nil}}
	for i := 0; i < 11; i++ {
		eleven[0].Rows = append(eleven[0].Rows, PromptRow{ID: strconv.Itoa(i), Title: "Day " + strconv.Itoa(i)})
	}
	for _, tc := range []struct {
		name     string
		text     string
		footer   string
		button   string
		sections []PromptSection
	}{
		{"no text", "  ", "", "Choose", ok},
		{"text too long", strings.Repeat("x", 4097), "", "Choose", ok},
		{"no button label", "Which day?", "", " ", ok},
		{"button label too long", "Which day?", "", strings.Repeat("b", 21), ok},
		{"no sections", "Which day?", "", "Choose", nil},
		{"a section with no choices", "Which day?", "", "Choose", []PromptSection{{Title: "This week"}}},
		// Ten is the ceiling across the whole list, not per section.
		{"too many choices in all", "Which day?", "", "Choose", eleven},
		{"too many choices across sections", "Which day?", "", "Choose", []PromptSection{
			{Rows: []PromptRow{{ID: "a1", Title: "A1"}, {ID: "a2", Title: "A2"}, {ID: "a3", Title: "A3"}, {ID: "a4", Title: "A4"}, {ID: "a5", Title: "A5"}, {ID: "a6", Title: "A6"}}},
			{Rows: []PromptRow{{ID: "b1", Title: "B1"}, {ID: "b2", Title: "B2"}, {ID: "b3", Title: "B3"}, {ID: "b4", Title: "B4"}, {ID: "b5", Title: "B5"}}},
		}},
		// Across the whole list, not merely within one section.
		{"repeated identifier across sections", "Which day?", "", "Choose", []PromptSection{
			{Rows: []PromptRow{{ID: "mon", Title: "Monday"}}},
			{Rows: []PromptRow{{ID: "mon", Title: "Tuesday"}}},
		}},
		{"repeated title across sections", "Which day?", "", "Choose", []PromptSection{
			{Rows: []PromptRow{{ID: "mon", Title: "Monday"}}},
			{Rows: []PromptRow{{ID: "tue", Title: " Monday"}}},
		}},
		{"no choice identifier", "Which day?", "", "Choose", []PromptSection{{Rows: []PromptRow{{ID: " ", Title: "Monday"}}}}},
		{"choice title too long", "Which day?", "", "Choose", []PromptSection{{Rows: []PromptRow{{ID: "mon", Title: strings.Repeat("t", 25)}}}}},
		{"choice description too long", "Which day?", "", "Choose", []PromptSection{{Rows: []PromptRow{{ID: "mon", Title: "Monday", Description: strings.Repeat("d", 73)}}}}},
		{"section heading too long", "Which day?", "", "Choose", []PromptSection{{Title: strings.Repeat("s", 25), Rows: []PromptRow{{ID: "mon", Title: "Monday"}}}}},
		{"choice title with a newline", "Which day?", "", "Choose", []PromptSection{{Rows: []PromptRow{{ID: "mon", Title: "Mon\nday"}}}}},
	} {
		if _, err := NewListPrompt(tc.text, tc.footer, tc.button, tc.sections); err == nil {
			t.Fatalf("%s was accepted", tc.name)
		}
	}
}

func TestAListOfExactlyTenChoicesIsAccepted(t *testing.T) {
	var rows []PromptRow
	for i := 0; i < 10; i++ {
		rows = append(rows, PromptRow{ID: strconv.Itoa(i), Title: "Day " + strconv.Itoa(i)})
	}
	if _, err := NewListPrompt("Which day?", "", "Choose", []PromptSection{{Rows: rows}}); err != nil {
		t.Fatalf("ten choices were refused: %v", err)
	}
}
