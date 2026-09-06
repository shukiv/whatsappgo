package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

// awaitEvent waits for one named event, ignoring whatever else the service
// happens to publish.
func awaitEvent(stream <-chan events.Event, name string, wait time.Duration) (events.Event, bool) {
	deadline := time.After(wait)
	for {
		select {
		case event, open := <-stream:
			if !open {
				return events.Event{}, false
			}
			if event.Name == name {
				return event, true
			}
		case <-deadline:
			return events.Event{}, false
		}
	}
}

// nextAnnouncement waits for the next update.available event.
func nextAnnouncement(stream <-chan events.Event, wait time.Duration) (events.Event, bool) {
	return awaitEvent(stream, "update.available", wait)
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

func TestUpdateDownloadRefusesWhenThereIsNothingToInstall(t *testing.T) {
	svc, _ := updateService(t, "v1.0.0")
	svc.AllowUpdateDownloads(func(context.Context, updates.Release, func(int64, int64)) (string, error) {
		t.Fatal("nothing should have been downloaded")
		return "", nil
	})
	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err == nil {
		t.Fatal("a download started with no newer release to install")
	}
}

func TestUpdateDownloadAnnouncesProgressAndThenTheFile(t *testing.T) {
	svc, stream := updateService(t, "v1.0.0")
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.1.0", URL: "https://example.test/v1.1.0"}, nil
	}
	svc.checkForUpdate(context.Background())
	svc.AllowUpdateDownloads(func(_ context.Context, release updates.Release, progress func(int64, int64)) (string, error) {
		if release.Version != "v1.1.0" {
			t.Errorf("downloaded %q", release.Version)
		}
		progress(50, 100)
		return "/tmp/WhatsAppGo.AppImage", nil
	})

	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	progress, saw := awaitEvent(stream, "update.progress", time.Second)
	if !saw {
		t.Fatal("no progress was reported")
	}
	if fields, _ := progress.Data.(map[string]any); fields["received"] != int64(50) || fields["total"] != int64(100) {
		t.Fatalf("unexpected progress: %#v", progress.Data)
	}
	ready, saw := awaitEvent(stream, "update.ready", time.Second)
	if !saw {
		t.Fatal("the finished download was not announced")
	}
	fields, _ := ready.Data.(map[string]any)
	if fields["path"] != "/tmp/WhatsAppGo.AppImage" || fields["version"] != "v1.1.0" {
		t.Fatalf("unexpected announcement: %#v", fields)
	}
}

func TestUpdateDownloadReportsWhatWentWrong(t *testing.T) {
	svc, stream := updateService(t, "v1.0.0")
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.1.0"}, nil
	}
	svc.checkForUpdate(context.Background())
	svc.AllowUpdateDownloads(func(context.Context, updates.Release, func(int64, int64)) (string, error) {
		return "", errors.New("does not match the checksum the release published")
	})

	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	failed, saw := awaitEvent(stream, "update.failed", time.Second)
	if !saw {
		t.Fatal("a failed download was never reported")
	}
	fields, _ := failed.Data.(map[string]any)
	if fields["error"] != "does not match the checksum the release published" {
		t.Fatalf("unexpected failure: %#v", fields)
	}
	// The failure is part of the status, so a reader who missed the event
	// still sees why the update did not happen.
	status, err := svc.Handle(context.Background(), "update.status", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if fields, _ := status.(map[string]any); fields["error"] == "" {
		t.Fatal("the status forgot the failure")
	}
}

func TestUpdateDownloadDoesNotStartTwice(t *testing.T) {
	svc, _ := updateService(t, "v1.0.0")
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.1.0"}, nil
	}
	svc.checkForUpdate(context.Background())
	release := make(chan struct{})
	starts := make(chan struct{}, 4)
	svc.AllowUpdateDownloads(func(context.Context, updates.Release, func(int64, int64)) (string, error) {
		starts <- struct{}{}
		<-release
		return "/tmp/WhatsAppGo.AppImage", nil
	})

	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	<-starts
	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	close(release)
	time.Sleep(100 * time.Millisecond)
	if len(starts) != 0 {
		t.Fatal("a second download ran while the first was still going")
	}
}

func TestUpdateForgetsAFileThatIsNoLongerTheNewest(t *testing.T) {
	svc, stream := updateService(t, "v1.0.0")
	offered := updates.Release{Version: "v1.1.0"}
	svc.updates.check = func(context.Context) (updates.Release, error) { return offered, nil }
	svc.checkForUpdate(context.Background())
	svc.AllowUpdateDownloads(func(context.Context, updates.Release, func(int64, int64)) (string, error) {
		return "/tmp/WhatsAppGo-x86_64.AppImage", nil
	})
	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, saw := awaitEvent(stream, "update.ready", time.Second); !saw {
		t.Fatal("the download never finished")
	}

	// A newer release arrives before the reader installed the last one. The
	// artifacts share one name, so the file on disk is the old version and
	// offering it as the new one would install the wrong thing.
	offered = updates.Release{Version: "v1.2.0"}
	svc.checkForUpdate(context.Background())
	status, err := svc.Handle(context.Background(), "update.status", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	fields, _ := status.(map[string]any)
	if fields["downloaded"] != "" {
		t.Fatalf("a file downloaded for v1.1.0 is still offered as %v: %v", fields["latest"], fields["downloaded"])
	}
}

func TestUpdateDownloadStartsOnceWhenAskedAtOnce(t *testing.T) {
	svc, _ := updateService(t, "v1.0.0")
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.1.0"}, nil
	}
	svc.checkForUpdate(context.Background())
	release := make(chan struct{})
	var starts atomic.Int64
	svc.AllowUpdateDownloads(func(context.Context, updates.Release, func(int64, int64)) (string, error) {
		starts.Add(1)
		<-release
		return "/tmp/WhatsAppGo-x86_64.AppImage", nil
	})

	// Two requests that arrive together are still one download: a second
	// writer over the same file is a corrupt install.
	var asking sync.WaitGroup
	begin := make(chan struct{})
	for i := 0; i < 8; i++ {
		asking.Add(1)
		go func() {
			defer asking.Done()
			<-begin
			if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
				t.Error(err)
			}
		}()
	}
	close(begin)
	asking.Wait()
	// The download runs on its own goroutine, so wait for one to be under way
	// and leave any second one the same chance to start.
	deadline := time.Now().Add(2 * time.Second)
	for starts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	got := starts.Load()
	close(release)
	if got != 1 {
		t.Fatalf("the download started %d times", got)
	}
}

// A download that finishes after a newer release has been found belongs to
// neither: the artifacts share a name, so offering it under the newer version's
// label would install the older one.
func TestFinishedDownloadOfASupersededReleaseIsDiscarded(t *testing.T) {
	svc, stream := updateService(t, "v1.0.0")
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.1.0", URL: "https://example.test/v1.1.0"}, nil
	}
	svc.checkForUpdate(context.Background())

	artifact := filepath.Join(t.TempDir(), "WhatsAppGo-x86_64.AppImage")
	if err := os.WriteFile(artifact, []byte("older release"), 0o600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	svc.AllowUpdateDownloads(func(context.Context, updates.Release, func(int64, int64)) (string, error) {
		<-release
		return artifact, nil
	})
	if _, err := svc.Handle(context.Background(), "update.download", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}

	// A newer release turns up while the transfer is still running.
	svc.updates.check = func(context.Context) (updates.Release, error) {
		return updates.Release{Version: "v1.2.0", URL: "https://example.test/v1.2.0"}, nil
	}
	svc.checkForUpdate(context.Background())
	close(release)

	if _, saw := awaitEvent(stream, "update.ready", time.Second); saw {
		t.Fatal("a file fetched for the previous release was announced as ready")
	}
	status := svc.updateStatus()
	if status["latest"] != "v1.2.0" {
		t.Fatalf("the newer release was lost: %#v", status)
	}
	if downloaded, _ := status["downloaded"].(string); downloaded != "" {
		t.Fatalf("the superseded artifact is still on offer: %q", downloaded)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("the superseded artifact was left on disk: %v", err)
	}
}
