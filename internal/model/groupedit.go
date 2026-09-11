package model

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GroupInfoEdit updates one field, so a description save never overwrites a
// concurrently changed name (or vice versa). Empty description clears it.
type GroupInfoEdit struct {
	Field    string  `json:"field"`
	Value    string  `json:"value"`
	Previous *string `json:"previous"`
}

func (e GroupInfoEdit) Validate() error {
	if e.Previous == nil {
		return errors.New("previous group information is required; reopen the editor")
	}
	if !utf8.ValidString(e.Value) || !utf8.ValidString(*e.Previous) {
		return errors.New("group information must be valid text")
	}
	limit := 2048
	switch e.Field {
	case "name":
		limit = 100
		if strings.TrimSpace(e.Value) == "" {
			return errors.New("a group name is required")
		}
	case "description":
	default:
		return errors.New("only the group name or description can be edited")
	}
	if utf8.RuneCountInString(e.Value) > limit {
		return errors.New("group name must be at most 100 characters; description at most 2048")
	}
	for _, r := range e.Value {
		if unicode.IsControl(r) && !(e.Field == "description" && (r == '\n' || r == '\t')) || e.Field == "name" && (r == '\u2028' || r == '\u2029') {
			return errors.New("group information contains unsupported control characters")
		}
	}
	return nil
}
