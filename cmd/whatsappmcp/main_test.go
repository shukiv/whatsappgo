package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// A daemon that answers from a script, so a test can say what came back and
// see exactly what was asked for.
type fakeDaemon struct {
	discovery json.RawMessage
	answers   map[string]json.RawMessage
	failures  map[string]error
	// events is what a watching connection is given, in order.
	events []rpc.Event

	mu      sync.Mutex
	calls   []string
	params  map[string]any
	dials   int
	closed  int
	watches int
}

func (f *fakeDaemon) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls = append(f.calls, method)
	if typed, ok := params.(map[string]any); ok {
		f.params = typed
	}
	f.mu.Unlock()
	if err, failing := f.failures[method]; failing {
		return nil, err
	}
	if method == "rpc.discover" {
		return f.discovery, nil
	}
	if answer, known := f.answers[method]; known {
		return answer, nil
	}
	return json.RawMessage(`{}`), nil
}

// Watch hands over the scripted events, then blocks as the real one does
// until the connection is closed.
func (f *fakeDaemon) Watch(ctx context.Context, fn func(rpc.Event) error) error {
	f.mu.Lock()
	f.watches++
	events := f.events
	f.mu.Unlock()
	for _, evt := range events {
		if err := fn(evt); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeDaemon) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed++
	return nil
}

func (f *fakeDaemon) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeDaemon) connections() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dials, f.closed
}

func testServer(t *testing.T, daemon *fakeDaemon) *server {
	t.Helper()
	return newServer(func(context.Context) (caller, error) {
		daemon.mu.Lock()
		daemon.dials++
		// A connection is recorded beside the calls: the daemon subscribes one
		// when it accepts it, so where it was opened is where it started
		// receiving events.
		daemon.calls = append(daemon.calls, "connect")
		daemon.mu.Unlock()
		return daemon, nil
	})
}

func discovery() json.RawMessage {
	return json.RawMessage(`{"methods":[
		{"name":"chats.list","summary":"List conversations","mutating":false,
		 "params_example":{"limit":50,"query":"","archived":false}},
		{"name":"message.send","summary":"Send a text message","mutating":true,
		 "params_example":{"chat_jid":"1@s.whatsapp.net","text":"Hello","mentions":["1@s.whatsapp.net"]}},
		{"name":"status.get","summary":"Get connection state","mutating":false,"params_example":{}}
	]}`)
}

func request(t *testing.T, s *server, line string) map[string]any {
	t.Helper()
	reply := s.handle(context.Background(), []byte(line), time.Second)
	if reply == nil {
		t.Fatalf("a request went unanswered: %s", line)
	}
	encoded, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// Whatever the daemon serves is what the client is offered, without this
// command holding a list of its own.
func TestToolsAreBuiltFromWhatTheDaemonServes(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery()}
	s := testServer(t, daemon)

	reply := request(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	result, ok := reply["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %#v", reply)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != 3+len(builtinTools) {
		t.Fatalf("expected the three methods the daemon listed and the built-ins, got %#v", result["tools"])
	}
	first := named(tools, "chats_list")
	if first == nil {
		t.Fatalf("a method's dots did not become underscores: %#v", tools)
	}
	schema := first["inputSchema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if properties["limit"].(map[string]any)["type"] != "integer" ||
		properties["query"].(map[string]any)["type"] != "string" ||
		properties["archived"].(map[string]any)["type"] != "boolean" {
		t.Fatalf("the example did not describe the arguments: %#v", properties)
	}
}

// A tool that changes the account says so, so a reader of the list can tell
// what is safe to call.
func TestAMutatingMethodIsMarked(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery()}
	s := testServer(t, daemon)

	reply := request(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tools := reply["result"].(map[string]any)["tools"].([]any)
	for _, entry := range tools {
		described := entry.(map[string]any)
		description := described["description"].(string)
		if described["name"] == "message_send" {
			if !strings.HasPrefix(description, "[mutating] ") {
				t.Fatalf("sending a message is not marked as changing anything: %q", description)
			}
			if !strings.Contains(description, "message.send") {
				t.Fatalf("the description does not name the method it calls: %q", description)
			}
		}
		if described["name"] == "chats_list" && strings.Contains(description, "[mutating]") {
			t.Fatalf("reading conversations was marked as changing them: %q", description)
		}
	}
}

// A call reaches the method the tool stands for, carrying its arguments.
func TestACallReachesTheMethodWithItsArguments(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{"chats.list": json.RawMessage(`[{"jid":"1@lid"}]`)}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"chats_list","arguments":{"limit":5}}}`)
	result := reply["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("the call was reported as failed: %#v", result)
	}
	content := result["content"].([]any)[0].(map[string]any)
	if !strings.Contains(content["text"].(string), "1@lid") {
		t.Fatalf("the daemon's answer did not reach the caller: %#v", content)
	}
	if daemon.params["limit"] != float64(5) {
		t.Fatalf("the arguments did not reach the method: %#v", daemon.params)
	}
	calls := daemon.called()
	if calls[len(calls)-1] != "chats.list" {
		t.Fatalf("the wrong method was called: %#v", calls)
	}
}

// A refusal is an answer the caller can read and act on, not a transport
// failure that tells it nothing.
func TestARefusedCallIsReportedInsideTheResult(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		failures: map[string]error{"message.send": errors.New("chat_jid is required")}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"message_send","arguments":{}}}`)
	if _, failed := reply["error"]; failed {
		t.Fatalf("a refusal was reported as a protocol error: %#v", reply)
	}
	result := reply["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("a refusal was reported as success: %#v", result)
	}
	content := result["content"].([]any)[0].(map[string]any)
	if !strings.Contains(content["text"].(string), "chat_jid is required") {
		t.Fatalf("the caller cannot see why the call was refused: %#v", content)
	}
}

// A tool the daemon does not serve is refused by name.
func TestAnUnknownToolIsRefused(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery()}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"chat_explode","arguments":{}}}`)
	failure, failed := reply["error"].(map[string]any)
	if !failed || !strings.Contains(failure["message"].(string), "chat_explode") {
		t.Fatalf("an unknown tool was not refused: %#v", reply)
	}
}

// A daemon that was down when the client listed must not leave it without
// tools once the daemon is back.
func TestAFailedListingIsNotRemembered(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		failures: map[string]error{"rpc.discover": errors.New("connect: no such file or directory")}}
	s := testServer(t, daemon)

	reply := request(t, s, `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`)
	if _, failed := reply["error"]; !failed {
		t.Fatalf("a listing with no daemon reported success: %#v", reply)
	}
	delete(daemon.failures, "rpc.discover")
	reply = request(t, s, `{"jsonrpc":"2.0","id":6,"method":"tools/list"}`)
	tools, ok := reply["result"].(map[string]any)["tools"].([]any)
	if !ok || len(tools) != 3+len(builtinTools) {
		t.Fatalf("the tools did not appear once the daemon answered: %#v", reply)
	}
}

// The handshake says which protocol is spoken and that tools are served.
func TestInitializeAnnouncesTools(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery()}
	s := testServer(t, daemon)

	reply := request(t, s, `{"jsonrpc":"2.0","id":7,"method":"initialize","params":{}}`)
	result := reply["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("the wrong protocol revision was announced: %#v", result["protocolVersion"])
	}
	if _, offered := result["capabilities"].(map[string]any)["tools"]; !offered {
		t.Fatalf("tools were not announced: %#v", result["capabilities"])
	}
	if dials, _ := daemon.connections(); dials != 0 {
		t.Fatalf("the handshake required a running daemon")
	}
}

// A notification carries no id and must never be answered.
func TestANotificationIsNotAnswered(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery()}
	s := testServer(t, daemon)

	if reply := s.handle(context.Background(),
		[]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`), time.Second); reply != nil {
		t.Fatalf("a notification was answered: %#v", reply)
	}
}

// Every connection is closed after the call it was opened for.
func TestEachCallClosesItsConnection(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery()}
	s := testServer(t, daemon)

	request(t, s, `{"jsonrpc":"2.0","id":8,"method":"tools/list"}`)
	request(t, s,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"status_get","arguments":{}}}`)
	dials, closed := daemon.connections()
	if dials == 0 || dials != closed {
		t.Fatalf("connections were left open: %d dialled, %d closed", dials, closed)
	}
}

func named(tools []any, name string) map[string]any {
	for _, entry := range tools {
		described, ok := entry.(map[string]any)
		if ok && described["name"] == name {
			return described
		}
	}
	return nil
}

func contentOf(t *testing.T, result map[string]any, kind string) map[string]any {
	t.Helper()
	blocks, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("the result carried no content: %#v", result)
	}
	for _, block := range blocks {
		if typed, ok := block.(map[string]any); ok && typed["type"] == kind {
			return typed
		}
	}
	return nil
}

func pairingEvent(name string, data map[string]any) rpc.Event {
	return rpc.Event{Version: 1, Event: name, Data: data}
}

func qrEvent() rpc.Event {
	// A stub stands in for the picture the daemon draws; what matters here is
	// that it reaches the client unaltered.
	return pairingEvent("pairing.qr", map[string]any{
		"code":       "2@abcd,efgh,ijkl",
		"png_base64": "iVBORw0KGgo=",
		"expires_in": float64(20),
	})
}

func unlinked() json.RawMessage {
	return json.RawMessage(`{"state":"pairing","logged_in":false,"connected":false}`)
}

// The code to scan arrives as an event, not as an answer, so a client that can
// only call methods still has to be given one.
func TestPairingReturnsTheCodeThatArrivesAsAnEvent(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{"status.get": unlinked()},
		events:  []rpc.Event{pairingEvent("pairing.state", map[string]any{"state": "connecting"}), qrEvent()}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"pairing_qr","arguments":{}}}`)
	result := reply["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("pairing was reported as failed: %#v", result)
	}
	image := contentOf(t, result, "image")
	if image == nil || image["data"] != "iVBORw0KGgo=" || image["mimeType"] != "image/png" {
		t.Fatalf("the picture of the code did not reach the client: %#v", result["content"])
	}
	text := contentOf(t, result, "text")
	if text == nil || !strings.Contains(text["text"].(string), "2@abcd,efgh,ijkl") {
		t.Fatalf("the code itself did not reach the client: %#v", result["content"])
	}
	// A client that draws no pictures still has to be able to show the code.
	if !strings.Contains(text["text"].(string), "\u2588") {
		t.Fatalf("the code was not drawn as text: %q", text["text"])
	}
	if !strings.Contains(text["text"].(string), "20 seconds") {
		t.Fatalf("the client was not told the code expires: %q", text["text"])
	}
}

// Watching has to start before pairing does: a code emitted in between is a
// code nobody sees.
func TestPairingListensBeforeItStarts(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{
			"status.get":    unlinked(),
			"pairing.start": json.RawMessage(`{"ok":true}`)},
		events: []rpc.Event{qrEvent()}}
	s := testServer(t, daemon)

	request(t, s,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"pairing_qr","arguments":{}}}`)
	calls := daemon.called()
	started := -1
	for i, method := range calls {
		if method == "pairing.start" {
			started = i
		}
	}
	if started < 0 {
		t.Fatalf("pairing was never started: %#v", calls)
	}
	daemon.mu.Lock()
	watches := daemon.watches
	daemon.mu.Unlock()
	if watches == 0 {
		t.Fatalf("nothing watched for the code: %#v", calls)
	}
	asked := -1
	for i, method := range calls {
		if method == "status.get" {
			asked = i
		}
	}
	if asked < 0 || asked > started {
		t.Fatalf("pairing started without first asking whether the profile is already linked: %#v", calls)
	}
	// Two connections are opened between the two calls: the one that listens
	// for the code, and the one pairing.start itself travels on. One would
	// mean pairing started with nothing listening, and the first code lost.
	opened := 0
	for _, method := range calls[asked:started] {
		if method == "connect" {
			opened++
		}
	}
	if opened != 2 {
		t.Fatalf("pairing started before anything was listening, so its first code is lost: %#v", calls)
	}
}

// A profile that is already linked has nothing to pair, and unlinking it is
// not something a pairing tool should do by surprise.
func TestPairingRefusesAnAccountThatIsAlreadyLinked(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{
			"status.get": json.RawMessage(`{"state":"connected","logged_in":true,"connected":true,"user_jid":"1@s.whatsapp.net"}`)}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"pairing_qr","arguments":{}}}`)
	result := reply["result"].(map[string]any)
	text := contentOf(t, result, "text")
	if text == nil || !strings.Contains(text["text"].(string), "already linked") {
		t.Fatalf("the caller was not told the profile is linked: %#v", result)
	}
	for _, method := range daemon.called() {
		if method == "pairing.start" {
			t.Fatal("pairing was started on an account that is already linked")
		}
	}
}

// Pairing already running is the state this tool wants to read, not a failure.
func TestPairingInProgressStillReturnsTheNextCode(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers:  map[string]json.RawMessage{"status.get": unlinked()},
		failures: map[string]error{"pairing.start": errors.New("pairing is already in progress")},
		events:   []rpc.Event{qrEvent()}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"pairing_qr","arguments":{}}}`)
	result := reply["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("a code that was on its way was reported as a failure: %#v", result)
	}
	text := contentOf(t, result, "text")
	if text == nil || !strings.Contains(text["text"].(string), "2@abcd,efgh,ijkl") {
		t.Fatalf("the code did not reach the client: %#v", result)
	}
}

// Waiting ends on the outcome, and says which one it was.
func TestWaitingEndsWhenThePhoneAccepts(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{"status.get": unlinked()},
		events: []rpc.Event{
			pairingEvent("pairing.state", map[string]any{"state": "connecting"}),
			pairingEvent("pairing.success", nil)}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"pairing_wait","arguments":{}}}`)
	result := reply["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("a successful pairing was reported as failed: %#v", result)
	}
	text := contentOf(t, result, "text")
	if text == nil || !strings.HasPrefix(text["text"].(string), "paired") {
		t.Fatalf("the caller was not told the phone accepted: %#v", result)
	}
}

func TestWaitingReportsWhyPairingFailed(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{"status.get": unlinked()},
		events:  []rpc.Event{pairingEvent("pairing.error", map[string]any{"message": "the code timed out"})}}
	s := testServer(t, daemon)

	reply := request(t, s,
		`{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"pairing_wait","arguments":{}}}`)
	result := reply["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("a failed pairing was reported as success: %#v", result)
	}
	text := contentOf(t, result, "text")
	if text == nil || !strings.Contains(text["text"].(string), "the code timed out") {
		t.Fatalf("the caller cannot see why pairing failed: %#v", result)
	}
}

// A tool call that never returns is worse than one that says nothing happened.
func TestWaitingGivesUpAndSaysSo(t *testing.T) {
	daemon := &fakeDaemon{discovery: discovery(),
		answers: map[string]json.RawMessage{"status.get": unlinked()}}
	s := testServer(t, daemon)

	started := time.Now()
	reply := request(t, s,
		`{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"pairing_wait","arguments":{"timeout_seconds":1}}}`)
	if waited := time.Since(started); waited > 20*time.Second {
		t.Fatalf("the wait ignored the timeout it was given: %s", waited)
	}
	result := reply["result"].(map[string]any)
	text := contentOf(t, result, "text")
	if text == nil || !strings.Contains(text["text"].(string), "nothing happened") {
		t.Fatalf("the caller was not told that nothing happened: %#v", result)
	}
}

// What a client asks this server to block for is bounded, whatever it asks.
func TestAnUnboundedWaitIsBroughtBack(t *testing.T) {
	if waitFor(map[string]any{"timeout_seconds": float64(86400)}, time.Minute) != waitCeiling {
		t.Fatal("a client could ask this server to block for a day")
	}
	if waitFor(map[string]any{}, time.Minute) != time.Minute {
		t.Fatal("a call with no timeout did not use the default")
	}
	if waitFor(map[string]any{"timeout_seconds": float64(-5)}, time.Minute) != time.Minute {
		t.Fatal("a negative timeout did not use the default")
	}
}
