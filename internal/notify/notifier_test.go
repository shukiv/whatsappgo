package notify

import "testing"

// Presents decides whether the desktop client shows a notification itself. A
// notifier that says it presents when it does not leaves the user with no
// notifications at all; one that says it does not when it does gives them two.
func TestNoopDoesNotPresent(t *testing.T) {
	if (Noop{}).Presents() {
		t.Fatal("the no-op notifier claims it puts messages on screen")
	}
}
