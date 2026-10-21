package whatsapp

import (
	"context"
	"errors"
	"strings"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// contactCardPayload builds the vCard WhatsApp carries for a shared contact.
// The waid parameter is what tells the recipient's client that the number
// belongs to a registered account: with it the card offers "Message" and shows
// the contact's profile photo, without it the client can only offer "Invite".
// An empty waid therefore means "not confirmed to be on WhatsApp", and the card
// stays an invitation rather than claiming an account that may not exist.
func contactCardPayload(card model.ContactCard, waid string, reply *waE2E.ContextInfo) *waE2E.Message {
	// vCard 3.0 TEXT escaping prevents names from introducing new properties.
	escape := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,")
	name := escape.Replace(card.Name)
	tel := "TEL;type=CELL;type=VOICE"
	if waid != "" {
		tel += ";waid=" + waid
	}
	vcard := "BEGIN:VCARD\r\nVERSION:3.0\r\nN:;" + name + ";;;\r\nFN:" + name +
		"\r\n" + tel + ":" + card.Phone + "\r\nEND:VCARD\r\n"
	return &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: proto.String(card.Name), Vcard: proto.String(vcard), ContextInfo: reply,
	}}
}

// contactWAID reads the digits that identify a shared number's WhatsApp account
// from a registration lookup. The canonical number the server answers with wins
// over the digits the sender typed, because some countries register a different
// form of the same number than the one people dial. A lookup that only returns
// a hidden identifier still confirms the account exists, so the typed digits
// are used instead of dropping the parameter. Only digits are ever returned: a
// value carrying vCard punctuation could otherwise forge extra properties.
func contactWAID(results []types.IsOnWhatsAppResponse, typed string) string {
	for _, result := range results {
		if !result.IsIn {
			continue
		}
		for _, jid := range []types.JID{result.PhoneNumber, result.JID} {
			if jid.Server == types.DefaultUserServer && onlyDigits(jid.User) {
				return jid.User
			}
		}
		if onlyDigits(typed) {
			return typed
		}
	}
	return ""
}

func onlyDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (c *Client) SendContactCard(ctx context.Context, req gateway.ContactCardRequest) (model.Message, error) {
	card, err := model.NewContactCard(req.Contact.Name, req.Contact.Phone)
	if err != nil {
		return model.Message{}, err
	}
	chat, err := types.ParseJID(req.ChatJID)
	if err != nil || chat.User == "" || (chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.GroupServer) {
		return model.Message{}, errors.New("a direct or group conversation is required")
	}
	if c.wa == nil || !c.wa.IsConnected() {
		return model.Message{}, errors.New("WhatsApp is disconnected")
	}
	// A failed lookup must not block the send: the card still arrives, only as
	// an invitation, so a hiccup degrades the card instead of losing the message.
	waid := ""
	if results, err := c.wa.IsOnWhatsApp(ctx, []string{card.Phone}); err == nil {
		waid = contactWAID(results, strings.TrimPrefix(card.Phone, "+"))
	}
	payload := contactCardPayload(card, waid, c.replyContext(ctx, req.ChatJID, req.ReplyTo))
	resp, err := c.wa.SendMessage(ctx, chat, payload)
	if err != nil {
		return model.Message{}, err
	}
	msg := model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(), Kind: "contact", Body: card.Name, FromMe: true,
		Status: "sent", ReplyTo: req.ReplyTo, ContactName: card.Name, ContactPhone: card.Phone, ContactCount: 1}
	return c.withReplyPreview(ctx, msg), nil
}
