package whatsapp

import (
	"context"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/types"
)

var _ gateway.GroupInfoEditor = (*Client)(nil)

func (c *Client) canEditGroupInfo(info *types.GroupInfo) bool {
	// Community/announcement metadata has distinct rules and is not edited here.
	if info == nil || info.Suspended || info.IsParent || info.IsDefaultSubGroup || c.wa == nil || c.wa.Store == nil {
		return false
	}
	for _, p := range info.Participants {
		for _, jid := range []types.JID{p.JID, p.PhoneNumber, p.LID} {
			jid = jid.ToNonAD()
			if !jid.IsEmpty() && ((c.wa.Store.ID != nil && jid == c.wa.Store.ID.ToNonAD()) || jid == c.wa.Store.GetLID().ToNonAD()) {
				return !info.IsLocked || p.IsAdmin || p.IsSuperAdmin
			}
		}
	}
	return false
}

func (c *Client) SetGroupInfo(ctx context.Context, jid string, edit model.GroupInfoEdit) error {
	if err := edit.Validate(); err != nil {
		return err
	}
	info, err := c.readGroup(ctx, jid)
	if err != nil {
		return err
	}
	if !c.canEditGroupInfo(info) {
		return errors.New("you no longer have permission to edit this group's information")
	}
	current := info.Topic
	if edit.Field == "name" {
		current = info.Name
	}
	// Retry after a lost acknowledgment is harmless if the value already matches.
	if current == edit.Value {
		return nil
	}
	if current != *edit.Previous {
		return errors.New("group information changed while you were editing; copy your draft and reopen the editor")
	}
	if edit.Field == "name" {
		err = c.wa.SetGroupName(ctx, info.JID, edit.Value)
	} else {
		err = c.wa.SetGroupTopic(ctx, info.JID, info.TopicID, "", edit.Value)
	}
	if err != nil {
		return err
	}
	if edit.Field == "name" {
		_ = c.store.UpdateChatTitle(ctx, info.JID.String(), edit.Value)
		c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": info.JID.String(), "title": edit.Value}})
	}
	c.emit(gateway.Event{Name: "group.updated", Data: map[string]string{"jid": info.JID.String()}})
	return nil
}
