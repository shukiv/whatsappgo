package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// daemon is the terminal client's side of the socket the desktop client also
// talks to. Two connections, because Call skips events while it waits for its
// answer and Watch blocks reading the stream: one asks, one listens.
type daemon struct {
	calls  *rpc.Client
	events *rpc.Client

	// Held for the whole of one request, because the connection carries one
	// at a time. See call.
	asking sync.Mutex
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
//
// One request at a time, which the lock is for. The connection carries one:
// rpc.Client writes through a single encoder, reads through a single decoder,
// and its reader loop throws away any answer whose id is not the one it is
// waiting for. Two requests at once over it therefore both corrupt the reader
// and steal each other's answers. This interface asks for several things at
// once as a matter of course - the account and the conversation list when it
// starts, a page and a receipt when a conversation opens - and each of those
// runs on a goroutine of its own, so without this they raced on the first
// screen the program ever drew.
func (d *daemon) call(ctx context.Context, method string, params any, into any) error {
	if d == nil || d.calls == nil {
		return errNoDaemon
	}
	if params == nil {
		params = map[string]any{}
	}
	d.asking.Lock()
	defer d.asking.Unlock()
	// The wait for the connection is not part of the time the daemon is given
	// to answer, so a queued request still gets its full budget.
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
