//go:build !linux

package notify

import (
	"context"
	"errors"
)

// Desktop is the freedesktop backend, which talks to org.freedesktop.Notifications
// over D-Bus. That service exists on Linux and nowhere else, so this build
// declines to construct one and cmd/whatsappd falls back to the broadcast
// notifier, which hands the message to the desktop client to display natively.
type Desktop struct{}

func NewDesktop(string) (*Desktop, error) {
	return nil, errors.New("no freedesktop notification service on this platform")
}

func (*Desktop) Notify(context.Context, Message) error { return nil }
func (*Desktop) Presents() bool                        { return false }
func (*Desktop) Close() error                          { return nil }
