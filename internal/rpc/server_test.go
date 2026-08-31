package rpc

import (
	"bufio"
	"context"
	"encoding/json"
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
