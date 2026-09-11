package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
)

func (s *Service) handleInteractiveFeature(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	gw, ok := s.gateway.(gateway.InteractiveFeatures)
	if !ok {
		return nil, errors.New("this feature is unavailable with the current backend")
	}
	return gw.InteractiveFeature(ctx, method, raw)
}
