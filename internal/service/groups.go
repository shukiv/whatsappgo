package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

func (s *Service) handleGroupInfoEdit(ctx context.Context, raw json.RawMessage) (any, error) {
	var p struct {
		ChatJID string `json:"chat_jid"`
		model.GroupInfoEdit
	}
	if err := decode(raw, &p); err != nil {
		return nil, err
	}
	if err := p.GroupInfoEdit.Validate(); err != nil {
		return nil, err
	}
	gw, ok := s.gateway.(gateway.GroupInfoEditor)
	if !ok {
		return nil, errors.New("editing group information is unavailable with this backend")
	}
	err := gw.SetGroupInfo(ctx, p.ChatJID, p.GroupInfoEdit)
	return map[string]bool{"ok": err == nil}, err
}

func (s *Service) handleGroup(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p struct {
		ChatJID      string   `json:"chat_jid"`
		Action       string   `json:"action"`
		Participants []string `json:"participants"`
		Reset        bool     `json:"reset"`
	}
	if err := decode(raw, &p); err != nil {
		return nil, err
	}
	gw, ok := s.gateway.(gateway.GroupManager)
	if !ok {
		return nil, errors.New("group management is unavailable with this backend")
	}
	switch method {
	case "group.info":
		return gw.GroupInfo(ctx, p.ChatJID)
	case "group.invite_link":
		link, err := gw.GroupInviteLink(ctx, p.ChatJID, p.Reset)
		return map[string]string{"link": link}, err
	case "group.members":
		err := gw.ChangeGroupMembers(ctx, p.ChatJID, p.Action, p.Participants)
		return map[string]bool{"ok": err == nil}, err
	case "group.leave":
		err := gw.LeaveGroup(ctx, p.ChatJID)
		return map[string]bool{"ok": err == nil}, err
	}
	return nil, errors.New("unsupported group method")
}
