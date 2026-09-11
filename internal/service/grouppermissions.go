package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

func (s *Service) handleGroupPermissionEdit(ctx context.Context, raw json.RawMessage) (any, error) {
	var p struct {
		ChatJID string `json:"chat_jid"`
		model.GroupPermissionEdit
	}
	if err := decode(raw, &p); err != nil {
		return nil, err
	}
	if err := p.GroupPermissionEdit.Validate(); err != nil {
		return nil, err
	}
	gw, ok := s.gateway.(gateway.GroupPermissionEditor)
	if !ok {
		return nil, errors.New("editing group permissions is unavailable with this backend")
	}
	err := gw.SetGroupPermission(ctx, p.ChatJID, p.GroupPermissionEdit)
	return map[string]bool{"ok": err == nil}, err
}
