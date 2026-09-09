package gateway

import (
	"context"
	"github.com/shukiv/whatsappgo/internal/model"
)

// GroupManager is optional for offline gateways; the live client supplies it.
// Keeping live group requests separate avoids slowing down local chat.info.
type GroupManager interface {
	GroupInfo(context.Context, string) (model.GroupInfo, error)
	GroupInviteLink(context.Context, string, bool) (string, error)
	ChangeGroupMembers(context.Context, string, string, []string) error
	LeaveGroup(context.Context, string) error
}
