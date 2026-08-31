package whatsapp

import (
	"testing"

	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func TestMediaCursorRoundTrip(t *testing.T) {
	cursor := localstore.MessageCursor{Timestamp: 1788103290123, MessageID: "3EB0F1388807E98A42"}
	if got := parseMediaCursor(formatMediaCursor(cursor)); got != cursor {
		t.Fatalf("cursor did not survive a round trip: %#v", got)
	}
	// A missing or damaged marker must restart the scan rather than skip work.
	for _, broken := range []string{"", "nonsense", "notanumber|id", "123"} {
		if got := parseMediaCursor(broken); got != (localstore.MessageCursor{}) {
			t.Fatalf("%q produced %#v, want an empty cursor", broken, got)
		}
	}
	// An id containing the separator keeps everything after the first one.
	odd := localstore.MessageCursor{Timestamp: 5, MessageID: "a|b"}
	if got := parseMediaCursor(formatMediaCursor(odd)); got != odd {
		t.Fatalf("separator in the id broke the cursor: %#v", got)
	}
}
