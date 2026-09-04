//go:build linux

package notify

import "testing"

// The freedesktop service draws the notification whether or not a client of
// this daemon is running, so the client must stay quiet for these.
func TestFreedesktopBackendPresents(t *testing.T) {
	if !(&Desktop{}).Presents() {
		t.Fatal("the freedesktop backend does not claim its own notifications")
	}
}
