package whatsapp

import (
	"context"
	"errors"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// statusBackgrounds are the plain colours WhatsApp offers behind a text status.
// The wire value is a packed ARGB integer, so the palette lives here rather
// than in the interface, which should not have to know that.
var statusBackgrounds = []int32{
	int32(-15494520), // teal
	int32(-15064194), // green
	int32(-1499549),  // red
	int32(-16537100), // blue
	int32(-4144960),  // grey
}

// PostTextStatus publishes a text status update.
//
// WhatsApp treats a status as a message to the status broadcast address;
// whatsmeow works out who receives it from the account's own status privacy,
// so nothing here has to build a recipient list.
func (c *Client) PostTextStatus(ctx context.Context, text string, background int) (model.Message, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return model.Message{}, errors.New("status text is required")
	}
	if len([]rune(trimmed)) > 700 {
		return model.Message{}, errors.New("a status update is limited to 700 characters")
	}
	if background < 0 || background >= len(statusBackgrounds) {
		background = 0
	}
	payload := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String(trimmed),
			BackgroundArgb: proto.Uint32(uint32(statusBackgrounds[background])),
		},
	}
	resp, err := c.wa.SendMessage(ctx, types.StatusBroadcastJID, payload)
	if err != nil {
		return model.Message{}, err
	}
	result := model.Message{
		ID:        string(resp.ID),
		ChatJID:   types.StatusBroadcastJID.String(),
		SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(),
		Kind:      "text",
		Body:      trimmed,
		FromMe:    true,
		Status:    "sent",
	}
	c.emit(gateway.Event{Name: "status.posted", Data: map[string]any{"id": result.ID}})
	return result, nil
}

// PostMediaStatus publishes a photo or video as a status update. It goes
// through the same upload path as any other attachment, addressed to the status
// broadcast rather than to a conversation.
func (c *Client) PostMediaStatus(ctx context.Context, path, caption string) (model.Message, error) {
	if strings.TrimSpace(path) == "" {
		return model.Message{}, errors.New("a file is required")
	}
	message, err := c.SendMedia(ctx, gateway.MediaRequest{
		ChatJID: types.StatusBroadcastJID.String(),
		Path:    path,
		Caption: caption,
	})
	if err != nil {
		return model.Message{}, err
	}
	c.emit(gateway.Event{Name: "status.posted", Data: map[string]any{"id": message.ID}})
	return message, nil
}
