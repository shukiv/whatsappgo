package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
)

// drifted is what a collection looks like once its patches stop adding up to
// the hash the server signs them with. whatsmeow writes this to a log and
// leaves the stored version where it was.
func drifted(name appstate.WAPatchName) error {
	return fmt.Errorf("failed to decode app state %s patches: failed to verify patch v155: %w",
		name, appstate.ErrMismatchingLTHash)
}

// A call taken, or a conversation pinned, while the app was closed is never
// announced to it: the server tells a device that is listening. Every
// collection is therefore asked for what it missed on the way back in.
func TestEveryCollectionIsAskedForWhatItMissedOnConnection(t *testing.T) {
	c := newConflictClient(t)
	asked := map[appstate.WAPatchName]int{}
	c.fetchAppState = func(_ context.Context, name appstate.WAPatchName, full, onlyIfNotSynced bool) error {
		if full {
			t.Fatalf("collection %s was replaced when nothing was wrong with it", name)
		}
		if onlyIfNotSynced {
			t.Fatalf("collection %s was skipped because it had synced once before", name)
		}
		asked[name]++
		return nil
	}
	c.catchUpAppState()
	for _, name := range appstate.AllPatchNames {
		if asked[name] != 1 {
			t.Fatalf("collection %s was asked %d times, want once", name, asked[name])
		}
	}
}

// The failure that froze the call list: patches that no longer verify. Nothing
// in whatsmeow asks again, so the copy is replaced here instead.
func TestACollectionThatStoppedVerifyingIsReplaced(t *testing.T) {
	c := newConflictClient(t)
	const broken = appstate.WAPatchRegular
	replaced := 0
	c.fetchAppState = func(_ context.Context, name appstate.WAPatchName, full, _ bool) error {
		if name != broken {
			if full {
				t.Fatalf("healthy collection %s was replaced", name)
			}
			return nil
		}
		if !full {
			return drifted(name)
		}
		replaced++
		return nil
	}
	c.sendPeerMessage = func(context.Context, *waE2E.Message) (whatsmeow.SendResponse, error) {
		t.Fatal("the phone was asked for a collection a fresh copy had already settled")
		return whatsmeow.SendResponse{}, nil
	}
	c.catchUpAppState()
	if replaced != 1 {
		t.Fatalf("the drifted collection was replaced %d times, want once", replaced)
	}
}

// A fresh copy that still does not add up is beyond this device. Only the
// phone holds anything better, and it is asked without telling the reader:
// nobody asked for this sync.
func TestAFreshCopyThatStillFailsAsksThePhone(t *testing.T) {
	c := newConflictClient(t)
	const broken = appstate.WAPatchRegular
	c.fetchAppState = func(_ context.Context, name appstate.WAPatchName, _ bool, _ bool) error {
		if name != broken {
			return nil
		}
		return drifted(name)
	}
	asked := 0
	c.sendPeerMessage = func(_ context.Context, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
		asked++
		request := msg.GetProtocolMessage().GetPeerDataOperationRequestMessage().GetSyncdCollectionFatalRecoveryRequest()
		if request.GetCollectionName() != string(broken) {
			t.Fatalf("the phone was asked for %q rather than for %s", request.GetCollectionName(), broken)
		}
		return whatsmeow.SendResponse{}, nil
	}
	c.catchUpAppState()
	if asked != 1 {
		t.Fatalf("the phone was asked %d times, want once", asked)
	}

	// And asking is remembered, so a reconnection does not ask again while the
	// answer is still on its way.
	c.catchUpAppState()
	if asked != 1 {
		t.Fatalf("the phone was asked again on the next connection: %d requests", asked)
	}
}

// A collection that could not be reached at all has not drifted. Replacing it
// would throw away a good copy over a dropped connection.
func TestACollectionThatCouldNotBeReachedIsLeftAlone(t *testing.T) {
	c := newConflictClient(t)
	offline := errors.New("websocket disconnected before info query returned response")
	c.fetchAppState = func(_ context.Context, name appstate.WAPatchName, full, _ bool) error {
		if full {
			t.Fatalf("collection %s was replaced after a failure that was not drift", name)
		}
		return offline
	}
	c.sendPeerMessage = func(context.Context, *waE2E.Message) (whatsmeow.SendResponse, error) {
		t.Fatal("the phone was asked for a collection over a dropped connection")
		return whatsmeow.SendResponse{}, nil
	}
	c.catchUpAppState()
}
