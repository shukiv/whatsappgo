package config

import (
	"path/filepath"
	"testing"
)

func TestResolveUsesXDGDirectories(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/wago-data")
	t.Setenv("XDG_CACHE_HOME", "/tmp/wago-cache")
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/wago-runtime")

	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.Socket, filepath.Join("/tmp/wago-runtime", "whatsappgo", "whatsappd.sock"); got != want {
		t.Fatalf("socket = %q, want %q", got, want)
	}
	if got, want := p.MessageDB, filepath.Join("/tmp/wago-data", "whatsappgo", "messages.db"); got != want {
		t.Fatalf("message db = %q, want %q", got, want)
	}
}

func TestResolveProfileIsolatesDataAndSocket(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/wago-data")
	t.Setenv("XDG_CACHE_HOME", "/tmp/wago-cache")
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/wago-runtime")
	p, err := ResolveProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.DeviceDB, filepath.Join("/tmp/wago-data", "whatsappgo", "profiles", "work", "device.db"); got != want {
		t.Fatalf("device db=%q want %q", got, want)
	}
	if got, want := p.Socket, filepath.Join("/tmp/wago-runtime", "whatsappgo", "whatsappd-work.sock"); got != want {
		t.Fatalf("socket=%q want %q", got, want)
	}
}

func TestResolveProfileRejectsUnsafeName(t *testing.T) {
	if _, err := ResolveProfile("../escape"); err == nil {
		t.Fatal("expected unsafe profile to be rejected")
	}
}

func TestResolveProfileSeparatesAttachmentDatabase(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/wago-data")
	t.Setenv("XDG_CACHE_HOME", "/tmp/wago-cache")
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/wago-runtime")
	p, err := ResolveProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	// Attachments are large; keeping them out of messages.db protects the
	// message index that the user guide tells people to back up.
	if got, want := p.MediaDB, filepath.Join("/tmp/wago-data", "whatsappgo", "profiles", "work", "media.db"); got != want {
		t.Fatalf("media db=%q want %q", got, want)
	}
	if p.MediaDB == p.MessageDB {
		t.Fatal("attachments share the message database")
	}
}
