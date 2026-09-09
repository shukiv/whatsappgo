package model

type OwnProfile struct {
	Name       string `json:"name"`
	About      string `json:"about"`
	AvatarPath string `json:"avatar_path"`
}

type StatusAudience struct {
	Type string   `json:"type"`
	JIDs []string `json:"jids"`
}
