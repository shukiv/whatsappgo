package model

import "errors"

// GroupJoinRequest is live admin-only data; it is not stored as chat history.
type GroupJoinRequest struct {
	Member      GroupMember `json:"member"`
	RequestedAt int64       `json:"requested_at"`
}

type GroupJoinReview struct {
	Participant string `json:"participant"`
	RequestedAt int64  `json:"requested_at"`
	Action      string `json:"action"`
}

func (r GroupJoinReview) Validate() error {
	if r.Action != "approve" && r.Action != "reject" {
		return errors.New("choose approve or reject")
	}
	if r.Participant == "" || r.RequestedAt <= 0 {
		return errors.New("a participant and original request time are required; refresh join requests")
	}
	return nil
}
