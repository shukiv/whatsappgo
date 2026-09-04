//go:build windows

package config

import (
	"os"
	"strings"
)

// PipeUserSegment reduces the current account name to characters that are legal
// in a pipe name and stable across the daemon and the desktop client. The Qt
// client repeats this transformation in desktop/src/rpcclient.cpp; the two must
// stay in step or the client will dial a pipe nobody is listening on.
func PipeUserSegment() string {
	name := os.Getenv("USERNAME")
	var builder strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
		if builder.Len() >= 32 {
			break
		}
	}
	if builder.Len() == 0 {
		return "user"
	}
	return builder.String()
}
