package model

import (
	"strings"
	"testing"
)

func TestSendTargetRefusesAddressesThatAreNotConversations(t *testing.T) {
	refused := map[string]string{
		"the status broadcast":      "status@broadcast",
		"any broadcast list":        "1234567890@broadcast",
		"a channel":                 "123456@newsletter",
		"a call address":            "123456@call",
		"an empty chat":             "",
		"blank space":               "   ",
		"an address with no @":      "1234567890",
		"an address with no user":   "@s.whatsapp.net",
		"an address with no server": "1234567890@",
	}
	for name, target := range refused {
		if err := ValidateSendTarget(target); err == nil {
			t.Fatalf("%s (%q) was accepted as a send destination", name, target)
		}
	}
}

func TestSendTargetAcceptsPeopleAndGroups(t *testing.T) {
	for _, target := range []string{
		"1234567890@s.whatsapp.net",
		"98765432109876@lid",
		"120363000000000000@g.us",
		"  120363000000000000@g.us  ",
	} {
		if err := ValidateSendTarget(target); err != nil {
			t.Fatalf("%q was refused: %v", target, err)
		}
	}
}

func TestStatusBroadcastRefusalNamesTheRightMethod(t *testing.T) {
	err := ValidateSendTarget("status@broadcast")
	if err == nil {
		t.Fatal("the status broadcast address was accepted")
	}
	if got := err.Error(); !strings.Contains(got, "status.post") {
		t.Fatalf("refusal does not say what to use instead: %q", got)
	}
}
