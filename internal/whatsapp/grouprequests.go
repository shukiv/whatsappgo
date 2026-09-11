package whatsapp

import (
	"context"
	"errors"
	"sort"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var _ gateway.GroupRequestManager = (*Client)(nil)

type groupRequestAPI interface {
	GetGroupRequestParticipants(context.Context, types.JID) ([]types.GroupParticipantRequest, error)
	UpdateGroupRequestParticipants(context.Context, types.JID, []types.JID, whatsmeow.ParticipantRequestChange) ([]types.GroupParticipant, error)
}

func (c *Client) readAdminGroup(ctx context.Context, jid string) (*types.GroupInfo, error) {
	info, err := c.readGroup(ctx, jid)
	if err != nil {
		return nil, err
	}
	if c.wa.Store == nil || !canEditGroupPermissions(info, c.wa.Store.GetJID(), c.wa.Store.GetLID()) {
		return nil, errors.New("only current admins of supported groups can manage join requests")
	}
	return info, nil
}

func (c *Client) GroupJoinRequests(ctx context.Context, jid string) ([]model.GroupJoinRequest, error) {
	info, err := c.readAdminGroup(ctx, jid)
	if err != nil {
		return nil, err
	}
	requests, err := c.wa.GetGroupRequestParticipants(ctx, info.JID)
	if err != nil {
		return nil, err
	}
	result := make([]model.GroupJoinRequest, 0, len(requests))
	for _, request := range requests {
		if request.JID.User == "" || (request.JID.Server != types.DefaultUserServer && request.JID.Server != types.HiddenUserServer) {
			continue
		}
		row := model.GroupJoinRequest{Member: c.describeGroupMember(ctx, types.GroupParticipant{JID: request.JID})}
		if !request.RequestedAt.IsZero() {
			row.RequestedAt = request.RequestedAt.UnixMilli()
		}
		result = append(result, row)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].RequestedAt == result[j].RequestedAt {
			return result[i].Member.JID < result[j].Member.JID
		}
		return result[i].RequestedAt < result[j].RequestedAt
	})
	return result, nil
}

// Verify fresh admin rights and the specific still-pending request, never a
// similarly named applicant or a later request by the same person.
func applyGroupJoinReview(ctx context.Context, api groupRequestAPI, info *types.GroupInfo, selfPN, selfLID types.JID, review model.GroupJoinReview) error {
	if err := review.Validate(); err != nil {
		return err
	}
	if !canEditGroupPermissions(info, selfPN, selfLID) {
		return errors.New("only current admins of supported groups can manage join requests")
	}
	participant, err := types.ParseJID(review.Participant)
	if err != nil || participant.User == "" || (participant.Server != types.DefaultUserServer && participant.Server != types.HiddenUserServer) {
		return errors.New("a WhatsApp user address is required")
	}
	participant = participant.ToNonAD()
	requests, err := api.GetGroupRequestParticipants(ctx, info.JID)
	if err != nil {
		return err
	}
	found := false
	for _, request := range requests {
		if request.JID.ToNonAD() == participant && !request.RequestedAt.IsZero() && request.RequestedAt.UnixMilli() == review.RequestedAt {
			found = true
			break
		}
	}
	if !found {
		return errors.New("this join request changed or is no longer pending; refresh requests before deciding")
	}
	result, err := api.UpdateGroupRequestParticipants(ctx, info.JID, []types.JID{participant}, whatsmeow.ParticipantRequestChange(review.Action))
	if err != nil {
		return err
	}
	if len(result) != 1 || result[0].Error != 0 {
		return errors.New("WhatsApp did not confirm the decision; refresh requests before trying again")
	}
	for _, identity := range []types.JID{result[0].JID, result[0].PhoneNumber, result[0].LID} {
		if !identity.IsEmpty() && identity.ToNonAD() == participant {
			return nil
		}
	}
	return errors.New("WhatsApp did not confirm this applicant; refresh requests before trying again")
}

func (c *Client) ReviewGroupJoinRequest(ctx context.Context, jid string, review model.GroupJoinReview) error {
	if err := review.Validate(); err != nil {
		return err
	}
	info, err := c.readAdminGroup(ctx, jid)
	if err != nil {
		return err
	}
	err = applyGroupJoinReview(ctx, c.wa, info, c.wa.Store.GetJID(), c.wa.Store.GetLID(), review)
	// Even a failed acknowledgement can conceal an applied server change.
	c.emit(gateway.Event{Name: "group.updated", Data: map[string]string{"jid": info.JID.String()}})
	return err
}
