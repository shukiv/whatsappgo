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

func contactCardPayload(card model.ContactCard, reply *waE2E.ContextInfo) *waE2E.Message {
	// vCard 3.0 TEXT escaping prevents names from introducing new properties.
	escape := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,")
	name := escape.Replace(card.Name)
	vcard := "BEGIN:VCARD\r\nVERSION:3.0\r\nN:;" + name + ";;;\r\nFN:" + name +
		"\r\nTEL;TYPE=CELL:" + card.Phone + "\r\nEND:VCARD\r\n"
	return &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: proto.String(card.Name), Vcard: proto.String(vcard), ContextInfo: reply,
	}}
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
	payload := contactCardPayload(card, c.replyContext(ctx, req.ChatJID, req.ReplyTo))
	resp, err := c.wa.SendMessage(ctx, chat, payload)
	if err != nil {
		return model.Message{}, err
	}
	msg := model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(), Kind: "contact", Body: card.Name, FromMe: true,
		Status: "sent", ReplyTo: req.ReplyTo, ContactName: card.Name, ContactPhone: card.Phone, ContactCount: 1}
	return c.withReplyPreview(ctx, msg), nil
}
