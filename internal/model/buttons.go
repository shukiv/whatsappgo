package model

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxPromptButtons is WhatsApp's own ceiling for a quick-reply prompt. More
// buttons than this are not shown rather than wrapped, so the extra ones would
// silently become choices nobody can make.
const MaxPromptButtons = 3

// PromptButton is one tappable choice. The identifier is what comes back when
// the reader taps, so it belongs to the caller that will have to act on it; the
// title is only what the reader sees.
type PromptButton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ButtonPrompt is a question with tappable answers.
type ButtonPrompt struct {
	Text    string         `json:"text"`
	Footer  string         `json:"footer"`
	Buttons []PromptButton `json:"buttons"`
}

// NewButtonPrompt checks a prompt before it can reach WhatsApp. Everything here
// arrives from a socket client, so each field is bounded and each identifier is
// required to be unique: a duplicate identifier would make a tap ambiguous, and
// the caller acting on it could not tell which choice the reader made.
func NewButtonPrompt(text, footer string, buttons []PromptButton) (ButtonPrompt, error) {
	prompt := ButtonPrompt{Text: strings.TrimSpace(text), Footer: strings.TrimSpace(footer)}
	if !utf8.ValidString(prompt.Text) || prompt.Text == "" || utf8.RuneCountInString(prompt.Text) > 1024 {
		return ButtonPrompt{}, errors.New("prompt text must contain 1–1024 characters")
	}
	if err := singleLine(prompt.Text, true, "prompt text"); err != nil {
		return ButtonPrompt{}, err
	}
	if !utf8.ValidString(prompt.Footer) || utf8.RuneCountInString(prompt.Footer) > 60 {
		return ButtonPrompt{}, errors.New("prompt footer must contain at most 60 characters")
	}
	if err := singleLine(prompt.Footer, false, "prompt footer"); err != nil {
		return ButtonPrompt{}, err
	}
	if len(buttons) < 1 || len(buttons) > MaxPromptButtons {
		return ButtonPrompt{}, errors.New("a prompt needs 1–" + strconv.Itoa(MaxPromptButtons) + " buttons")
	}
	seen := make(map[string]bool, len(buttons))
	titles := make(map[string]bool, len(buttons))
	prompt.Buttons = make([]PromptButton, 0, len(buttons))
	for _, button := range buttons {
		id := strings.TrimSpace(button.ID)
		title := strings.TrimSpace(button.Title)
		if !utf8.ValidString(id) || id == "" || utf8.RuneCountInString(id) > 256 {
			return ButtonPrompt{}, errors.New("each button needs an identifier of 1–256 characters")
		}
		if err := singleLine(id, false, "button identifier"); err != nil {
			return ButtonPrompt{}, err
		}
		if seen[id] {
			return ButtonPrompt{}, errors.New("button identifiers must differ: " + id + " is used twice")
		}
		seen[id] = true
		// Twenty characters is the limit WhatsApp documents for a reply button
		// title (developers.facebook.com/docs/whatsapp/guides/interactive-messages,
		// "Maximum of 20 characters for title"). A longer title is refused here
		// rather than sent and truncated by the reader's client, which would
		// hide part of a choice.
		if !utf8.ValidString(title) || title == "" || utf8.RuneCountInString(title) > 20 {
			return ButtonPrompt{}, errors.New("each button needs a title of 1–20 characters")
		}
		if err := singleLine(title, false, "button title"); err != nil {
			return ButtonPrompt{}, err
		}
		// WhatsApp requires the titles of several buttons to differ as well.
		// Two buttons reading the same are also a worse question: the reader
		// cannot tell which answer they are giving.
		if titles[title] {
			return ButtonPrompt{}, errors.New("button titles must differ: " + title + " is used twice")
		}
		titles[title] = true
		prompt.Buttons = append(prompt.Buttons, PromptButton{ID: id, Title: title})
	}
	return prompt, nil
}

// singleLine rejects the control characters that would let a caller's text
// break out of the field it was put in. Line feeds are allowed only where the
// field is a body of text that may legitimately wrap.
func singleLine(value string, allowNewlines bool, field string) error {
	for _, r := range value {
		if allowNewlines && (r == '\n' || r == '\r') {
			continue
		}
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return errors.New(field + " must not contain control characters")
		}
	}
	return nil
}

// MaxPromptRows is WhatsApp's ceiling for a list, counted across all of its
// sections rather than per section.
const MaxPromptRows = 10

// PromptRow is one choice in a list. As with a button, the identifier belongs
// to the caller that will act on the answer and the title is what is read.
type PromptRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// PromptSection groups rows under a heading. A list of unrelated choices may
// leave the heading empty.
type PromptSection struct {
	Title string      `json:"title"`
	Rows  []PromptRow `json:"rows"`
}

// ListPrompt is a question whose answers are too many to be buttons.
type ListPrompt struct {
	Text       string          `json:"text"`
	Footer     string          `json:"footer"`
	ButtonText string          `json:"button_text"`
	Sections   []PromptSection `json:"sections"`
}

// NewListPrompt checks a list before it can reach WhatsApp. The identifiers
// and the titles are both required to be distinct across the whole list, not
// merely within a section: a reader choosing between two rows that read the
// same cannot tell which answer they are giving, and a caller receiving a
// repeated identifier cannot tell which row was chosen.
//
// The lengths below are what WhatsApp's clients accept. Ten rows is documented
// and agreed everywhere; the twenty-four and seventy-two character limits come
// from third-party guides rather than from Meta's own reference, and are not
// verified here. They are applied as refusals because the alternative those
// guides describe is worse: an over-long title is cut off on the reader's
// screen with no error, which hides part of a choice.
func NewListPrompt(text, footer, buttonText string, sections []PromptSection) (ListPrompt, error) {
	prompt := ListPrompt{
		Text:       strings.TrimSpace(text),
		Footer:     strings.TrimSpace(footer),
		ButtonText: strings.TrimSpace(buttonText),
	}
	if !utf8.ValidString(prompt.Text) || prompt.Text == "" || utf8.RuneCountInString(prompt.Text) > 4096 {
		return ListPrompt{}, errors.New("prompt text must contain 1–4096 characters")
	}
	if err := singleLine(prompt.Text, true, "prompt text"); err != nil {
		return ListPrompt{}, err
	}
	if !utf8.ValidString(prompt.Footer) || utf8.RuneCountInString(prompt.Footer) > 60 {
		return ListPrompt{}, errors.New("prompt footer must contain at most 60 characters")
	}
	if err := singleLine(prompt.Footer, false, "prompt footer"); err != nil {
		return ListPrompt{}, err
	}
	if !utf8.ValidString(prompt.ButtonText) || prompt.ButtonText == "" || utf8.RuneCountInString(prompt.ButtonText) > 20 {
		return ListPrompt{}, errors.New("the button that opens the list needs a label of 1–20 characters")
	}
	if err := singleLine(prompt.ButtonText, false, "list button label"); err != nil {
		return ListPrompt{}, err
	}
	if len(sections) < 1 || len(sections) > MaxPromptRows {
		return ListPrompt{}, errors.New("a list needs 1–" + strconv.Itoa(MaxPromptRows) + " sections")
	}
	ids := make(map[string]bool)
	titles := make(map[string]bool)
	rows := 0
	for _, section := range sections {
		checked := PromptSection{Title: strings.TrimSpace(section.Title)}
		if !utf8.ValidString(checked.Title) || utf8.RuneCountInString(checked.Title) > 24 {
			return ListPrompt{}, errors.New("a section heading must contain at most 24 characters")
		}
		if err := singleLine(checked.Title, false, "section heading"); err != nil {
			return ListPrompt{}, err
		}
		for _, row := range section.Rows {
			id := strings.TrimSpace(row.ID)
			title := strings.TrimSpace(row.Title)
			description := strings.TrimSpace(row.Description)
			rows++
			if rows > MaxPromptRows {
				return ListPrompt{}, errors.New("a list may offer at most " + strconv.Itoa(MaxPromptRows) + " choices in all")
			}
			if !utf8.ValidString(id) || id == "" || utf8.RuneCountInString(id) > 256 {
				return ListPrompt{}, errors.New("each choice needs an identifier of 1–256 characters")
			}
			if err := singleLine(id, false, "choice identifier"); err != nil {
				return ListPrompt{}, err
			}
			if ids[id] {
				return ListPrompt{}, errors.New("choice identifiers must differ: " + id + " is used twice")
			}
			ids[id] = true
			if !utf8.ValidString(title) || title == "" || utf8.RuneCountInString(title) > 24 {
				return ListPrompt{}, errors.New("each choice needs a title of 1–24 characters")
			}
			if err := singleLine(title, false, "choice title"); err != nil {
				return ListPrompt{}, err
			}
			if titles[title] {
				return ListPrompt{}, errors.New("choice titles must differ: " + title + " is used twice")
			}
			titles[title] = true
			if !utf8.ValidString(description) || utf8.RuneCountInString(description) > 72 {
				return ListPrompt{}, errors.New("a choice description must contain at most 72 characters")
			}
			if err := singleLine(description, false, "choice description"); err != nil {
				return ListPrompt{}, err
			}
			checked.Rows = append(checked.Rows, PromptRow{ID: id, Title: title, Description: description})
		}
		if len(checked.Rows) == 0 {
			return ListPrompt{}, errors.New("a section needs at least one choice")
		}
		prompt.Sections = append(prompt.Sections, checked)
	}
	if rows == 0 {
		return ListPrompt{}, errors.New("a list needs at least one choice")
	}
	return prompt, nil
}
