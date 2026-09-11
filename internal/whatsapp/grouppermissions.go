package whatsapp

import (
	"context"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/types"
)

var _ gateway.GroupPermissionEditor = (*Client)(nil)

type groupPermissionAPI interface {
	SetGroupLocked(context.Context, types.JID, bool) error
	SetGroupAnnounce(context.Context, types.JID, bool) error
	SetGroupMemberAddMode(context.Context, types.JID, types.GroupMemberAddMode) error
	SetGroupJoinApprovalMode(context.Context, types.JID, bool) error
}

func groupPermissionValues(info *types.GroupInfo) map[string]bool {
	if info == nil {
		return nil
	}
	values := map[string]bool{
		"send_messages":       !info.IsAnnounce,
		"edit_info":           !info.IsLocked,
		"approve_new_members": info.IsJoinApprovalRequired,
	}
	switch info.MemberAddMode {
	case types.GroupMemberAddModeAllMember:
		values["add_members"] = true
	case types.GroupMemberAddModeAdmin:
		values["add_members"] = false
	}
	return values
}

func canEditGroupPermissions(info *types.GroupInfo, selfPN, selfLID types.JID) bool {
	if info == nil || info.Suspended || info.IsParent || info.IsDefaultSubGroup {
		return false
	}
	for _, p := range info.Participants {
		if !p.IsAdmin && !p.IsSuperAdmin {
			continue
		}
		for _, jid := range []types.JID{p.JID, p.PhoneNumber, p.LID} {
			jid = jid.ToNonAD()
			if !jid.IsEmpty() && (jid == selfPN.ToNonAD() || jid == selfLID.ToNonAD()) {
				return true
			}
		}
	}
	return false
}

func applyGroupPermission(ctx context.Context, api groupPermissionAPI, info *types.GroupInfo, selfPN, selfLID types.JID, edit model.GroupPermissionEdit) error {
	if err := edit.Validate(); err != nil {
		return err
	}
	if !canEditGroupPermissions(info, selfPN, selfLID) {
		return errors.New("only current admins can change permissions for this group")
	}
	current, known := groupPermissionValues(info)[edit.Field]
	if !known {
		return errors.New("WhatsApp did not supply this permission; refresh group info before editing")
	}
	if current == *edit.Value {
		return nil
	} // Safe retry after a lost acknowledgement.
	if current != *edit.Previous {
		return errors.New("group permissions changed; reopen the setting before saving")
	}
	switch edit.Field {
	case "send_messages":
		return api.SetGroupAnnounce(ctx, info.JID, !*edit.Value)
	case "edit_info":
		return api.SetGroupLocked(ctx, info.JID, !*edit.Value)
	case "add_members":
		mode := types.GroupMemberAddModeAdmin
		if *edit.Value {
			mode = types.GroupMemberAddModeAllMember
		}
		return api.SetGroupMemberAddMode(ctx, info.JID, mode)
	case "approve_new_members":
		return api.SetGroupJoinApprovalMode(ctx, info.JID, *edit.Value)
	}
	return errors.New("unsupported group permission")
}

func (c *Client) SetGroupPermission(ctx context.Context, jid string, edit model.GroupPermissionEdit) error {
	if err := edit.Validate(); err != nil {
		return err
	}
	// readGroup validates the address and connection and fetches current metadata.
	info, err := c.readGroup(ctx, jid)
	if err != nil {
		return err
	}
	if c.wa.Store == nil {
		return errors.New("account identity is unavailable")
	}
	err = applyGroupPermission(ctx, c.wa, info, c.wa.Store.GetJID(), c.wa.Store.GetLID(), edit)
	// Refresh after errors too: a lost response may conceal an applied change.
	c.emit(gateway.Event{Name: "group.updated", Data: map[string]string{"jid": info.JID.String()}})
	return err
}
