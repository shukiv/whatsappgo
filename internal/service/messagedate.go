package service

import (
	"context"
	"encoding/json"
)

func (s *Service) messageOnDate(ctx context.Context, raw json.RawMessage) (any, error) {
	var p struct {
		ChatJID string `json:"chat_jid"`
		Start   int64  `json:"start"`
		End     int64  `json:"end"`
	}
	if err := decode(raw, &p); err != nil {
		return nil, err
	}
	id, err := s.store.MessageOnDate(ctx, p.ChatJID, p.Start, p.End)
	if err != nil {
		return nil, err
	}
	return map[string]any{"chat_jid": p.ChatJID, "message_id": id}, nil
}
