package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// daemon is the terminal client's side of the socket the desktop client also
// talks to. Two connections, because Call skips events while it waits for its
// answer and Watch blocks reading the stream: one asks, one listens.
type daemon struct {
	calls  *rpc.Client
	events *rpc.Client
}

func dialDaemon(ctx context.Context, address string) (*daemon, error) {
	calls, err := rpc.Dial(ctx, address)
	if err != nil {
		return nil, err
	}
	events, err := rpc.Dial(ctx, address)
	if err != nil {
		calls.Close()
		return nil, err
	}
	return &daemon{calls: calls, events: events}, nil
}

func (d *daemon) Close() {
	if d == nil {
		return
	}
	if d.events != nil {
		d.events.Close()
	}
	if d.calls != nil {
		d.calls.Close()
	}
}

// call is the one way this program reaches WhatsApp. Every feature is a method
// on the daemon, so nothing here needs to know what a message is.
func (d *daemon) call(ctx context.Context, method string, params any, into any) error {
	if params == nil {
		params = map[string]any{}
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := d.calls.Call(callCtx, method, params)
	if err != nil {
		return err
	}
	if into == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, into)
}

// follow turns the daemon's event stream into a channel the interface can
// select on. The stream is the same one the desktop client follows, so both
// see a new message at the same moment.
func (d *daemon) follow(ctx context.Context) <-chan rpc.Event {
	stream := make(chan rpc.Event, 32)
	// Nothing to listen to is not a reason to fall over: a window built
	// without a daemon - a test, or a session whose socket has gone - reads a
	// stream that is simply already over.
	if d == nil || d.events == nil {
		close(stream)
		return stream
	}
	go func() {
		defer close(stream)
		_ = d.events.Watch(ctx, func(evt rpc.Event) error {
			select {
			case stream <- evt:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}()
	return stream
}
