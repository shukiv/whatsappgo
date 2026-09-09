package gateway

import (
	"context"

	"github.com/shukiv/whatsappgo/internal/model"
)

type ContactCardRequest struct {
	ChatJID string            `json:"chat_jid"`
	ReplyTo string            `json:"reply_to"`
	Contact model.ContactCard `json:"contact"`
}

// Optional: gateways without contact-message support still implement Gateway.
type ContactCardSender interface {
	SendContactCard(context.Context, ContactCardRequest) (model.Message, error)
}
