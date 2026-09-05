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

// PrivacySettings mirrors the account's WhatsApp privacy choices. The values
// are WhatsApp's own vocabulary ("all", "contacts", "none", ...) rather than a
// local invention, so a setting written here means the same thing on the phone.
type PrivacySettings struct {
	LastSeen     string `json:"last_seen"`
	Online       string `json:"online"`
	ProfilePhoto string `json:"profile_photo"`
	About        string `json:"about"`
	Status       string `json:"status"`
	ReadReceipts string `json:"read_receipts"`
	GroupAdd     string `json:"group_add"`
	CallAdd      string `json:"call_add"`
}

type Chat struct {
	JID                string `json:"jid"`
	Title              string `json:"title"`
	AvatarPath         string `json:"avatar_path,omitempty"`
	LastMessageID      string `json:"last_message_id,omitempty"`
	LastMessageAt      int64  `json:"last_message_at,omitempty"`
	LastMessagePreview string `json:"last_message_preview,omitempty"`
	// The chat list draws the last message the way WhatsApp Web does: a type
	// icon, the sender's receipt state, and a voice note's length. Those need
	// the message's own fields, not just the rendered preview text.
	LastMessageKind     string `json:"last_message_kind,omitempty"`
	LastMessageFromMe   bool   `json:"last_message_from_me"`
	LastMessageStatus   string `json:"last_message_status,omitempty"`
	LastMessageDuration int    `json:"last_message_duration,omitempty"`
	// The lists this chat belongs to, so the sidebar can filter by one
	// without a round trip per chat.
	LabelIDs            []string `json:"label_ids,omitempty"`
	UnreadCount         int      `json:"unread_count"`
	MutedUntil          int64    `json:"muted_until,omitempty"`
	Pinned              bool     `json:"pinned"`
	PinnedAt            int64    `json:"pinned_at,omitempty"`
	Favorite            bool     `json:"favorite"`
	DisappearingSeconds int64    `json:"disappearing_seconds"`
	Archived            bool     `json:"archived"`
	IsGroup             bool     `json:"is_group"`
}

// Label is one of WhatsApp's chat lists. WhatsApp Web calls them lists in the
// interface and labels on the wire.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color int    `json:"color"`
}

type Message struct {
	ID              string     `json:"id"`
	ChatJID         string     `json:"chat_jid"`
	SenderJID       string     `json:"sender_jid,omitempty"`
	SenderName      string     `json:"sender_name,omitempty"`
	Timestamp       int64      `json:"timestamp"`
	Kind            string     `json:"kind"`
	Body            string     `json:"body,omitempty"`
	FromMe          bool       `json:"from_me"`
	Status          string     `json:"status"`
	DeliveredAt     int64      `json:"delivered_at,omitempty"`
	ReadAt          int64      `json:"read_at,omitempty"`
	PlayedAt        int64      `json:"played_at,omitempty"`
	Starred         bool       `json:"starred,omitempty"`
	ChatTitle       string     `json:"chat_title,omitempty"`
	ForwardingScore int        `json:"forwarding_score,omitempty"`
	ReplyTo         string     `json:"reply_to,omitempty"`
	ReplyPreview    string     `json:"reply_preview,omitempty"`
	ReplySender     string     `json:"reply_sender,omitempty"`
	ReplyFromMe     bool       `json:"reply_from_me,omitempty"`
	Edited          bool       `json:"edited"`
	Revoked         bool       `json:"revoked"`
	MediaMIME       string     `json:"media_mime,omitempty"`
	MediaName       string     `json:"media_name,omitempty"`
	MediaPath       string     `json:"media_path,omitempty"`
	MediaThumbnail  string     `json:"media_thumbnail,omitempty"`
	MediaSize       int64      `json:"media_size,omitempty"`
	Reactions       []Reaction `json:"reactions,omitempty"`

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

// LinkPreview is resolved only while composing a message. Thumbnail uses the
// JSON package's base64 encoding so the local RPC can carry it without a
// temporary public file or a second HTTP request from QML.
type LinkPreview struct {
	URL           string `json:"url,omitempty"`
	Title         string `json:"title,omitempty"`
	Description   string `json:"description,omitempty"`
	Thumbnail     []byte `json:"thumbnail,omitempty"`
	ThumbnailMIME string `json:"thumbnail_mime,omitempty"`
}

type Reaction struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	SenderJID string `json:"sender_jid"`
	Emoji     string `json:"emoji"`
	Timestamp int64  `json:"timestamp"`
	// Who left it, for the panel that lists them. Filled in when a stored
	// message is read; a reaction arriving as an event carries only the JID,
	// which the window already knows how to name.
	SenderName       string `json:"sender_name,omitempty"`
	SenderAvatarPath string `json:"sender_avatar_path,omitempty"`
}

type MessagePage struct {
	Messages   []Message `json:"messages"`
	HasMore    bool      `json:"has_more"`
	NextBefore int64     `json:"next_before,omitempty"`
	// Id of the oldest message in this page. Paging uses it together with
	// NextBefore so messages sharing a timestamp are not skipped.
	NextBeforeID string `json:"next_before_id,omitempty"`
}

// StatusGroup is one contact's active story. Items are ordered from oldest to
// newest so a viewer can play them in the same order they were posted.
type StatusGroup struct {
	SenderJID  string    `json:"sender_jid"`
	SenderName string    `json:"sender_name"`
	AvatarPath string    `json:"avatar_path,omitempty"`
	LatestAt   int64     `json:"latest_at"`
	Items      []Message `json:"items"`
}

// ChatInfo is the local information WhatsAppGo can show for a conversation.
// Shared content is paged separately so opening the drawer stays inexpensive
// even for chats with years of history.
type ChatInfo struct {
	Chat          Chat      `json:"chat"`
	Phone         string    `json:"phone,omitempty"`
	LastSeen      int64     `json:"last_seen,omitempty"`
	PinnedMessage *Message  `json:"pinned_message,omitempty"`
	PinnedUntil   int64     `json:"pinned_until,omitempty"`
	SharedCount   int       `json:"shared_count"`
	MediaCount    int       `json:"media_count"`
	DocumentCount int       `json:"document_count"`
	LinkCount     int       `json:"link_count"`
	Preview       []Message `json:"preview"`
}

type SharedMessagePage struct {
	Messages []Message `json:"messages"`
	HasMore  bool      `json:"has_more"`
	Offset   int       `json:"offset"`
}

type SearchResult struct {
	Messages []Message `json:"messages"`
}

// Contact is an address-book entry. The sidebar search lists people who have
// no conversation yet, which the chat list alone cannot supply.
type Contact struct {
	JID        string `json:"jid"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	AvatarPath string `json:"avatar_path,omitempty"`
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
