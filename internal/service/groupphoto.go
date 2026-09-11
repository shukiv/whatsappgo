package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
)

func (s *Service) handleGroupPhoto(ctx context.Context, raw json.RawMessage) (any, error) {
	var p struct {
		ChatJID string `json:"chat_jid"`
		Path    string `json:"path"`
		Remove  bool   `json:"remove"`
	}
	if err := decode(raw, &p); err != nil {
		return nil, err
	}
	if p.ChatJID == "" || (p.Path == "") != p.Remove {
		return nil, errors.New("chat_jid and a photo path, or remove=true without a path, are required")
	}
	gw, ok := s.gateway.(gateway.GroupPhotoEditor)
	if !ok {
		return nil, errors.New("group photo editing is unavailable with this backend")
	}
	path, err := gw.SetGroupPhoto(ctx, p.ChatJID, p.Path, p.Remove)
	return map[string]any{"ok": err == nil, "avatar_path": path}, err
}
