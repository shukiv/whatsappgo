package gateway

import (
	"context"
	"io"
	"time"

	"github.com/shukiv/whatsappgo/internal/model"
)

type Event struct {
	Name string `json:"name"`
	Data any    `json:"data,omitempty"`
}

type MediaRequest struct {
	ChatJID string
	Path    string
	Caption string
	ReplyTo string
	Voice   bool
	// ForwardingScore marks the attachment as forwarded and says how long the
	// chain is. Zero means an ordinary send.
	ForwardingScore int
}

type TextRequest struct {
	ChatJID      string
	Text         string
	ReplyTo      string
	ReplyChatJID string
	Preview      model.LinkPreview
}

type Gateway interface {
	Status() model.ConnectionStatus
	Connect(context.Context) error
	StartPairing(context.Context) error
	PairPhone(context.Context, string) (string, error)
	Disconnect()
	Logout(context.Context) error
	SendText(context.Context, TextRequest) (model.Message, error)
	SendMedia(context.Context, MediaRequest) (model.Message, error)
	DownloadMedia(context.Context, string, string) (model.Message, error)
	RefreshLinkPreview(context.Context, string, string) (model.Message, error)
	RequestHistory(context.Context, string, int) error
	RefreshHistory(context.Context, string, int) error
	SendReaction(context.Context, string, string, string, string) error
	PinMessage(context.Context, string, string, string, time.Duration) error
	UnpinMessage(context.Context, string, string, string) error
	EditText(context.Context, string, string, string) error
	DeleteMessage(context.Context, string, string, string) error
	ResolvePhone(context.Context, string) (model.Chat, error)
	FetchAvatar(context.Context, string) (string, error)
	RefreshAvatar(context.Context, string) (string, error)
	MarkRead(context.Context, string, string, []string, int64) error
	MarkPlayed(context.Context, string, string, string, int64) error
	SubscribePresence(context.Context, string) error
	SetTyping(context.Context, string, bool) error
	SetChatPinned(context.Context, string, bool) error
	SetChatMuted(context.Context, string, bool, time.Duration) error
	DeleteChat(context.Context, string) error
	ClearChat(context.Context, string) error
	SetChatDisappearing(context.Context, string, int64) error
	ExportChat(context.Context, string, string) (string, error)
	PrivacySettings(context.Context) (model.PrivacySettings, error)
	SetPrivacySetting(context.Context, string, string) (model.PrivacySettings, error)
	SetAbout(context.Context, string) error
	PostTextStatus(context.Context, string, int) (model.Message, error)
	PostMediaStatus(context.Context, string, string) (model.Message, error)
	SetChannelFollowed(context.Context, string, bool) error
	SetChannelMuted(context.Context, string, bool) error
	CreateChannel(context.Context, string, string) (model.Channel, error)
	FollowChannelLink(context.Context, string) (model.Channel, error)
	CreateCommunity(context.Context, string) (model.Community, error)
	JoinGroupLink(context.Context, string) (model.Chat, error)
	SetChatFavorite(context.Context, string, bool) error
	SetChatArchived(context.Context, string, bool) error
	SetChatRead(context.Context, string, bool) error
	MarkAllChatsRead(context.Context) (int, error)
	CreateGroup(context.Context, string, []string) (model.Chat, error)
	SetContactBlocked(context.Context, string, bool) error
	CreateLabel(context.Context, string, int) (model.Label, error)
	SetChatLabeled(context.Context, string, string, bool) error
	BlockedContacts(context.Context) ([]string, error)
	SetMessageStarred(context.Context, string, string, string, bool, bool) error
	ForwardMessage(context.Context, string, string, string) (model.Message, error)
	ListChannels(context.Context) ([]model.Channel, error)
	ListCommunities(context.Context) ([]model.Community, error)
	Subscribe(func(Event)) (unsubscribe func())
	Close() error
}

type Unavailable struct{}

func (Unavailable) Status() model.ConnectionStatus {
	return model.ConnectionStatus{State: "unavailable"}
}
func (Unavailable) Connect(context.Context) error                     { return ErrUnavailable }
func (Unavailable) StartPairing(context.Context) error                { return ErrUnavailable }
func (Unavailable) PairPhone(context.Context, string) (string, error) { return "", ErrUnavailable }
func (Unavailable) Disconnect()                                       {}
func (Unavailable) Logout(context.Context) error                      { return ErrUnavailable }
func (Unavailable) SendText(context.Context, TextRequest) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) SendMedia(context.Context, MediaRequest) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) DownloadMedia(context.Context, string, string) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) RefreshLinkPreview(context.Context, string, string) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) RequestHistory(context.Context, string, int) error { return ErrUnavailable }
func (Unavailable) RefreshHistory(context.Context, string, int) error { return ErrUnavailable }
func (Unavailable) SendReaction(context.Context, string, string, string, string) error {
	return ErrUnavailable
}
func (Unavailable) PinMessage(context.Context, string, string, string, time.Duration) error {
	return ErrUnavailable
}
func (Unavailable) UnpinMessage(context.Context, string, string, string) error { return ErrUnavailable }
func (Unavailable) EditText(context.Context, string, string, string) error     { return ErrUnavailable }
func (Unavailable) DeleteMessage(context.Context, string, string, string) error {
	return ErrUnavailable
}
func (Unavailable) ResolvePhone(context.Context, string) (model.Chat, error) {
	return model.Chat{}, ErrUnavailable
}
func (Unavailable) FetchAvatar(context.Context, string) (string, error) { return "", ErrUnavailable }
func (Unavailable) RefreshAvatar(context.Context, string) (string, error) {
	return "", ErrUnavailable
}
func (Unavailable) MarkRead(context.Context, string, string, []string, int64) error {
	return ErrUnavailable
}
func (Unavailable) MarkPlayed(context.Context, string, string, string, int64) error {
	return ErrUnavailable
}
func (Unavailable) SubscribePresence(context.Context, string) error   { return ErrUnavailable }
func (Unavailable) SetTyping(context.Context, string, bool) error     { return ErrUnavailable }
func (Unavailable) SetChatPinned(context.Context, string, bool) error { return ErrUnavailable }
func (Unavailable) SetMessageStarred(context.Context, string, string, string, bool, bool) error {
	return ErrUnavailable
}
func (Unavailable) ForwardMessage(context.Context, string, string, string) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) SetChatMuted(context.Context, string, bool, time.Duration) error {
	return ErrUnavailable
}
func (Unavailable) DeleteChat(context.Context, string) error { return ErrUnavailable }

func (Unavailable) ClearChat(context.Context, string) error { return ErrUnavailable }

func (Unavailable) SetChatDisappearing(context.Context, string, int64) error { return ErrUnavailable }

func (Unavailable) ExportChat(context.Context, string, string) (string, error) {
	return "", ErrUnavailable
}

func (Unavailable) PrivacySettings(context.Context) (model.PrivacySettings, error) {
	return model.PrivacySettings{}, ErrUnavailable
}

func (Unavailable) SetPrivacySetting(context.Context, string, string) (model.PrivacySettings, error) {
	return model.PrivacySettings{}, ErrUnavailable
}

func (Unavailable) SetAbout(context.Context, string) error { return ErrUnavailable }

func (Unavailable) PostTextStatus(context.Context, string, int) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}

func (Unavailable) PostMediaStatus(context.Context, string, string) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}

func (Unavailable) SetChannelFollowed(context.Context, string, bool) error { return ErrUnavailable }

func (Unavailable) SetChannelMuted(context.Context, string, bool) error { return ErrUnavailable }

func (Unavailable) CreateChannel(context.Context, string, string) (model.Channel, error) {
	return model.Channel{}, ErrUnavailable
}

func (Unavailable) FollowChannelLink(context.Context, string) (model.Channel, error) {
	return model.Channel{}, ErrUnavailable
}

func (Unavailable) CreateCommunity(context.Context, string) (model.Community, error) {
	return model.Community{}, ErrUnavailable
}

func (Unavailable) JoinGroupLink(context.Context, string) (model.Chat, error) {
	return model.Chat{}, ErrUnavailable
}

func (Unavailable) SetChatFavorite(context.Context, string, bool) error { return ErrUnavailable }
func (Unavailable) SetChatArchived(context.Context, string, bool) error { return ErrUnavailable }
func (Unavailable) SetChatRead(context.Context, string, bool) error     { return ErrUnavailable }
func (Unavailable) MarkAllChatsRead(context.Context) (int, error)       { return 0, ErrUnavailable }
func (Unavailable) CreateGroup(context.Context, string, []string) (model.Chat, error) {
	return model.Chat{}, ErrUnavailable
}
func (Unavailable) SetContactBlocked(context.Context, string, bool) error { return ErrUnavailable }
func (Unavailable) CreateLabel(context.Context, string, int) (model.Label, error) {
	return model.Label{}, ErrUnavailable
}
func (Unavailable) SetChatLabeled(context.Context, string, string, bool) error {
	return ErrUnavailable
}
func (Unavailable) BlockedContacts(context.Context) ([]string, error)     { return nil, ErrUnavailable }
func (Unavailable) ListChannels(context.Context) ([]model.Channel, error) { return nil, ErrUnavailable }
func (Unavailable) ListCommunities(context.Context) ([]model.Community, error) {
	return nil, ErrUnavailable
}
func (Unavailable) Subscribe(func(Event)) func() { return func() {} }
func (Unavailable) Close() error                 { return nil }

var ErrUnavailable = io.ErrClosedPipe
