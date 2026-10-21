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

type ListPromptRequest struct {
	ChatJID string           `json:"chat_jid"`
	ReplyTo string           `json:"reply_to"`
	Prompt  model.ListPrompt `json:"prompt"`
}

// Optional, like ButtonPromptSender: a gateway without interactive-message
// support still implements Gateway.
type ListPromptSender interface {
	SendListPrompt(context.Context, ListPromptRequest) (model.Message, error)
}
