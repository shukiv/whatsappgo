package service

import (
	"context"
	"strings"

	"github.com/shukiv/whatsappgo/internal/model"
)

func (s *Service) resolveMessageMentions(ctx context.Context, m model.Message) model.Message {
	if !strings.HasSuffix(m.ChatJID, "@g.us") || !strings.Contains(m.Body, "@") {
		return m
	}
	if resolver, ok := s.gateway.(interface {
		ResolveMessageMentions(context.Context, model.Message) model.Message
	}); ok {
		return resolver.ResolveMessageMentions(ctx, m)
	}
	return m
}
