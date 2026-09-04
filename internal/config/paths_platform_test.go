package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The desktop client mirrors these directories in desktop/src/rpcclient.cpp.
// If the XDG variables ever stopped winning, the client and the daemon would
// disagree about where an account lives and the client would show an empty one.
func TestXDGVariablesWinOnEveryPlatform(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join("/tmp", "wago-data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join("/tmp", "wago-cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join("/tmp", "wago-runtime"))
	p, err := ResolveProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.DataDir, filepath.Join("/tmp", "wago-data")) {
		t.Fatalf("data dir=%q ignored XDG_DATA_HOME", p.DataDir)
	}
	if !strings.HasPrefix(p.CacheDir, filepath.Join("/tmp", "wago-cache")) {
		t.Fatalf("cache dir=%q ignored XDG_CACHE_HOME", p.CacheDir)
	}
}

func TestDefaultDirectoriesFollowThePlatform(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join("/tmp", "wago-runtime"))
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}
	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var wantData string
	switch runtime.GOOS {
	case "darwin":
		wantData = filepath.Join(home, "Library", "Application Support", "whatsappgo")
	case "windows":
		wantData = "" // %AppData% is not predictable from here; only check it is absolute.
	default:
		wantData = filepath.Join(home, ".local", "share", "whatsappgo")
	}
	if wantData != "" && p.DataDir != wantData {
		t.Fatalf("data dir=%q want %q", p.DataDir, wantData)
	}
	if !filepath.IsAbs(p.DataDir) {
		t.Fatalf("data dir=%q is not absolute", p.DataDir)
	}
}
