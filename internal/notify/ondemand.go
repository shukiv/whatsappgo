package notify

import (
	"context"
	"errors"
	"sync"
)

// OnDemand presents notifications only while a desktop client holds it.
//
// A daemon started without notifications - by the MCP server, or as a
// systemd service - is still the one the desktop window attaches to when it
// finds it already listening, and on Linux the window leaves notifications to
// the daemon. Nothing reached the screen. The window now holds the daemon's
// notifications for as long as it stays connected, and this notifier presents
// through the real one only for that long. The real one is opened on the first
// hold, so a daemon no window attaches to never touches the session bus.
type OnDemand struct {
	open func() (Notifier, error)

	// opening serialises opens without holding mu, which Presents and Notify
	// take for every incoming message: opening can take seconds when it has
	// to start a notification service.
	opening sync.Mutex

	mu      sync.Mutex
	current Notifier
	holders int
}

// NewOnDemand returns a notifier that opens the real one with open when it is
// first held. open must return a nil interface with its error, never a nil
// pointer wrapped in one.
func NewOnDemand(open func() (Notifier, error)) *OnDemand {
	return &OnDemand{open: open}
}

// Hold presents notifications until ctx is done. It returns at once and lets
// go in the background, so a caller can hand it the context of a connection
// and have the hold end when the connection does. A notifier that cannot be
// opened is an error and holds nothing; the client then presents what the
// daemon leaves to it, as it would have without asking.
func (o *OnDemand) Hold(ctx context.Context) error {
	if _, err := o.presenter(); err != nil {
		return err
	}
	o.mu.Lock()
	o.holders++
	o.mu.Unlock()
	go func() {
		<-ctx.Done()
		o.mu.Lock()
		o.holders--
		o.mu.Unlock()
	}()
	return nil
}

func (o *OnDemand) presenter() (Notifier, error) {
	o.opening.Lock()
	defer o.opening.Unlock()
	o.mu.Lock()
	current := o.current
	o.mu.Unlock()
	if current != nil {
		return current, nil
	}
	opened, err := o.open()
	if err != nil {
		return nil, err
	}
	if opened == nil {
		return nil, errors.New("no desktop notifier")
	}
	o.mu.Lock()
	o.current = opened
	o.mu.Unlock()
	return opened, nil
}

// held is the notifier to present through, or nil while nobody holds one.
func (o *OnDemand) held() Notifier {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.holders == 0 {
		return nil
	}
	return o.current
}

// Presents is true only while a client holds the notifier. The event that says
// so is sent before the notification is posted, so one that arrives just as a
// window connects or leaves can be shown by both or by neither; the window is
// at that moment either not yet listening or already gone.
func (o *OnDemand) Presents() bool {
	current := o.held()
	return current != nil && current.Presents()
}

func (o *OnDemand) Notify(ctx context.Context, msg Message) error {
	current := o.held()
	if current == nil {
		return nil
	}
	return current.Notify(ctx, msg)
}

func (o *OnDemand) Close() error {
	o.opening.Lock()
	defer o.opening.Unlock()
	o.mu.Lock()
	current := o.current
	o.current = nil
	o.mu.Unlock()
	if current == nil {
		return nil
	}
	return current.Close()
}
