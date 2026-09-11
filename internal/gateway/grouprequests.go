package gateway

import (
	"context"
	"github.com/shukiv/whatsappgo/internal/model"
)

// GroupRequestManager is optional and only available for live group admins.
type GroupRequestManager interface {
	GroupJoinRequests(context.Context, string) ([]model.GroupJoinRequest, error)
	ReviewGroupJoinRequest(context.Context, string, model.GroupJoinReview) error
}
