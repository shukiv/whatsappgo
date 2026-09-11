package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var _ gateway.GroupManager = (*Client)(nil)

func (c *Client) readGroup(ctx context.Context, value string) (*types.GroupInfo, error) {
	jid, err := types.ParseJID(strings.TrimSpace(value))
	if err != nil || jid.User == "" || jid.Server != types.GroupServer {
		return nil, errors.New("a group chat_jid is required")
	}
	if c.wa == nil || !c.wa.IsConnected() || !c.wa.IsLoggedIn() {
		return nil, errors.New("connect to WhatsApp to load group members")
	}
	info, err := c.wa.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, errors.New("WhatsApp returned no group information")
	}
	return info, nil
}

func (c *Client) describeGroupMember(ctx context.Context, p types.GroupParticipant) model.GroupMember {
	m := model.GroupMember{JID: p.JID.ToNonAD().String(), Name: p.DisplayName, IsAdmin: p.IsAdmin || p.IsSuperAdmin, IsOwner: p.IsSuperAdmin, Aliases: []string{}}
	seen := map[string]bool{}
	for _, jid := range []types.JID{p.JID, p.PhoneNumber, p.LID} {
		jid = jid.ToNonAD()
		if jid.IsEmpty() || seen[jid.String()] {
			continue
		}
		seen[jid.String()] = true
		m.Aliases = append(m.Aliases, jid.String())
		if jid.Server == types.DefaultUserServer {
			m.Phone = jid.User
		}
		if (c.wa.Store.ID != nil && jid == c.wa.Store.ID.ToNonAD()) || jid == c.wa.Store.GetLID().ToNonAD() {
			m.IsSelf = true
		}
		if known, err := c.store.GetChat(ctx, jid.String()); err == nil {
			if known.Title != "" && known.Title != displayJID(known.JID) {
				m.Name = known.Title
			}
			if known.AvatarPath != "" {
				m.AvatarPath = known.AvatarPath
			}
		}
		if m.Name == "" {
			if contact, err := c.wa.Store.Contacts.GetContact(ctx, jid); err == nil {
				m.Name = firstNonEmpty(contact.FullName, contact.FirstName, contact.BusinessName, contact.PushName)
			}
		}
	}
	if m.Name == "" && m.Phone != "" {
		m.Name = "+" + m.Phone
	}
	// An LID is not a phone number. Do not mislabel its opaque digits as one.
	if m.Name == "" {
		m.Name = "WhatsApp member"
	}
	return m
}

func (c *Client) GroupInfo(ctx context.Context, chatJID string) (model.GroupInfo, error) {
	info, err := c.readGroup(ctx, chatJID)
	if err != nil {
		return model.GroupInfo{}, err
	}
	result := model.GroupInfo{JID: info.JID.String(), Name: info.Name, Description: info.Topic,
		Participants: make([]model.GroupMember, 0, len(info.Participants)), ParticipantCount: info.ParticipantCount}
	if !info.GroupCreated.IsZero() {
		result.CreatedAt = info.GroupCreated.UnixMilli()
	}
	result.Creator = c.describeGroupMember(ctx, types.GroupParticipant{JID: info.OwnerJID, PhoneNumber: info.OwnerPN})
	for _, p := range info.Participants {
		member := c.describeGroupMember(ctx, p)
		result.Participants = append(result.Participants, member)
		if member.IsSelf {
			result.IsMember = true
			result.CanManage = member.IsAdmin
		}
	}
	if result.ParticipantCount < len(result.Participants) {
		result.ParticipantCount = len(result.Participants)
	}
	result.CanAdd = result.IsMember && (result.CanManage || info.MemberAddMode == types.GroupMemberAddModeAllMember)
	result.CanInvite = result.CanAdd
	result.CanEditInfo = c.canEditGroupInfo(info)
	result.Permissions = groupPermissionValues(info)
	result.CanEditPermissions = canEditGroupPermissions(info, c.wa.Store.GetJID(), c.wa.Store.GetLID())
	return result, nil
}

func (c *Client) GroupInviteLink(ctx context.Context, chatJID string, reset bool) (string, error) {
	info, err := c.GroupInfo(ctx, chatJID)
	if err != nil {
		return "", err
	}
	if !info.CanInvite || (reset && !info.CanManage) {
		return "", errors.New("only permitted members can access group invitations; only admins can reset the link")
	}
	jid, _ := types.ParseJID(info.JID)
	return c.wa.GetGroupInviteLink(ctx, jid, reset)
}

func (c *Client) ChangeGroupMembers(ctx context.Context, chatJID, action string, members []string) error {
	if action != "add" && action != "remove" && action != "promote" && action != "demote" {
		return errors.New("unsupported group member action")
	}
	if len(members) == 0 || len(members) > 100 {
		return errors.New("select between 1 and 100 members")
	}
	info, err := c.GroupInfo(ctx, chatJID)
	if err != nil {
		return err
	}
	if (action == "add" && !info.CanAdd) || (action != "add" && !info.CanManage) {
		return errors.New("you do not have permission to change these group members")
	}
	known := map[string]model.GroupMember{}
	for _, member := range info.Participants {
		for _, alias := range member.Aliases {
			known[alias] = member
		}
	}
	changes := make([]types.JID, 0, len(members))
	seen := map[string]bool{}
	for _, value := range members {
		jid, err := types.ParseJID(strings.TrimSpace(value))
		if err != nil || jid.User == "" || (jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
			return errors.New("participants must be WhatsApp user addresses")
		}
		jid = jid.ToNonAD()
		member, exists := known[jid.String()]
		if action == "add" && exists {
			continue
		}
		if action != "add" {
			if !exists {
				return errors.New("a selected person is no longer in this group; refresh and try again")
			}
			if member.IsSelf || member.IsOwner {
				return errors.New("this action cannot change yourself or the group owner")
			}
			jid, _ = types.ParseJID(member.JID)
		}
		if !seen[jid.String()] {
			seen[jid.String()] = true
			changes = append(changes, jid)
		}
	}
	if len(changes) == 0 {
		return errors.New("the selected people are already members")
	}
	jid, _ := types.ParseJID(info.JID)
	results, err := c.wa.UpdateGroupParticipants(ctx, jid, changes, whatsmeow.ParticipantChange(action))
	if err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "group.updated", Data: map[string]string{"jid": info.JID}})
	failed := 0
	for _, result := range results {
		if result.Error != 0 {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("WhatsApp could not update %d of %d selected members (permissions or invitation privacy); the member list has been refreshed", failed, len(changes))
	}
	if len(results) != len(changes) {
		return errors.New("WhatsApp did not confirm every member change; refresh the group before retrying")
	}
	return nil
}

func (c *Client) LeaveGroup(ctx context.Context, chatJID string) error {
	info, err := c.readGroup(ctx, chatJID)
	if err != nil {
		return err
	}
	if err = c.wa.LeaveGroup(ctx, info.JID); err != nil {
		return err
	}
	// Leaving never deletes the locally retained conversation or its media.
	c.emit(gateway.Event{Name: "group.updated", Data: map[string]string{"jid": info.JID.String(), "left": "true"}})
	return nil
}
