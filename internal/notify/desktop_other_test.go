//go:build !linux

package notify

import "testing"

func TestNewDesktopIsUnavailableOffLinux(t *testing.T) {
	if _, err := NewDesktop("default"); err == nil {
		t.Fatal("constructed a freedesktop notifier on a platform without one")
	}
}
