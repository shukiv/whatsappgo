package model

import (
	"errors"
	"strings"
)

// conversationServers are the address spaces a message may be written to: one
// person by phone number, one person by their privacy-preserving identifier,
// or a group.
var conversationServers = map[string]bool{
	"s.whatsapp.net": true,
	"lid":            true,
	"g.us":           true,
}

// ValidateSendTarget refuses a destination that a conversation send must never
// reach.
//
// A status update arrives as an ordinary message whose chat is the status
// broadcast address, so a program that answers an incoming message by replying
// to the chat it came from would publish a status update to its contacts
// instead of writing to a person. Publishing a status is status.post's work,
// which asks who may see it. Channels and call addresses are not conversations
// this client writes to either.
//
// Only the destination is validated. A reply may still point at a message that
// lives in another chat, which is how a reply to a status is addressed to the
// person who posted it.
func ValidateSendTarget(chatJID string) error {
	target := strings.TrimSpace(chatJID)
	if target == "" {
		return errors.New("chat_jid is required")
	}
	at := strings.LastIndex(target, "@")
	if at <= 0 || at == len(target)-1 {
		return errors.New("chat_jid must be a chat address such as 15551234567@s.whatsapp.net")
	}
	server := target[at+1:]
	if server == "broadcast" {
		return errors.New("this address broadcasts to your contacts; publish a status with status.post instead")
	}
	if !conversationServers[server] {
		return errors.New("send to a person or a group; " + server + " addresses are not conversations")
	}
	return nil
}
