package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	appEvents "github.com/shukiv/whatsappgo/internal/events"
)

type testHandler struct{}

func (testHandler) Handle(_ context.Context, method string, _ json.RawMessage) (any, error) {
	return map[string]string{"method": method}, nil
}

func TestServerRequestResponseAndEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpc.sock")
	broker := appEvents.New()
	srv := NewServer(path, testHandler{}, broker)
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(bufio.NewReader(conn))
	if err := enc.Encode(Request{Version: 1, ID: "1", Method: "status.get"}); err != nil {
		t.Fatal(err)
	}
	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil || resp.ID != "1" {
		t.Fatalf("bad response: %#v", resp)
	}
	time.Sleep(10 * time.Millisecond)
	broker.Publish(appEvents.Event{Name: "connection.changed", Data: map[string]bool{"connected": true}})
	var evt Event
	if err := dec.Decode(&evt); err != nil {
		t.Fatal(err)
	}
	if evt.Event != "connection.changed" {
		t.Fatalf("bad event: %#v", evt)
	}
}

func TestClientCallSkipsInterleavedEventAndReturnsRemoteError(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	client := NewClient(clientConn)
	defer client.Close()
	go func() {
		defer serverConn.Close()
		dec := json.NewDecoder(serverConn)
		enc := json.NewEncoder(serverConn)
		var req Request
		_ = dec.Decode(&req)
		_ = enc.Encode(Event{Version: ProtocolVersion, Event: "message.upsert"})
		_ = enc.Encode(Response{Version: ProtocolVersion, ID: req.ID, Result: map[string]bool{"ok": true}})
		_ = dec.Decode(&req)
		_ = enc.Encode(Response{Version: ProtocolVersion, ID: req.ID, Error: &Error{Code: "denied", Message: "no"}})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := client.Call(ctx, "status.get", map[string]any{})
	if err != nil || string(result) != `{"ok":true}` {
		t.Fatalf("unexpected call result=%s err=%v", result, err)
	}
	_, err = client.Call(ctx, "account.logout", map[string]any{})
	var remote *RemoteError
	if !errors.As(err, &remote) || remote.Code != "denied" {
		t.Fatalf("unexpected remote error: %v", err)
	}
}

func TestClientWatchStreamsEvents(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	client := NewClient(clientConn)
	defer client.Close()
	go func() {
		defer serverConn.Close()
		enc := json.NewEncoder(serverConn)
		_ = enc.Encode(Event{Version: ProtocolVersion, Event: "chat.updated", Data: map[string]string{"jid": "a@lid"}})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var name string
	err := client.Watch(ctx, func(evt Event) error {
		name = evt.Event
		return io.EOF
	})
	if !errors.Is(err, io.EOF) || name != "chat.updated" {
		t.Fatalf("watch name=%q err=%v", name, err)
	}
}
