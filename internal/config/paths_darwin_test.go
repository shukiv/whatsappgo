//go:build darwin

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// desktop/src/rpcclient.cpp names this same directory outright. The daemon and
// the client have to agree on it: if they drift apart the client dials a socket
// nobody is listening on, the daemon starts fine, and nothing reports a fault.
func TestSocketLivesInApplicationSupport(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}
	p, err := ResolveProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Library", "Application Support", "whatsappgo", "whatsappd-work.sock")
	if p.Socket != want {
		t.Fatalf("socket=%q want %q", p.Socket, want)
	}
	// macOS caps sun_path at 104 bytes, and bind fails with a bare EINVAL.
	if len(p.Socket) > 104 {
		t.Fatalf("socket path is %d bytes, over the 104-byte sun_path limit: %q", len(p.Socket), p.Socket)
	}
}
