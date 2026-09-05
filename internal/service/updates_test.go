package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/store"
	"github.com/shukiv/whatsappgo/internal/updates"
)

func updateService(t *testing.T, version string) (*Service, <-chan events.Event) {
	t.Helper()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	broker := events.New()
	svc := New(st, &fakeGateway{}, broker)
	t.Cleanup(svc.Close)
	svc.Describe(version, 1)
	stream, unsubscribe := broker.Subscribe(8)
	t.Cleanup(unsubscribe)
	return svc, stream
}

// nextAnnouncement waits for the next update.available event, ignoring
// whatever else the service happens to publish.
func nextAnnouncement(stream <-chan events.Event, wait time.Duration) (events.Event, bool) {
	deadline := time.After(wait)
	for {
		select {
		case event, open := <-stream:
			if !open {
				return events.Event{}, false
			}
			if event.Name == "update.available" {
				return event, true
			}
		case <-deadline:
			return events.Event{}, false
		}
	}
}

func TestUpdateCheckAnnouncesANewerRelease(t *testing.T) {
	svc, stream := updateService(t, "v1.0.0")

	release := updates.Release{Version: "v1.1.0", URL: "https://example.test/v1.1.0", Notes: "faster"}
	svc.updates.check = func(context.Context) (updates.Release, error) { return release, nil }

	svc.checkForUpdate(context.Background())
	event, announced := nextAnnouncement(stream, time.Second)
	if !announced {
		t.Fatal("a newer release was not announced")
	}
	data, _ := event.Data.(map[string]any)
	if data["latest"] != "v1.1.0" || data["available"] != true || data["url"] != release.URL {
		t.Fatalf("unexpected announcement: %#v", data)
	}

	// The same version on the next tick is not news. A release that stayed the
	// newest for a week would otherwise interrupt the reader every three hours.
	svc.checkForUpdate(context.Background())
	if event, again := nextAnnouncement(stream, 200*time.Millisecond); again {
		t.Fatalf("the same release was announced twice: %#v", event.Data)
	}
}

func TestUpdateCheckStaysQuietWhenThereIsNothingNewer(t *testing.T) {
	svc, stream := updateService(t, "v2.0.0")

	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.9.0"}, nil
	}
	svc.checkForUpdate(context.Background())
	if event, announced := nextAnnouncement(stream, 200*time.Millisecond); announced {
		t.Fatalf("an older release was announced: %#v", event.Data)
	}

	status, err := svc.Handle(context.Background(), "update.status", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	fields, _ := status.(map[string]any)
	if fields["available"] != false || fields["current"] != "v2.0.0" {
		t.Fatalf("unexpected status: %#v", fields)
	}
}

func TestUpdateCheckKeepsTheFailureItRanInto(t *testing.T) {
	svc, _ := updateService(t, "v1.0.0")
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{}, errors.New("github answered 503 Service Unavailable")
	}

	// A check that fails is not an error the reader has to answer for - the
	// next tick tries again - so the request still succeeds and carries what
	// went wrong.
	status, err := svc.Handle(context.Background(), "update.check", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	fields, _ := status.(map[string]any)
	if fields["error"] != "github answered 503 Service Unavailable" || fields["available"] != false {
		t.Fatalf("unexpected status: %#v", fields)
	}
	if _, recorded := fields["checked_at"]; !recorded {
		t.Fatal("a failed check did not record when it ran")
	}
}

func TestUpdateWatchStopsWithItsContext(t *testing.T) {
	svc, _ := updateService(t, "v1.0.0")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checks := make(chan struct{}, 8)
	svc.WatchForUpdates(ctx, 20*time.Millisecond, func(context.Context) (updates.Release, error) {
		checks <- struct{}{}
		return updates.Release{Version: "v1.0.0"}, nil
	})

	select {
	case <-checks:
	case <-time.After(time.Second):
		t.Fatal("the first check did not run")
	}
	cancel()
	// Drain whatever was already in flight, then require silence.
	time.Sleep(60 * time.Millisecond)
	for len(checks) > 0 {
		<-checks
	}
	time.Sleep(80 * time.Millisecond)
	if len(checks) != 0 {
		t.Fatal("checks continued after the context was cancelled")
	}
}
