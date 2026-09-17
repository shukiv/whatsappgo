package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"

	"github.com/shukiv/whatsappgo/internal/gateway"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

// conflict is what the server answers when this device's copy of a collection
// is behind: the change is refused, and the catch-up patches that come back do
// not add up to the hash they are signed with. It reached the reader verbatim.
func conflict() error {
	return fmt.Errorf("%w (%s): <error code=\"409\" text=\"conflict\"/> "+
		"(also, applying patches in the response failed: failed to decode app state %s patches: "+
		"failed to verify patch v21: %w",
		whatsmeow.ErrAppStateUpdate, appstate.WAPatchRegularLow, appstate.WAPatchRegularLow,
		appstate.ErrMismatchingLTHash)
}

func pinPatch(t *testing.T) appstate.PatchInfo {
	t.Helper()
	target, err := types.ParseJID("573112522689@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	return appstate.BuildPin(target, false)
}

func newConflictClient(t *testing.T) *Client {
	t.Helper()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &Client{store: st, subs: make(map[uint64]func(gateway.Event))}
}

// Unpinning a conversation failed with a hash the reader can do nothing about.
// The copy is what is wrong, so it is replaced and the change sent again.
func TestARefusedSettingChangeIsSentAgainOnAFreshCopy(t *testing.T) {
	c := newConflictClient(t)
	patch := pinPatch(t)

	sends := 0
	c.sendAppState = func(_ context.Context, sent appstate.PatchInfo) error {
		sends++
		if sent.Type != patch.Type {
			t.Fatalf("the wrong collection was sent: %s", sent.Type)
		}
		if sends == 1 {
			return conflict()
		}
		return nil
	}
	fetches := 0
	c.fetchAppState = func(_ context.Context, name appstate.WAPatchName, full, onlyIfNotSynced bool) error {
		fetches++
		if name != patch.Type {
			t.Fatalf("the wrong collection was fetched: %s", name)
		}
		if !full {
			t.Fatal("the drifted copy was patched rather than replaced")
		}
		if onlyIfNotSynced {
			t.Fatal("a collection that had already synced was left as it was")
		}
		return nil
	}
	c.sendPeerMessage = func(context.Context, *waE2E.Message) (whatsmeow.SendResponse, error) {
		t.Fatal("the phone was asked before a fresh copy had been tried")
		return whatsmeow.SendResponse{}, nil
	}

	if err := c.sendAppStatePatch(context.Background(), patch); err != nil {
		t.Fatalf("the change was refused even after the copy was replaced: %v", err)
	}
	if sends != 2 {
		t.Fatalf("the change was sent %d times, expected the first refusal and one retry", sends)
	}
	if fetches != 1 {
		t.Fatalf("the collection was fetched %d times", fetches)
	}
}

// A change that works must not cost a fetch of the whole collection.
func TestASettingChangeThatWorksAsksForNothing(t *testing.T) {
	c := newConflictClient(t)

	sends := 0
	c.sendAppState = func(context.Context, appstate.PatchInfo) error {
		sends++
		return nil
	}
	c.fetchAppState = func(context.Context, appstate.WAPatchName, bool, bool) error {
		t.Fatal("a change that worked fetched the collection anyway")
		return nil
	}

	if err := c.sendAppStatePatch(context.Background(), pinPatch(t)); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatalf("the change was sent %d times", sends)
	}
}

// Everything else - offline, refused for another reason - is the reader's to
// see, unchanged and unretried.
func TestAnyOtherFailureIsReportedAsItIs(t *testing.T) {
	c := newConflictClient(t)
	refused := errors.New("websocket disconnected before the request could be sent")

	sends := 0
	c.sendAppState = func(context.Context, appstate.PatchInfo) error {
		sends++
		return refused
	}
	c.fetchAppState = func(context.Context, appstate.WAPatchName, bool, bool) error {
		t.Fatal("a failure that a fresh copy cannot settle fetched the collection")
		return nil
	}

	err := c.sendAppStatePatch(context.Background(), pinPatch(t))
	if !errors.Is(err, refused) {
		t.Fatalf("the failure reached the reader as %v", err)
	}
	if sends != 1 {
		t.Fatalf("the change was sent %d times", sends)
	}
}

// When the fresh copy does not verify either, only the phone can settle it. The
// reader is told that, and what to do, rather than being shown a hash.
func TestACopyThatCannotBeRebuiltAsksThePhoneAndSaysSo(t *testing.T) {
	c := newConflictClient(t)
	patch := pinPatch(t)

	sends := 0
	c.sendAppState = func(context.Context, appstate.PatchInfo) error {
		sends++
		return conflict()
	}
	c.fetchAppState = func(context.Context, appstate.WAPatchName, bool, bool) error {
		return fmt.Errorf("failed to decode app state %s patches: failed to verify patch v21: %w",
			patch.Type, appstate.ErrMismatchingLTHash)
	}
	asked := 0
	c.sendPeerMessage = func(_ context.Context, message *waE2E.Message) (whatsmeow.SendResponse, error) {
		asked++
		if got := message.GetProtocolMessage().GetPeerDataOperationRequestMessage().
			GetSyncdCollectionFatalRecoveryRequest().GetCollectionName(); got != string(patch.Type) {
			t.Fatalf("the phone was asked for %q", got)
		}
		return whatsmeow.SendResponse{}, nil
	}

	err := c.sendAppStatePatch(context.Background(), patch)
	if err == nil {
		t.Fatal("a change that never took effect was reported as done")
	}
	if asked != 1 {
		t.Fatalf("the phone was asked %d times", asked)
	}
	if sends != 1 {
		t.Fatalf("the change was sent %d times against a copy known to be broken", sends)
	}
	// The reader is not shown a hash, and is told the one thing they can do.
	for _, unwanted := range []string{"LTHash", "409", "patch v21"} {
		if strings.Contains(err.Error(), unwanted) {
			t.Fatalf("the reader was shown %q: %s", unwanted, err)
		}
	}
	for _, expected := range []string{"phone", "again"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("the message does not mention %q: %s", expected, err)
		}
	}
}

// A fresh copy that still gets refused is refused once, not forever.
func TestASecondRefusalIsNotRetriedAgain(t *testing.T) {
	c := newConflictClient(t)

	sends := 0
	c.sendAppState = func(context.Context, appstate.PatchInfo) error {
		sends++
		return conflict()
	}
	fetches := 0
	c.fetchAppState = func(context.Context, appstate.WAPatchName, bool, bool) error {
		fetches++
		return nil
	}

	if err := c.sendAppStatePatch(context.Background(), pinPatch(t)); err == nil {
		t.Fatal("a change that was refused twice was reported as done")
	}
	if sends != 2 || fetches != 1 {
		t.Fatalf("the change was sent %d times with %d fetches", sends, fetches)
	}
}
