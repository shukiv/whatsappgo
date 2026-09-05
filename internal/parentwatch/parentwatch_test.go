package parentwatch

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchStopsWhenTheParentChanges(t *testing.T) {
	var current atomic.Int64
	current.Store(4242)
	stopped := make(chan struct{})

	go watch(context.Background(), time.Millisecond, func() int { return int(current.Load()) },
		func() { close(stopped) })

	select {
	case <-stopped:
		t.Fatal("stopped while the parent was still alive")
	case <-time.After(20 * time.Millisecond):
	}

	// A reparented orphan reports a different parent - 1 on Linux, launchd on
	// macOS. Any change means the process that started this one is gone.
	current.Store(1)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("did not stop after the parent went away")
	}
}

func TestWatchReturnsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	stops := 0

	go func() {
		watch(ctx, time.Millisecond, func() int { return 7 }, func() { stops++ })
		close(returned)
	}()

	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("watch did not return after the context was cancelled")
	}
	if stops != 0 {
		t.Fatalf("stop was called %d times for a parent that never changed", stops)
	}
}

func TestWatchStopsWhenItIsAlreadyAnOrphan(t *testing.T) {
	// The client can die between spawning the daemon and the daemon reading
	// its parent, in which case there is no change left to observe.
	stopped := make(chan struct{})
	watch(context.Background(), time.Hour, func() int { return 1 }, func() { close(stopped) })
	select {
	case <-stopped:
	default:
		t.Fatal("a process that was already an orphan kept running")
	}
}
