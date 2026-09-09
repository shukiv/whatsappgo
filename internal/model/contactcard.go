package model

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ContactCard deliberately shares only the fields the sender reviews, never
// an address-book record containing notes, addresses or other private fields.
type ContactCard struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

func NewContactCard(name, phone string) (ContactCard, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 100 {
		return ContactCard{}, errors.New("contact name must contain 1–100 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ContactCard{}, errors.New("contact name must be a single line")
		}
	}
	phone = strings.TrimSpace(phone)
	if !strings.HasPrefix(phone, "+") {
		return ContactCard{}, errors.New("use an international phone number starting with + and the country code")
	}
	var digits strings.Builder
	for _, r := range phone[1:] {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
		default:
			return ContactCard{}, errors.New("phone number contains unsupported characters")
		}
	}
	value := digits.String()
	if len(value) < 7 || len(value) > 15 || value[0] == '0' {
		return ContactCard{}, errors.New("phone number must contain 7–15 digits including the country code")
	}
	return ContactCard{Name: name, Phone: "+" + value}, nil
}
