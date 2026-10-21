package gateway

import (
	"context"

	"github.com/shukiv/whatsappgo/internal/model"
)

type ButtonPromptRequest struct {
	ChatJID string             `json:"chat_jid"`
	ReplyTo string             `json:"reply_to"`
	Prompt  model.ButtonPrompt `json:"prompt"`
}

// Optional: gateways without interactive-message support still implement
// Gateway, and the service reports the feature as unavailable instead.
type ButtonPromptSender interface {
	SendButtonPrompt(context.Context, ButtonPromptRequest) (model.Message, error)
}
