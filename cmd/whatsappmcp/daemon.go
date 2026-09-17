package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// launcher reaches a daemon for one profile, starting one if nothing answers.
//
// Dialling first is what makes this safe beside the desktop: a profile the
// application already runs is reached, not duplicated. Two daemons on one
// profile would fight over the same SQLite files and the same socket path.
type launcher struct {
	address string
	profile string
	binary  string
	// How long a daemon started here is given to bind its socket.
	startup time.Duration

	mu      sync.Mutex
	started *exec.Cmd
	// exited is closed once the daemon started here has finished, which is the
	// only safe way to learn that from another goroutine.
	exited chan struct{}
}

// daemonName is the executable this looks for beside itself and on PATH.
var daemonName = map[bool]string{true: "whatsappd.exe", false: "whatsappd"}[runtime.GOOS == "windows"]

func newLauncher(address, profile, binary string) *launcher {
	return &launcher{address: address, profile: profile, binary: binary, startup: 20 * time.Second}
}

// connect returns a client for the profile's daemon, starting the daemon on
// the first call that finds nothing listening.
func (l *launcher) connect(ctx context.Context, timeout time.Duration) (caller, error) {
	if client, err := dialOnce(ctx, l.address, timeout); err == nil {
		return client, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// Another call may have started the daemon while this one waited.
	if client, err := dialOnce(ctx, l.address, timeout); err == nil {
		return client, nil
	}
	if err := l.start(); err != nil {
		return nil, err
	}
	return l.waitForSocket(ctx, timeout)
}

func dialOnce(ctx context.Context, address string, timeout time.Duration) (caller, error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return rpc.Dial(dialCtx, address)
}

// start runs whatsappd headless. Its arguments are a slice, never a shell
// string, so a profile name is one argument whatever it contains.
func (l *launcher) start() error {
	if l.started != nil && !l.hasExited() {
		return nil
	}
	binary, err := l.daemonPath()
	if err != nil {
		return err
	}
	args := []string{"--profile", l.profile, "--exit-with-parent", "--notifications=false"}
	if l.address != "" {
		args = append(args, "--socket", l.address)
	}
	cmd := exec.Command(binary, args...)
	// The daemon logs to stdout. Stdout here carries the protocol and nothing
	// else, so every line it writes is sent to stderr instead.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binary, err)
	}
	l.started = cmd
	exited := make(chan struct{})
	l.exited = exited
	// Nothing waits on a daemon that outlives a single call, so it is reaped
	// here to keep a finished one from becoming a zombie.
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	return nil
}

// hasExited reports whether the daemon started here has finished.
func (l *launcher) hasExited() bool {
	if l.exited == nil {
		return true
	}
	select {
	case <-l.exited:
		return true
	default:
		return false
	}
}

// daemonPath finds whatsappd: what the operator named, else the copy beside
// this binary, else whatever is on PATH.
func (l *launcher) daemonPath() (string, error) {
	if l.binary != "" {
		if _, err := os.Stat(l.binary); err != nil {
			return "", fmt.Errorf("daemon %s: %w", l.binary, err)
		}
		return l.binary, nil
	}
	if self, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(self), daemonName)
		if _, err := os.Stat(beside); err == nil {
			return beside, nil
		}
	}
	found, err := exec.LookPath(daemonName)
	if err != nil {
		return "", errors.New("no whatsappd found beside this binary or on PATH; pass --daemon with its path")
	}
	return found, nil
}

// waitForSocket dials until the daemon it just started is listening. A daemon
// opens two databases and migrates them before it binds, so the first dial
// after Start is expected to fail.
func (l *launcher) waitForSocket(ctx context.Context, timeout time.Duration) (caller, error) {
	deadline := time.Now().Add(l.startup)
	delay := 25 * time.Millisecond
	var last error
	for {
		client, err := dialOnce(ctx, l.address, timeout)
		if err == nil {
			return client, nil
		}
		last = err
		if l.hasExited() {
			return nil, fmt.Errorf("the daemon exited before it was reachable on %s; its output is above", l.address)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the daemon did not answer on %s within %s: %w", l.address, l.startup, last)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		if delay < 500*time.Millisecond {
			delay *= 2
		}
	}
}
