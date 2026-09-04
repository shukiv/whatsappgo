//go:build linux

package config

import "testing"

// XDG_RUNTIME_DIR is the only directory a Linux session guarantees to be
// per-user, mode 0700 and cleared at logout, and the socket's access control
// rests on all three. Falling back to somewhere world-traversable would widen
// who can reach the daemon, so the daemon refuses to start instead.
func TestLinuxRequiresARuntimeDirectory(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	if _, err := Resolve(); err == nil {
		t.Fatal("resolved paths without a per-user runtime directory")
	}
}
