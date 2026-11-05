//go:build !windows

package main_test

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The desktop client stops the daemons it starts, but only when it exits
// cleanly. This is the other case: the process that started the daemon is gone
// and nothing asked the daemon to stop.
func TestDaemonExitsWhenItsParentGoesAway(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the daemon")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "whatsappd")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build whatsappd: %v\n%s", err, out)
	}

	// A shell that starts the daemon, waits, and then exits leaves it
	// orphaned, which is exactly what a killed client leaves behind. The wait
	// is what makes the first half of the test mean anything: the daemon has
	// to keep running while the process that started it is still there.
	//
	// The daemon's path reaches the shell as an argument rather than as part
	// of the script, so nothing in the name of a temporary directory can run
	// as a command.
	launcher := exec.Command("/bin/sh", "-c",
		`"$1" --profile exitparent --notifications=false --exit-with-parent >/dev/null 2>&1 & echo $!; sleep 3`,
		"sh", binary)
	launcher.Env = isolatedEnvironment(t, root)
	// The shell and the daemon get a process group of their own, so the test
	// can end both however it ends, even before it has learned the daemon's
	// process id.
	launcher.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := launcher.StdoutPipe()
	if err != nil {
		t.Fatalf("read the launcher's output: %v", err)
	}
	if err := launcher.Start(); err != nil {
		t.Fatalf("start the daemon: %v", err)
	}
	// The group is killed before the shell is reaped: until then the shell
	// holds the group's id, so it cannot have been handed to anything else.
	t.Cleanup(func() {
		_ = syscall.Kill(-launcher.Process.Pid, syscall.SIGKILL)
		_ = launcher.Wait()
	})
	reported, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read the daemon's process id: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(reported))
	if err != nil {
		t.Fatalf("read the daemon's process id from %q: %v", reported, err)
	}

	// It has to survive while it is being used, or the flag would just be a
	// slower way of never starting.
	time.Sleep(time.Second)
	if !running(pid) {
		t.Fatal("the daemon exited while its parent was still alive")
	}

	deadline := time.Now().Add(30 * time.Second)
	for running(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatal("the daemon outlived the process that started it")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// isolatedEnvironment is the test's own environment with the home and every
// XDG directory moved under root, so neither the shell nor the daemon can
// reach the real ones, whatever they are asked to do.
func isolatedEnvironment(t *testing.T, root string) []string {
	t.Helper()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatalf("create a home directory: %v", err)
	}
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name == "HOME" || strings.HasPrefix(name, "XDG_") {
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"HOME="+home,
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		// Short, because macOS refuses a socket path over 104 characters.
		"XDG_RUNTIME_DIR="+shortRuntimeDir(t),
	)
}

func shortRuntimeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "wag-run-")
	if err != nil {
		t.Fatalf("create a runtime directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func running(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
