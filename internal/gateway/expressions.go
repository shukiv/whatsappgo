package gateway

import (
	"context"
	"github.com/shukiv/whatsappgo/internal/model"
)

// StickerSender is optional so gateways without media support remain usable.
type StickerSender interface {
	SendStoredSticker(ctx context.Context, fromChat, messageID, toChat, replyTo string) (model.Message, error)
}
