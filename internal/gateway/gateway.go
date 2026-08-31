package gateway

import (
	"context"
	"io"

	"github.com/shuki/whatsappgo/internal/model"
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
}

type Gateway interface {
	Status() model.ConnectionStatus
	Connect(context.Context) error
	StartPairing(context.Context) error
	PairPhone(context.Context, string) (string, error)
	Disconnect()
	Logout(context.Context) error
	SendText(context.Context, string, string, string) (model.Message, error)
	SendMedia(context.Context, MediaRequest) (model.Message, error)
	DownloadMedia(context.Context, string, string) (model.Message, error)
	RequestHistory(context.Context, string, int) error
	SendReaction(context.Context, string, string, string, string) error
	EditText(context.Context, string, string, string) error
	DeleteMessage(context.Context, string, string, string) error
	ResolvePhone(context.Context, string) (model.Chat, error)
	FetchAvatar(context.Context, string) (string, error)
	MarkRead(context.Context, string, string, []string, int64) error
	SetTyping(context.Context, string, bool) error
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
func (Unavailable) SendText(context.Context, string, string, string) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) SendMedia(context.Context, MediaRequest) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) DownloadMedia(context.Context, string, string) (model.Message, error) {
	return model.Message{}, ErrUnavailable
}
func (Unavailable) RequestHistory(context.Context, string, int) error { return ErrUnavailable }
func (Unavailable) SendReaction(context.Context, string, string, string, string) error {
	return ErrUnavailable
}
func (Unavailable) EditText(context.Context, string, string, string) error { return ErrUnavailable }
func (Unavailable) DeleteMessage(context.Context, string, string, string) error {
	return ErrUnavailable
}
func (Unavailable) ResolvePhone(context.Context, string) (model.Chat, error) {
	return model.Chat{}, ErrUnavailable
}
func (Unavailable) FetchAvatar(context.Context, string) (string, error) { return "", ErrUnavailable }
func (Unavailable) MarkRead(context.Context, string, string, []string, int64) error {
	return ErrUnavailable
}
func (Unavailable) SetTyping(context.Context, string, bool) error         { return ErrUnavailable }
func (Unavailable) ListChannels(context.Context) ([]model.Channel, error) { return nil, ErrUnavailable }
func (Unavailable) ListCommunities(context.Context) ([]model.Community, error) {
	return nil, ErrUnavailable
}
func (Unavailable) Subscribe(func(Event)) func() { return func() {} }
func (Unavailable) Close() error                 { return nil }

var ErrUnavailable = io.ErrClosedPipe
