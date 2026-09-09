package model

// GroupInfo is live membership metadata, kept separate from cheap local chat.info.
type GroupInfo struct {
	JID              string        `json:"jid"`
	Name             string        `json:"name"`
	Description      string        `json:"description"`
	CreatedAt        int64         `json:"created_at"`
	Creator          GroupMember   `json:"creator"`
	Participants     []GroupMember `json:"participants"`
	ParticipantCount int           `json:"participant_count"`
	IsMember         bool          `json:"is_member"`
	CanAdd           bool          `json:"can_add"`
	CanInvite        bool          `json:"can_invite"`
	CanManage        bool          `json:"can_manage"`
}

type GroupMember struct {
	JID        string   `json:"jid"`
	Aliases    []string `json:"aliases"`
	Name       string   `json:"name"`
	Phone      string   `json:"phone,omitempty"`
	AvatarPath string   `json:"avatar_path,omitempty"`
	IsSelf     bool     `json:"is_self"`
	IsAdmin    bool     `json:"is_admin"`
	IsOwner    bool     `json:"is_owner"`
}
