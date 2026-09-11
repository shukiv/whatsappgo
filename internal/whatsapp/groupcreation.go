package whatsapp

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.mau.fi/whatsmeow/types"
)

// Validate before making any network request. The creator is added by WhatsApp,
// leaving room for 1023 selected members in a 1024-member group.
func validateGroupCreation(name string, participants []string) (string, []types.JID, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || name == "" || utf8.RuneCountInString(name) > 100 {
		return "", nil, errors.New("a group name must contain between 1 and 100 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return "", nil, errors.New("group names cannot contain control characters")
		}
	}
	if len(participants) == 0 || len(participants) > 1023 {
		return "", nil, errors.New("select between 1 and 1023 other members")
	}
	targets := make([]types.JID, 0, len(participants))
	seen := make(map[types.JID]bool)
	for _, value := range participants {
		jid, err := types.ParseJID(strings.TrimSpace(value))
		if err != nil || jid.User == "" || strings.Trim(jid.User, "0123456789") != "" ||
			(jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
			return "", nil, errors.New("group members must be WhatsApp user addresses")
		}
		jid = jid.ToNonAD()
		if !seen[jid] {
			seen[jid] = true
			targets = append(targets, jid)
		}
	}
	return name, targets, nil
}
