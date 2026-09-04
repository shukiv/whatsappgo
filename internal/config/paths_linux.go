//go:build linux

package config

import (
	"errors"
	"os"
	"path/filepath"
)

func defaultDataBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func defaultCacheBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// runtimeBaseDir stays strict on Linux. XDG_RUNTIME_DIR is the only directory
// the session guarantees to be per-user, mode 0700 and cleared at logout, and
// the socket's access control depends on all three. Falling back to a
// world-traversable directory would silently widen who can reach the daemon.
func runtimeBaseDir() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir, nil
	}
	return "", errors.New("XDG_RUNTIME_DIR is not set; a per-user runtime directory is required")
}
