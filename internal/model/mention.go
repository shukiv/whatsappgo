package model

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Mention struct {
	JID  string `json:"jid"`
	Name string `json:"name,omitempty"`
}

var mentionJID = regexp.MustCompile(`^[0-9]+@(s\.whatsapp\.net|lid)$`)

var mentionToken = regexp.MustCompile(`@[0-9]+`)

// MentionTokens returns whole identifier tokens, not email addresses or
// substrings of a longer username. Offsets are byte offsets into the wire text.
func MentionTokens(text string) [][2]int {
	var result [][2]int
	for _, span := range mentionToken.FindAllStringIndex(text, -1) {
		before, _ := utf8.DecodeLastRuneInString(text[:span[0]])
		after, _ := utf8.DecodeRuneInString(text[span[1]:])
		word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '@' }
		if word(before) || word(after) {
			continue
		}
		prefix := text[:span[0]]
		if separator := strings.LastIndexFunc(prefix, unicode.IsSpace); separator >= 0 {
			prefix = prefix[separator+1:]
		}
		prefix = strings.ToLower(prefix)
		if strings.Contains(prefix, "https://") || strings.Contains(prefix, "http://") {
			continue
		}
		result = append(result, [2]int{span[0], span[1]})
	}
	return result
}

// MentionDisplayText never changes the stored body or creates outgoing tags.
func MentionDisplayText(text string, mentions []Mention) string {
	names := map[string]string{}
	for _, mention := range mentions {
		if mentionJID.MatchString(mention.JID) && mention.Name != "" {
			names["@"+strings.SplitN(mention.JID, "@", 2)[0]] = "@" + mention.Name
		}
	}
	spans := MentionTokens(text)
	for i := len(spans) - 1; i >= 0; i-- {
		start, end := spans[i][0], spans[i][1]
		if name := names[text[start:end]]; name != "" {
			text = text[:start] + name + text[end:]
		}
	}
	return text
}

// NormalizeMentions accepts only explicit group mentions whose wire token is
// actually present. Plain @names and email addresses never become mentions.
func NormalizeMentions(chat, text string, mentions []Mention) ([]Mention, error) {
	if len(mentions) == 0 {
		return nil, nil
	}
	if !strings.HasSuffix(chat, "@g.us") {
		return nil, errors.New("mentions require a group chat")
	}
	if len(mentions) > 128 {
		return nil, errors.New("a message can mention at most 128 members")
	}
	result := make([]Mention, 0, len(mentions))
	seen := map[string]bool{}
	tokens := map[string]bool{}
	for _, span := range MentionTokens(text) {
		tokens[text[span[0]:span[1]]] = true
	}
	for _, mention := range mentions {
		if !mentionJID.MatchString(mention.JID) {
			return nil, errors.New("invalid mention member")
		}
		user := strings.SplitN(mention.JID, "@", 2)[0]
		if !tokens["@"+user] {
			return nil, errors.New("mention is missing from the message text")
		}
		if seen[mention.JID] {
			continue
		}
		seen[mention.JID] = true
		mention.Name = strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, mention.Name))
		if len([]rune(mention.Name)) > 128 {
			mention.Name = string([]rune(mention.Name)[:128])
		}
		result = append(result, mention)
	}
	return result, nil
}
