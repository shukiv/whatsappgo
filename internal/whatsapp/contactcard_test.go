package whatsapp

import (
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/types"
)

func phoneLine(t *testing.T, vcard string) string {
	t.Helper()
	for _, line := range strings.Split(vcard, "\r\n") {
		if strings.HasPrefix(line, "TEL") {
			return line
		}
	}
	t.Fatalf("no telephone line in %q", vcard)
	return ""
}

// A card for a number that is on WhatsApp must carry the waid parameter, or the
// recipient's client can only offer to invite a contact who is already there.
func TestASharedNumberOnWhatsAppIsMarkedAsAnAccount(t *testing.T) {
	card, err := model.NewContactCard("Adony Robles", "+57 311 2522689")
	if err != nil {
		t.Fatal(err)
	}
	vcard := contactCardPayload(card, "573112522689", nil).GetContactMessage().GetVcard()
	if got := phoneLine(t, vcard); got != "TEL;type=CELL;type=VOICE;waid=573112522689:+573112522689" {
		t.Fatalf("telephone line = %q", got)
	}
	if got := phoneFromVCard(vcard); got != card.Phone {
		t.Fatalf("own card no longer parses: phone = %q", got)
	}
	if got := nameFromVCard(vcard); got != card.Name {
		t.Fatalf("own card no longer parses: name = %q", got)
	}
}

// A number nobody could confirm must stay an invitation: claiming an account
// that does not exist would leave the recipient with a dead "Message" button.
func TestASharedNumberWithoutAnAccountStaysAnInvitation(t *testing.T) {
	card, err := model.NewContactCard("Adony Robles", "+57 311 2522689")
	if err != nil {
		t.Fatal(err)
	}
	vcard := contactCardPayload(card, "", nil).GetContactMessage().GetVcard()
	if strings.Contains(vcard, "waid") {
		t.Fatalf("unconfirmed number claimed an account: %q", vcard)
	}
	if got := phoneLine(t, vcard); got != "TEL;type=CELL;type=VOICE:+573112522689" {
		t.Fatalf("telephone line = %q", got)
	}
	if got := phoneFromVCard(vcard); got != card.Phone {
		t.Fatalf("own card no longer parses: phone = %q", got)
	}
}

func TestTheCardIdentifiesTheAccountTheServerAnswersWith(t *testing.T) {
	phone := func(user string) types.JID { return types.NewJID(user, types.DefaultUserServer) }
	hidden := func(user string) types.JID { return types.NewJID(user, types.HiddenUserServer) }
	for _, tc := range []struct {
		name    string
		results []types.IsOnWhatsAppResponse
		typed   string
		want    string
	}{
		// Argentina and Mexico register a different form of the number than the
		// one people dial, so the server's answer wins over the typed digits.
		{"canonical number wins", []types.IsOnWhatsAppResponse{{IsIn: true, JID: phone("5215512345678"), PhoneNumber: phone("525512345678")}}, "5215512345678", "525512345678"},
		{"phone jid without a phone number field", []types.IsOnWhatsAppResponse{{IsIn: true, JID: phone("573112522689")}}, "573112522689", "573112522689"},
		// A hidden identifier means nothing to the recipient, but the answer
		// still confirms the account, so the typed digits carry the card. This
		// is the live shape, not a hypothetical one: asked for +972524877788 on
		// 2026-10-16, the account answered with 254365210251510@lid, so a card
		// that only trusted the answer's identifier would carry no parameter
		// at all and would keep arriving as an invitation.
		{"hidden identifier falls back to the typed digits", []types.IsOnWhatsAppResponse{{IsIn: true, JID: hidden("207348303974441")}}, "972524877788", "972524877788"},
		{"not registered", []types.IsOnWhatsAppResponse{{IsIn: false, JID: phone("573112522689")}}, "573112522689", ""},
		{"nothing came back", nil, "573112522689", ""},
		// Anything but digits could forge further vCard properties.
		{"punctuation is never embedded", []types.IsOnWhatsAppResponse{{IsIn: true, JID: hidden("1")}}, "+57 311 2522689", ""},
	} {
		if got := contactWAID(tc.results, tc.typed); got != tc.want {
			t.Fatalf("%s: waid = %q, want %q", tc.name, got, tc.want)
		}
	}
}
