//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func defaultDataBase() (string, error) {
	// %AppData%. Roaming is right for the message history and the device
	// identity: they belong to the account, not to the machine.
	return os.UserConfigDir()
}

func defaultCacheBase() (string, error) {
	// %LocalAppData%. Downloaded media is a cache and must not roam.
	return os.UserCacheDir()
}

// runtimeBaseDir has nothing to do with the RPC socket on Windows, where the
// daemon listens on a named pipe rather than a filesystem path. It only backs
// Paths.RuntimeDir for the few scratch files that want a per-user location.
func runtimeBaseDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "run"), nil
}
