//go:build !windows

package rpc

import (
	"context"
	"errors"
	"net"
	"os"
	"time"
)

// listen binds the daemon's owner-only socket. The 0600 mode on the socket and
// the 0700 mode on its runtime directory are the whole access control story on
// Unix: the kernel checks both before a connect succeeds.
func listen(path string) (net.Listener, error) {
	if err := removeStaleSocket(path); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

func dialContext(ctx context.Context, path string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", path)
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("refusing to replace non-socket path: " + path)
	}
	conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err == nil {
		conn.Close()
		return errors.New("daemon is already listening on " + path)
	}
	return os.Remove(path)
}

// defaultSocketCheckInterval is how often a listening daemon checks that it
// still owns its socket path. It is long enough to be free and short enough
// that an orphan does not sit on the databases for minutes.
const defaultSocketCheckInterval = 5 * time.Second

// watchListener closes the returned channel once the socket this daemon bound
// is no longer the socket clients reach, because the file was unlinked or
// replaced by another daemon.
//
// Unlinking a bound Unix socket does not disturb the process holding it: the
// listener keeps accepting on an inode with no name, so the daemon survives
// with nobody able to reach it and keeps its SQLite handles open. Every retry
// by a client then leaves one more of these behind. Exiting instead makes the
// condition self-healing.
func watchListener(ctx context.Context, path string, interval time.Duration) <-chan struct{} {
	if interval <= 0 {
		interval = defaultSocketCheckInterval
	}
	replaced := make(chan struct{})
	bound, err := os.Stat(path)
	if err != nil {
		close(replaced)
		return replaced
	}
	go func() {
		defer close(replaced)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := os.Stat(path)
				if err != nil || !os.SameFile(bound, current) {
					return
				}
			}
		}
	}()
	return replaced
}
