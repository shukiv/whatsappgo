package model

import "time"

type ConnectionStatus struct {
	State      string `json:"state"`
	LoggedIn   bool   `json:"logged_in"`
	Connected  bool   `json:"connected"`
	UserJID    string `json:"user_jid,omitempty"`
	UserName   string `json:"user_name,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	LastChange int64  `json:"last_change"`
}

type Chat struct {
	JID                string `json:"jid"`
	Title              string `json:"title"`
	AvatarPath         string `json:"avatar_path,omitempty"`
	LastMessageID      string `json:"last_message_id,omitempty"`
	LastMessageAt      int64  `json:"last_message_at,omitempty"`
	LastMessagePreview string `json:"last_message_preview,omitempty"`
	UnreadCount        int    `json:"unread_count"`
	MutedUntil         int64  `json:"muted_until,omitempty"`
	Pinned             bool   `json:"pinned"`
	Favorite           bool   `json:"favorite"`
	Archived           bool   `json:"archived"`
	IsGroup            bool   `json:"is_group"`
}

type Message struct {
	ID             string     `json:"id"`
	ChatJID        string     `json:"chat_jid"`
	SenderJID      string     `json:"sender_jid,omitempty"`
	SenderName     string     `json:"sender_name,omitempty"`
	Timestamp      int64      `json:"timestamp"`
	Kind           string     `json:"kind"`
	Body           string     `json:"body,omitempty"`
	FromMe         bool       `json:"from_me"`
	Status         string     `json:"status"`
	ReplyTo        string     `json:"reply_to,omitempty"`
	ReplyPreview   string     `json:"reply_preview,omitempty"`
	ReplySender    string     `json:"reply_sender,omitempty"`
	ReplyFromMe    bool       `json:"reply_from_me,omitempty"`
	Edited         bool       `json:"edited"`
	Revoked        bool       `json:"revoked"`
	MediaMIME      string     `json:"media_mime,omitempty"`
	MediaName      string     `json:"media_name,omitempty"`
	MediaPath      string     `json:"media_path,omitempty"`
	MediaThumbnail string     `json:"media_thumbnail,omitempty"`
	MediaSize      int64      `json:"media_size,omitempty"`
	Reactions      []Reaction `json:"reactions,omitempty"`

	// Voice notes carry the amplitude bars their sender recorded, plus the
	// length of the recording, so the bubble can draw a real waveform.
	MediaDuration int   `json:"media_duration,omitempty"`
	AudioWaveform []int `json:"audio_waveform,omitempty"`

	// A shared contact and a shared place, as the sender wrote them.
	ContactName  string  `json:"contact_name,omitempty"`
	ContactPhone string  `json:"contact_phone,omitempty"`
	ContactCount int     `json:"contact_count,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`

	// Link preview, as supplied by the sender inside the message. WhatsApp
	// resolves the page when the message is composed, so no page is fetched
	// here and no request leaks to the sites people link to.
	LinkURL         string `json:"link_url,omitempty"`
	LinkTitle       string `json:"link_title,omitempty"`
	LinkDescription string `json:"link_description,omitempty"`
	LinkThumbnail   string `json:"link_thumbnail,omitempty"`
}

type Reaction struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	SenderJID string `json:"sender_jid"`
	Emoji     string `json:"emoji"`
	Timestamp int64  `json:"timestamp"`
}

type MessagePage struct {
	Messages   []Message `json:"messages"`
	HasMore    bool      `json:"has_more"`
	NextBefore int64     `json:"next_before,omitempty"`
}

type SearchResult struct {
	Messages []Message `json:"messages"`
}

type Channel struct {
	JID             string `json:"jid"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	SubscriberCount int    `json:"subscriber_count,omitempty"`
	Verified        bool   `json:"verified"`
	Muted           bool   `json:"muted"`
}

type Community struct {
	JID              string `json:"jid"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	ParticipantCount int    `json:"participant_count,omitempty"`
}

type CallLog struct {
	ID             string `json:"id"`
	PeerJID        string `json:"peer_jid"`
	PeerName       string `json:"peer_name,omitempty"`
	PeerAvatarPath string `json:"peer_avatar_path,omitempty"`
	Timestamp      int64  `json:"timestamp"`
	Duration       int64  `json:"duration"`
	Incoming       bool   `json:"incoming"`
	Video          bool   `json:"video"`
	Result         string `json:"result"`
}

func NowMillis() int64 { return time.Now().UnixMilli() }
