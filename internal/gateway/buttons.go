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

// PromptAnswerRequest presses one of the answers a received question offered.
type PromptAnswerRequest struct {
	ChatJID   string
	MessageID string
	ChoiceID  string
}

// PromptAnswerSender is implemented by a gateway that can answer a question
// somebody else asked. It is optional for the same reason the prompt senders
// are: a gateway that cannot do it should still be a gateway.
type PromptAnswerSender interface {
	SendPromptAnswer(ctx context.Context, req PromptAnswerRequest) (model.Message, error)
}
