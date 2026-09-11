package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

func (s *Service) handleGroupRequests(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		ChatJID string `json:"chat_jid"`
		model.GroupJoinReview
	}
	if err := decode(raw, &p); err != nil {
		return nil, err
	}
	if p.ChatJID == "" {
		return nil, errors.New("chat_jid is required")
	}
	if method == "group.requests.review" {
		if err := p.GroupJoinReview.Validate(); err != nil {
			return nil, err
		}
	}
	gw, ok := s.gateway.(gateway.GroupRequestManager)
	if !ok {
		return nil, errors.New("join request management is unavailable with this backend")
	}
	if method == "group.requests.list" {
		requests, err := gw.GroupJoinRequests(ctx, p.ChatJID)
		return map[string]any{"requests": requests}, err
	}
	err := gw.ReviewGroupJoinRequest(ctx, p.ChatJID, p.GroupJoinReview)
	return map[string]bool{"ok": err == nil}, err
}
