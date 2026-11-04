package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// socket is all the interface wants from the daemon: ask it something, and
// listen to what it announces. It is an interface rather than the connection
// itself so that a test can stand in for the daemon and read back what the
// keyboard asked for. These calls reach a real account - they mark messages
// read on somebody's phone - so the tests must not make them.
type socket interface {
	call(ctx context.Context, method string, params any, into any) error
	follow(ctx context.Context) <-chan rpc.Event
}

var errNoDaemon = errors.New("not connected to the daemon")

// noDaemon stands in when there is nothing on the other end, so a window built
// without a connection draws and answers the keyboard instead of falling over.
type noDaemon struct{}

func (noDaemon) call(context.Context, string, any, any) error { return errNoDaemon }

func (noDaemon) follow(context.Context) <-chan rpc.Event {
	stream := make(chan rpc.Event)
	close(stream)
	return stream
}

// eventData reads an announcement's payload into the shape this window expects
// of it. An event carries whatever the daemon put in it, so a payload that is
// not that shape is an announcement to ignore rather than a reason to stop.
func eventData(evt rpc.Event, into any) bool {
	raw, err := json.Marshal(evt.Data)
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, into) == nil
}
