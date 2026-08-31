package notify

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// privateDir returns a temporary directory that only its owner can write to.
// The default ACL of a shared checkout can otherwise grant group write to new
// directories, which resolveDesktopExecutable deliberately rejects.
func privateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeExecutable(t *testing.T, dir string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, desktopExecutableName)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func lookPathReturning(result string) func(string) (string, error) {
	return func(name string) (string, error) {
		if name != desktopExecutableName {
			return "", errors.New("unexpected lookup: " + name)
		}
		if result == "" {
			return "", errors.New("not found")
		}
		return result, nil
	}
}

func TestResolveDesktopExecutablePrefersSiblingOfBackend(t *testing.T) {
	dir := privateDir(t)
	sibling := writeExecutable(t, dir, 0o700)
	fallbackDir := privateDir(t)
	fallback := writeExecutable(t, fallbackDir, 0o700)
	got := resolveDesktopExecutable(func() (string, error) { return filepath.Join(dir, "whatsappd"), nil }, lookPathReturning(fallback))
	if got != sibling {
		t.Fatalf("resolved %q, want sibling %q", got, sibling)
	}
}

func TestOpenRunningDesktopUsesPrivateInstanceSocket(t *testing.T) {
	path := fmt.Sprintf("@whatsappgo-notify-%d-%d", os.Getpid(), time.Now().UnixNano())
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan string, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		buffer := make([]byte, 128)
		count, _ := conn.Read(buffer)
		received <- string(buffer[:count])
	}()
	if !openRunningDesktop(path, "123@s.whatsapp.net") {
		t.Fatal("running desktop was not activated")
	}
	select {
	case got := <-received:
		if got != "123@s.whatsapp.net\n" {
			t.Fatalf("desktop received %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("desktop activation was not delivered")
	}
	if openRunningDesktop(path, "bad\nchat") {
		t.Fatal("newline injection was accepted")
	}
}

func TestResolveDesktopExecutableFallsBackToPath(t *testing.T) {
	dir := privateDir(t)
	fallbackDir := privateDir(t)
	fallback := writeExecutable(t, fallbackDir, 0o700)
	got := resolveDesktopExecutable(func() (string, error) { return filepath.Join(dir, "whatsappd"), nil }, lookPathReturning(fallback))
	if got != fallback {
		t.Fatalf("resolved %q, want PATH result %q", got, fallback)
	}
}

func TestResolveDesktopExecutableSkipsNonExecutableSibling(t *testing.T) {
	dir := privateDir(t)
	writeExecutable(t, dir, 0o600)
	fallbackDir := privateDir(t)
	fallback := writeExecutable(t, fallbackDir, 0o700)
	got := resolveDesktopExecutable(func() (string, error) { return filepath.Join(dir, "whatsappd"), nil }, lookPathReturning(fallback))
	if got != fallback {
		t.Fatalf("resolved %q, want PATH result %q", got, fallback)
	}
}

func TestResolveDesktopExecutableIsEmptyWhenUnavailable(t *testing.T) {
	got := resolveDesktopExecutable(func() (string, error) { return "", errors.New("unknown") }, lookPathReturning(""))
	if got != "" {
		t.Fatalf("resolved %q, want empty", got)
	}
}

func TestResolveDesktopExecutableRejectsGroupWritableBinary(t *testing.T) {
	dir := privateDir(t)
	// Another local account could replace a group-writable program, and a
	// notification click would then run whatever they left behind.
	writeExecutable(t, dir, 0o770)
	got := resolveDesktopExecutable(func() (string, error) { return filepath.Join(dir, "whatsappd"), nil }, lookPathReturning(""))
	if got != "" {
		t.Fatalf("resolved group-writable binary %q", got)
	}
}

func TestResolveDesktopExecutableRejectsWorldWritableDirectory(t *testing.T) {
	dir := filepath.Join(privateDir(t), "shared")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, dir, 0o700)
	got := resolveDesktopExecutable(func() (string, error) { return filepath.Join(dir, "whatsappd"), nil }, lookPathReturning(""))
	if got != "" {
		t.Fatalf("resolved binary in a world-writable directory: %q", got)
	}
}

func TestIsTrustedExecutableAcceptsPrivateBinary(t *testing.T) {
	dir := privateDir(t)
	path := writeExecutable(t, dir, 0o755)
	if !isTrustedExecutable(path) {
		t.Fatalf("%q with mode 0755 in a private directory was rejected", path)
	}
	if isTrustedExecutable(filepath.Join(dir, "missing")) {
		t.Fatal("a missing path was accepted")
	}
	if isTrustedExecutable(dir) {
		t.Fatal("a directory was accepted as an executable")
	}
}

func TestSignalIsTrustedRequiresResolvedOwner(t *testing.T) {
	d := &Desktop{notificationOwner: ":1.42"}
	if !d.signalIsTrusted(":1.42") {
		t.Fatal("signal from the notification service owner was rejected")
	}
	if d.signalIsTrusted(":1.99") {
		t.Fatal("forged signal from another bus name was accepted")
	}
	unresolved := &Desktop{}
	if unresolved.signalIsTrusted("") || unresolved.signalIsTrusted(":1.42") {
		t.Fatal("signals were trusted without a resolved service owner")
	}
	portal := &Desktop{portalOwner: ":1.77"}
	if !portal.portalSignalIsTrusted(":1.77") || portal.portalSignalIsTrusted(":1.78") {
		t.Fatal("desktop portal signal owner was not enforced")
	}
}

func TestTrackedPortalNotificationsAreBounded(t *testing.T) {
	d := &Desktop{portalActions: make(map[string]string)}
	total := maxTrackedNotifications + 50
	for i := 0; i < total; i++ {
		d.trackPortalActionLocked(fmt.Sprintf("notification-%d", i), "chat@s.whatsapp.net")
	}
	if len(d.portalActions) != maxTrackedNotifications || len(d.portalActionOrder) != maxTrackedNotifications {
		t.Fatalf("unbounded portal tracking: actions=%d order=%d", len(d.portalActions), len(d.portalActionOrder))
	}
	if _, ok := d.portalActions["notification-0"]; ok {
		t.Fatal("oldest portal notification was not evicted")
	}
	d.forgetPortalActionLocked(fmt.Sprintf("notification-%d", total-1))
	if len(d.portalActions) != maxTrackedNotifications-1 {
		t.Fatal("portal notification action was not forgotten")
	}
}

func TestTrackedNotificationsAreBounded(t *testing.T) {
	d := &Desktop{actions: make(map[uint32]string)}
	total := maxTrackedNotifications + 50
	for i := 0; i < total; i++ {
		d.trackActionLocked(uint32(i), "chat@s.whatsapp.net")
	}
	if len(d.actions) != maxTrackedNotifications || len(d.actionOrder) != maxTrackedNotifications {
		t.Fatalf("unbounded tracking: actions=%d order=%d", len(d.actions), len(d.actionOrder))
	}
	if _, ok := d.actions[0]; ok {
		t.Fatal("oldest notification was not evicted")
	}
	if _, ok := d.actions[uint32(total-1)]; !ok {
		t.Fatal("newest notification was evicted")
	}
}

func TestForgetActionRemovesTrackingEntry(t *testing.T) {
	d := &Desktop{actions: make(map[uint32]string)}
	d.trackActionLocked(7, "a@s.whatsapp.net")
	d.trackActionLocked(8, "b@s.whatsapp.net")
	d.forgetActionLocked(7)
	if _, ok := d.actions[7]; ok {
		t.Fatal("closed notification still tracked")
	}
	if len(d.actionOrder) != 1 || d.actionOrder[0] != 8 {
		t.Fatalf("eviction order not maintained: %#v", d.actionOrder)
	}
	d.forgetActionLocked(7)
	if len(d.actionOrder) != 1 {
		t.Fatalf("repeated close changed tracking: %#v", d.actionOrder)
	}
}

func TestRepeatedNotifyForSameIDDoesNotGrowOrder(t *testing.T) {
	d := &Desktop{actions: make(map[uint32]string)}
	d.trackActionLocked(3, "a@s.whatsapp.net")
	d.trackActionLocked(3, "b@s.whatsapp.net")
	if len(d.actionOrder) != 1 || d.actions[3] != "b@s.whatsapp.net" {
		t.Fatalf("duplicate id mishandled: order=%#v actions=%#v", d.actionOrder, d.actions)
	}
}
