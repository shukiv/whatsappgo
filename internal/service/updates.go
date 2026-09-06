package service

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/updates"
)

// UpdateDownloader fetches the artifact for the running platform and returns
// where it was written. The service does not install anything itself: the
// desktop knows what it is running from and can restart itself, and this
// process cannot.
type UpdateDownloader func(ctx context.Context, release updates.Release,
	progress func(received, total int64)) (string, error)

// updateState is what the desktop is told about a newer version.
type updateState struct {
	mu          sync.Mutex
	latest      updates.Release
	checkedAt   time.Time
	lastError   string
	check       func(context.Context) (updates.Release, error)
	download    UpdateDownloader
	downloading bool
	downloaded  string
}

// WatchForUpdates looks for a newer release now and every interval after that,
// and publishes update.available whenever the version on offer changes.
//
// Nothing is downloaded and nothing is replaced: the desktop shows what was
// found and the reader decides. A build with no version of its own - anybody's
// working copy - is never behind a release, so check reports nothing for it.
func (s *Service) WatchForUpdates(ctx context.Context, interval time.Duration,
	check func(context.Context) (updates.Release, error)) {
	if check == nil {
		return
	}
	s.updates.mu.Lock()
	s.updates.check = check
	s.updates.mu.Unlock()
	if interval <= 0 {
		interval = updates.Interval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			s.checkForUpdate(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// checkForUpdate runs one check and announces a version nobody has been told
// about yet. Announcing the same version on every tick would turn a release
// into a recurring interruption.
func (s *Service) checkForUpdate(ctx context.Context) updates.Release {
	s.updates.mu.Lock()
	check := s.updates.check
	previous := s.updates.latest.Version
	s.updates.mu.Unlock()
	if check == nil {
		return updates.Release{}
	}

	release, err := check(ctx)
	s.updates.mu.Lock()
	s.updates.checkedAt = time.Now()
	if err != nil {
		s.updates.lastError = err.Error()
		s.updates.mu.Unlock()
		return updates.Release{}
	}
	s.updates.lastError = ""
	if release.Version != s.updates.latest.Version {
		// A file downloaded for the release before this one is not the one
		// the reader would be offered now, and the artifacts share a name.
		s.updates.downloaded = ""
	}
	s.updates.latest = release
	s.updates.mu.Unlock()

	if release.Version == "" || !updates.Newer(s.version, release.Version) || release.Version == previous {
		return release
	}
	s.events.Publish(events.Event{Name: "update.available", Data: s.updateStatus()})
	return release
}

// AllowUpdateDownloads lets update.download fetch an artifact. A daemon that
// was never given a downloader answers that it cannot install anything, which
// is the honest answer for a build that was not packaged.
func (s *Service) AllowUpdateDownloads(download UpdateDownloader) {
	s.updates.mu.Lock()
	defer s.updates.mu.Unlock()
	s.updates.download = download
}

// startUpdateDownload fetches the release that was found, in the background.
// The transfer is minutes of work on a slow line, so the request returns as
// soon as it has started and the progress arrives as events.
func (s *Service) startUpdateDownload() (map[string]any, error) {
	// Read and claim under one lock: two requests arriving together both
	// found "not running" when this was two.
	s.updates.mu.Lock()
	release := s.updates.latest
	download := s.updates.download
	running := s.updates.downloading
	startable := download != nil && !running &&
		release.Version != "" && updates.Newer(s.version, release.Version)
	if startable {
		s.updates.downloading = true
	}
	s.updates.mu.Unlock()

	if download == nil {
		return nil, errors.New("this build cannot install updates by itself - open the release page")
	}
	if release.Version == "" || !updates.Newer(s.version, release.Version) {
		return nil, errors.New("there is no newer version to install")
	}
	if !startable {
		// Asking twice is a double click, not a second download.
		return map[string]any{"started": true, "version": release.Version}, nil
	}

	go func() {
		// The request that started this is long gone, and a download that
		// outlives it is the point. An hour is far longer than any of these
		// artifacts needs and short enough that a stalled transfer ends.
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		var lastReported time.Time
		path, err := download(ctx, release, func(received, total int64) {
			// One event per received chunk would be thousands of them.
			if received < total && time.Since(lastReported) < 250*time.Millisecond {
				return
			}
			lastReported = time.Now()
			s.events.Publish(events.Event{Name: "update.progress", Data: map[string]any{
				"received": received, "total": total, "version": release.Version,
			}})
		})
		s.updates.mu.Lock()
		s.updates.downloading = false
		// A newer release can be found while this transfer runs, and the
		// artifacts share a name. A file fetched for the release before the
		// current one is not the one anybody would be offered, so it is thrown
		// away rather than presented under the newer version's label.
		superseded := err == nil && s.updates.latest.Version != release.Version
		if err != nil {
			s.updates.lastError = err.Error()
		} else if !superseded {
			s.updates.lastError = ""
			s.updates.downloaded = path
		}
		s.updates.mu.Unlock()
		if err != nil {
			s.events.Publish(events.Event{Name: "update.failed", Data: map[string]any{"error": err.Error()}})
			return
		}
		if superseded {
			_ = os.Remove(path)
			return
		}
		s.events.Publish(events.Event{Name: "update.ready", Data: map[string]any{
			"path": path, "version": release.Version, "url": release.URL,
		}})
	}()
	return map[string]any{"started": true, "version": release.Version}, nil
}

// updateStatus describes what is installed and what is on offer.
func (s *Service) updateStatus() map[string]any {
	s.updates.mu.Lock()
	defer s.updates.mu.Unlock()
	status := map[string]any{
		"current":   s.version,
		"available": s.updates.latest.Version != "" && updates.Newer(s.version, s.updates.latest.Version),
		"latest":    s.updates.latest.Version,
		"url":       s.updates.latest.URL,
		"notes":     s.updates.latest.Notes,
		"error":     s.updates.lastError,
		// What this build can do about it: a packaged build downloads and
		// installs, anything else can only be pointed at the release page.
		"installable": s.updates.download != nil && updates.AssetName(runtime.GOOS, runtime.GOARCH) != "",
		"downloading": s.updates.downloading,
		"downloaded":  s.updates.downloaded,
	}
	if !s.updates.checkedAt.IsZero() {
		status["checked_at"] = s.updates.checkedAt.UnixMilli()
	}
	if !s.updates.latest.PublishedAt.IsZero() {
		status["published_at"] = s.updates.latest.PublishedAt.UnixMilli()
	}
	return status
}
