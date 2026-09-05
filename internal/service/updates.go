package service

import (
	"context"
	"sync"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/updates"
)

// updateState is what the desktop is told about a newer version.
type updateState struct {
	mu        sync.Mutex
	latest    updates.Release
	checkedAt time.Time
	lastError string
	check     func(context.Context) (updates.Release, error)
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
	s.updates.latest = release
	s.updates.mu.Unlock()

	if release.Version == "" || !updates.Newer(s.version, release.Version) || release.Version == previous {
		return release
	}
	s.events.Publish(events.Event{Name: "update.available", Data: s.updateStatus()})
	return release
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
	}
	if !s.updates.checkedAt.IsZero() {
		status["checked_at"] = s.updates.checkedAt.UnixMilli()
	}
	if !s.updates.latest.PublishedAt.IsZero() {
		status["published_at"] = s.updates.latest.PublishedAt.UnixMilli()
	}
	return status
}
