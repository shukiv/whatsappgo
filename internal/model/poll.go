package model

// PollVote preserves the latest update per voter, including an encrypted
// envelope when the creation secret has not arrived yet.
type PollVote struct {
	ChatJID        string
	PollID         string
	Sender         string
	Timestamp      int64
	Options        []string
	Pending        bool
	Raw            []byte
	OriginalChat   string
	OriginalSender string
	UpdateID       string
	FromMe         bool
}
