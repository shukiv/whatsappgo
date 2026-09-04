//go:build !windows

package config

import (
	"path/filepath"
	"testing"
)

// The daemon and the Qt client build this string independently. They have to
// agree exactly: a mismatch is a client that dials a socket nobody serves.
func TestSocketAddressNaming(t *testing.T) {
	runtimeDir := filepath.Join("/run", "user", "1000", "whatsappgo")
	if got, want := SocketAddress(runtimeDir, "default"), filepath.Join(runtimeDir, "whatsappd.sock"); got != want {
		t.Fatalf("default socket=%q want %q", got, want)
	}
	if got, want := SocketAddress(runtimeDir, "work"), filepath.Join(runtimeDir, "whatsappd-work.sock"); got != want {
		t.Fatalf("profile socket=%q want %q", got, want)
	}
}
