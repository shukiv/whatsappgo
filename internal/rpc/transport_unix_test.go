//go:build !windows

package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	appEvents "github.com/shukiv/whatsappgo/internal/events"
)

// A daemon whose socket file is unlinked keeps accepting on an inode nobody can
// name any more: it stays alive, holds its databases open, and is unreachable.
// Before this check every failed client connection left one more behind.
func TestServerStopsWhenItsSocketIsReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpc.sock")
	srv := NewServer(path, testHandler{}, appEvents.New())
	srv.socketCheckInterval = 20 * time.Millisecond
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ctx) }()

	// Let the watcher record the socket it bound before taking it away.
	time.Sleep(40 * time.Millisecond)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("Serve returned %v, want a clean stop", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the daemon kept serving a socket that no client can reach")
	}
}

func TestServerKeepsServingItsOwnSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpc.sock")
	srv := NewServer(path, testHandler{}, appEvents.New())
	srv.socketCheckInterval = 20 * time.Millisecond
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ctx) }()

	select {
	case <-served:
		t.Fatal("the daemon stopped while it still owned its socket")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestListenRefusesASecondDaemon(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rpc.sock")
	first := NewServer(path, testHandler{}, appEvents.New())
	if err := first.Listen(); err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second := NewServer(path, testHandler{}, appEvents.New())
	if err := second.Listen(); err == nil {
		second.Close()
		t.Fatal("a second daemon took over a socket that was still being served")
	}
}
