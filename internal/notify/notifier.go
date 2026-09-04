package notify

import "context"

// Message is everything the desktop needs to present one incoming message.
// Keeping the avatar with the text prevents notification integrations from
// silently losing sender identity as their platform payloads evolve.
type Message struct {
	ChatJID  string
	Title    string
	Body     string
	IconPath string
}

type Notifier interface {
	Notify(context.Context, Message) error
	// Presents reports whether this notifier puts the message on the user's
	// screen itself. When it does not, the daemon says so in the
	// notification.received event and the desktop client presents the message
	// with the platform's own API. Without the distinction the user would see
	// every notification twice on Linux, or none at all elsewhere.
	Presents() bool
	Close() error
}

// Noop is the notifier the daemon runs with when no backend is available.
type Noop struct{}

func (Noop) Notify(context.Context, Message) error { return nil }
func (Noop) Presents() bool                        { return false }
func (Noop) Close() error                          { return nil }
