// Package parentwatch ends a helper process when the application that started
// it goes away.
//
// The desktop client owns its daemons: it starts one per account and stops
// them when it quits. That covers a clean exit and nothing else. A client that
// is killed, or that crashes, leaves its daemons running - reparented to init,
// still holding the account databases open, still connected to WhatsApp, and
// invisible to the user, who has no interface left to stop them with. Machines
// end up with daemons hours older than any window.
//
// A parent that has gone is visible from the child: the kernel reparents the
// orphan, so its parent process id changes. Watching that value needs no
// signal, no pipe and no platform-specific call, and it behaves the same on
// Linux and macOS. Windows does not reparent, so the client puts its daemons
// in a job object there instead.
package parentwatch

import (
	"context"
	"os"
	"time"
)

// DefaultInterval is how often the parent is checked. A daemon that outlives
// its client for a couple of seconds costs nothing; one that outlives it for
// hours is the problem being solved.
const DefaultInterval = 2 * time.Second

// orphanParent is the process id an orphan is handed on Linux and macOS alike.
const orphanParent = 1

// Watch calls stop once the process's parent is no longer parent. It returns
// when the context is cancelled or after stop has been called.
func Watch(ctx context.Context, stop func()) {
	watch(ctx, DefaultInterval, os.Getppid, stop)
}

func watch(ctx context.Context, interval time.Duration, parent func() int, stop func()) {
	started := parent()
	// Already an orphan: the client died between starting this process and
	// this check, which is a race a busy machine loses often enough to leave
	// exactly the daemons this package exists to prevent. Only a client passes
	// the flag that reaches here, so there is no other way to be started by
	// init.
	if started == orphanParent {
		stop()
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if parent() != started {
				stop()
				return
			}
		}
	}
}
