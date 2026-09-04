//go:build !windows

package config

import "path/filepath"

// SocketAddress returns the address the daemon listens on and the clients dial.
// On Unix that is a filesystem path inside the per-user runtime directory,
// whose 0700 mode is half of the socket's access control; the other half is the
// 0600 mode the listener sets on the socket itself.
func SocketAddress(runtimeDir, profile string) string {
	name := "whatsappd.sock"
	if profile != "default" {
		name = "whatsappd-" + profile + ".sock"
	}
	return filepath.Join(runtimeDir, name)
}
