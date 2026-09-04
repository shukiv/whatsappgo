//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

func defaultDataBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support"), nil
}

func defaultCacheBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Caches"), nil
}

// runtimeBaseDir puts the socket under ~/Library/Application Support, next to
// the databases.
//
// $TMPDIR (/var/folders/...) is the closer analogue of XDG_RUNTIME_DIR - macOS
// creates it per user with mode 0700 - but two things rule it out. The system's
// periodic cleaner deletes files there by age, and a bound socket it removed
// would strand the daemon on a path no client can reach. And this path has to
// match what desktop/src/rpcclient.cpp computes; naming the directory outright
// on both sides is what keeps them from drifting apart.
//
// The home directory is long enough to matter here: macOS caps sun_path at 104
// bytes. "/Users/<name>/Library/Application Support/whatsappgo/whatsappd-<profile>.sock"
// leaves room for a 32-character profile under any ordinary account name.
func runtimeBaseDir() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support"), nil
}
