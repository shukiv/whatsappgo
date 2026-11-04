package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// fakeDaemon stands in for the socket so a test can read back what the window
// asked the daemon to do. Several of these calls reach a real account - a read
// receipt appears on somebody else's phone - so asserting on what was sent is
// the only way to check the sequence without sending it.
type fakeDaemon struct {
	mutex    sync.Mutex
	asked    []askedCall
	answers  map[string][]any
	failures map[string][]error
	stream   chan rpc.Event
	hushed   sync.Once
}

type askedCall struct {
	method string
	params map[string]any
}

func newFakeDaemon() *fakeDaemon {
	return &fakeDaemon{answers: map[string][]any{}, failures: map[string][]error{},
		stream: make(chan rpc.Event, 8)}
}

// answer queues what the daemon says to one method. Several answers to the
// same method are served in turn, which is how a second page differs from the
// first.
func (f *fakeDaemon) answer(method string, results ...any) *fakeDaemon {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.answers[method] = append(f.answers[method], results...)
	return f
}

// refuse queues a refusal, one per call, so a method can be made to fail once
// and then work - which is what being asked to replace a file looks like.
func (f *fakeDaemon) refuse(method string, errs ...error) *fakeDaemon {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.failures[method] = append(f.failures[method], errs...)
	return f
}

func (f *fakeDaemon) call(_ context.Context, method string, params any, into any) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.asked = append(f.asked, askedCall{method: method, params: asMap(params)})
	if queued := f.failures[method]; len(queued) > 0 {
		f.failures[method] = queued[1:]
		return queued[0]
	}
	queued := f.answers[method]
	if len(queued) == 0 || into == nil {
		return nil
	}
	if len(queued) > 1 {
		f.answers[method] = queued[1:]
	}
	raw, err := json.Marshal(queued[0])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// follow hands over the queued announcements and then ends the stream, so a
// test never waits on a daemon that will not say anything else. A closed
// channel still serves what was put in it before it closed.
func (f *fakeDaemon) follow(context.Context) <-chan rpc.Event {
	f.hushed.Do(func() { close(f.stream) })
	return f.stream
}

// calls is every request the window has made, in order.
func (f *fakeDaemon) calls(method string) []askedCall {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	found := make([]askedCall, 0, 4)
	for _, call := range f.asked {
		if method == "" || call.method == method {
			found = append(found, call)
		}
	}
	return found
}

func (f *fakeDaemon) only(t *testing.T, method string) askedCall {
	t.Helper()
	found := f.calls(method)
	if len(found) != 1 {
		t.Fatalf("%s was asked for %d times, wanted once: %v", method, len(found), f.calls(""))
	}
	return found[0]
}

func (f *fakeDaemon) forget() {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.asked = nil
}

func asMap(params any) map[string]any {
	if params == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return map[string]any{}
	}
	var into map[string]any
	if json.Unmarshal(raw, &into) != nil {
		return map[string]any{}
	}
	return into
}

// pump drives the window the way the terminal does: every message is applied,
// and whatever that leaves to do is done, until there is nothing left. The
// real program does this in a loop of its own; a test needs it to see the
// sequence an action sets off rather than only its first step.
func pump(t *testing.T, current *app, messages ...tea.Msg) *app {
	t.Helper()
	steps := 0
	var run func(cmd tea.Cmd)
	apply := func(msg tea.Msg) {
		if msg == nil {
			return
		}
		// A test that wants to drive an action rather than a key hands the
		// work over directly, and it is run the way the program's own loop
		// would run it.
		if work, ok := msg.(doMsg); ok {
			run(work.cmd)
			return
		}
		updated, next := current.Update(msg)
		app, ok := updated.(*app)
		if !ok {
			t.Fatalf("update returned %T, not the app", updated)
		}
		current = app
		run(next)
	}
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		if steps++; steps > 200 {
			t.Fatal("the window kept giving itself work to do and never settled")
		}
		// Some of what the window gives itself to do is a wait: the pause
		// before typing is reported, the cursor's blink, the next
		// announcement. None of those belong to the keystroke being driven
		// here, so anything that does not answer at once is left alone. A
		// test that wants one drives its message itself.
		answered := make(chan tea.Msg, 1)
		go func() { answered <- cmd() }()
		var msg tea.Msg
		select {
		case msg = <-answered:
		case <-time.After(50 * time.Millisecond):
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, each := range batch {
				run(each)
			}
			return
		}
		apply(msg)
	}
	for _, msg := range messages {
		apply(msg)
	}
	return current
}

// driveFake is drive with a daemon behind it, for the tests that care what was
// asked of it.
func driveFake(t *testing.T, fake *fakeDaemon, messages ...tea.Msg) *app {
	t.Helper()
	current := newApp(context.Background(), fake, "default")
	current.width, current.height = 90, 24
	return pump(t, current, messages...)
}

// doMsg carries work for the pump to run, and never reaches Update.
type doMsg struct{ cmd tea.Cmd }

func do(cmd tea.Cmd) tea.Msg { return doMsg{cmd: cmd} }

func press(key string) tea.Msg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// What a request carried, read back the way JSON delivers it.
func (c askedCall) text(key string) string {
	value, _ := c.params[key].(string)
	return value
}

func (c askedCall) number(key string) int64 {
	value, _ := c.params[key].(float64)
	return int64(value)
}

func (c askedCall) texts(key string) []string {
	listed, _ := c.params[key].([]any)
	found := make([]string, 0, len(listed))
	for _, item := range listed {
		found = append(found, fmt.Sprint(item))
	}
	return found
}

// event is an announcement as the daemon delivers one, with its payload as
// JSON rather than as a Go value.
func event(name string, data map[string]any) rpc.Event {
	return rpc.Event{Version: 1, Event: name, Data: data}
}
