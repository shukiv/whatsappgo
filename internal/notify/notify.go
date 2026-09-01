package notify

import (
	"context"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// desktopExecutableName is the application that notification clicks activate.
const desktopExecutableName = "whatsappgo"

const (
	notificationService    = "org.freedesktop.Notifications"
	notificationInterface  = "org.freedesktop.Notifications"
	notificationPath       = "/org/freedesktop/Notifications"
	portalService          = "org.freedesktop.portal.Desktop"
	portalInterface        = "org.freedesktop.portal.Notification"
	portalPath             = "/org/freedesktop/portal/desktop"
	notificationStartWait  = 25 * time.Millisecond
	notificationStartTries = 40
)

var notificationDaemonCandidates = []string{
	"/usr/lib/notification-daemon/notification-daemon",
	"/usr/libexec/notification-daemon/notification-daemon",
	"/usr/libexec/notification-daemon",
}

const notificationSoundName = "message-new-instant"

// Message is everything the desktop needs to present one incoming message.
// Keeping the avatar with the text prevents notification integrations from
// silently losing sender identity as their platform payloads evolve.
type Message struct {
	ChatJID  string
	Title    string
	Body     string
	IconPath string
}

type Notifier interface {
	Notify(context.Context, Message) error
	Close() error
}

type Desktop struct {
	mu                sync.Mutex
	conn              *dbus.Conn
	signals           chan *dbus.Signal
	done              chan struct{}
	actions           map[uint32]string
	actionOrder       []uint32
	portalActions     map[string]string
	portalActionOrder []string
	nextPortalID      uint64
	profile           string
	notificationOwner string
	portalOwner       string
	usePortal         bool
	launcher          string
	uiSocket          string
	closeOnce         sync.Once
	serverPlaysSound  bool
	playSound         func()
}

func NewDesktop(profile string) (*Desktop, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	d := &Desktop{conn: conn, signals: make(chan *dbus.Signal, 16), done: make(chan struct{}), actions: make(map[uint32]string), portalActions: make(map[string]string), profile: profile, playSound: playNotificationSound}
	lookupOwner := func() (string, error) {
		var current string
		err := conn.BusObject().CallWithContext(context.Background(), "org.freedesktop.DBus.GetNameOwner", 0, notificationService).Store(&current)
		return current, err
	}
	owner, ownerErr := lookupOwner()
	if ownerErr != nil {
		// Ask D-Bus to activate a registered notification service first. Minimal
		// X11 sessions often install notification-daemon without a D-Bus service
		// file, so fall back to starting that trusted system executable directly.
		_ = conn.BusObject().CallWithContext(context.Background(), "org.freedesktop.DBus.StartServiceByName", 0, notificationService, uint32(0)).Err
		owner, ownerErr = lookupOwner()
		if ownerErr != nil {
			owner = startNotificationDaemon(notificationDaemonCandidates, lookupOwner, launchNotificationDaemon, time.Sleep)
			if owner != "" {
				log.Printf("started fallback desktop notification service")
			}
		}
	}
	if owner == "" {
		var portalOwner string
		if portalErr := conn.BusObject().CallWithContext(context.Background(), "org.freedesktop.DBus.GetNameOwner", 0, portalService).Store(&portalOwner); portalErr != nil {
			log.Printf("desktop notifications unavailable: neither %s nor %s has an owner", notificationService, portalService)
		} else {
			d.portalOwner = portalOwner
			d.usePortal = true
			log.Printf("using the desktop notification portal because %s is unavailable", notificationService)
		}
	}
	d.notificationOwner = owner
	if owner != "" {
		var capabilities []string
		if err := conn.Object(notificationService, dbus.ObjectPath(notificationPath)).
			CallWithContext(context.Background(), notificationInterface+".GetCapabilities", 0).
			Store(&capabilities); err == nil {
			for _, capability := range capabilities {
				if capability == "sound" {
					d.serverPlaysSound = true
					break
				}
			}
		}
	}
	d.launcher = resolveDesktopExecutable(os.Executable, exec.LookPath)
	d.uiSocket = desktopSocketPath(profile)
	// Signals are not restricted to the owner of an interface name, so the
	// match rules pin the sender and every delivered signal is checked against
	// the current owner of the notification service. Otherwise any process on
	// the session bus could forge a click and make this daemon spawn windows.
	if d.notificationOwner != "" {
		for _, member := range []string{"ActionInvoked", "NotificationClosed"} {
			if err := conn.AddMatchSignal(
				dbus.WithMatchInterface(notificationInterface),
				dbus.WithMatchMember(member),
				dbus.WithMatchSender(notificationService),
			); err != nil {
				conn.Close()
				return nil, err
			}
		}
	}
	if d.portalOwner != "" {
		if err := conn.AddMatchSignal(
			dbus.WithMatchInterface(portalInterface),
			dbus.WithMatchMember("ActionInvoked"),
			dbus.WithMatchSender(portalService),
		); err != nil {
			conn.Close()
			return nil, err
		}
	}
	conn.Signal(d.signals)
	go d.watchActions()
	return d, nil
}

func startNotificationDaemon(candidates []string, lookupOwner func() (string, error), launch func(string) error, pause func(time.Duration)) string {
	// Another profile may have started the service after our first lookup.
	if owner, err := lookupOwner(); err == nil && owner != "" {
		return owner
	}
	path := ""
	for _, candidate := range candidates {
		if isTrustedExecutable(candidate) {
			path = candidate
			break
		}
	}
	if path == "" || launch(path) != nil {
		return ""
	}
	for attempt := 0; attempt < notificationStartTries; attempt++ {
		if owner, err := lookupOwner(); err == nil && owner != "" {
			return owner
		}
		if attempt+1 < notificationStartTries {
			pause(notificationStartWait)
		}
	}
	return ""
}

func launchNotificationDaemon(path string) error {
	command := exec.Command(path)
	if err := command.Start(); err != nil {
		return err
	}
	_ = command.Process.Release()
	return nil
}

// maxTrackedNotifications bounds the click-target map. The server tells us
// when a notification closes, but that signal can be missed, so old entries
// are evicted in the order they were created.
const maxTrackedNotifications = 256

func (d *Desktop) Notify(ctx context.Context, message Message) error {
	if d.usePortal {
		return d.notifyPortal(ctx, message)
	}
	actions := []string{}
	if d.canOpenChat() {
		actions = []string{"default", "Open"}
	}
	appIcon, hints := freedesktopMessage(message)
	// The D-Bus round trip is not guarded, so delivering a notification never
	// blocks the signal watcher that resolves earlier notification clicks.
	call := d.conn.Object(notificationService, dbus.ObjectPath(notificationPath)).CallWithContext(ctx, notificationInterface+".Notify", 0,
		"WhatsAppGo", uint32(0), appIcon, message.Title, message.Body, actions, hints, int32(8000))
	if call.Err != nil {
		return call.Err
	}
	var id uint32
	if err := call.Store(&id); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.trackActionLocked(id, message.ChatJID)
	if !d.serverPlaysSound && d.playSound != nil {
		go d.playSound()
	}
	return nil
}

func freedesktopMessage(message Message) (string, map[string]dbus.Variant) {
	hints := map[string]dbus.Variant{
		"desktop-entry":                   dbus.MakeVariant("org.whatsappgo.Desktop"),
		"category":                        dbus.MakeVariant("im.received"),
		"sound-name":                      dbus.MakeVariant(notificationSoundName),
		"x-canonical-private-synchronous": dbus.MakeVariant("whatsappgo-" + message.ChatJID),
	}
	icon := notificationImagePath(message.IconPath)
	if icon != "" {
		hints["image-path"] = dbus.MakeVariant(icon)
	}
	return icon, hints
}

func notificationImagePath(path string) string {
	if !filepath.IsAbs(path) {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return filepath.Clean(path)
}

func playNotificationSound() {
	const player = "/usr/bin/paplay"
	const sound = "/usr/share/sounds/freedesktop/stereo/message-new-instant.oga"
	if !isTrustedExecutable(player) || notificationImagePath(sound) == "" {
		return
	}
	command := exec.Command(player, sound)
	if command.Start() == nil {
		_ = command.Process.Release()
	}
}

func (d *Desktop) notifyPortal(ctx context.Context, message Message) error {
	d.mu.Lock()
	d.nextPortalID++
	id := "whatsappgo-" + strconv.FormatInt(time.Now().UnixMilli(), 10) + "-" + strconv.FormatUint(d.nextPortalID, 10)
	if d.canOpenChat() {
		d.trackPortalActionLocked(id, message.ChatJID)
	}
	d.mu.Unlock()

	notification := map[string]dbus.Variant{
		"title":    dbus.MakeVariant(message.Title),
		"body":     dbus.MakeVariant(message.Body),
		"category": dbus.MakeVariant("im.received"),
		"priority": dbus.MakeVariant("normal"),
	}
	if d.canOpenChat() {
		notification["default-action"] = dbus.MakeVariant("open")
	}
	call := d.conn.Object(portalService, dbus.ObjectPath(portalPath)).CallWithContext(
		ctx, portalInterface+".AddNotification", 0, id, notification)
	if call.Err != nil {
		d.mu.Lock()
		d.forgetPortalActionLocked(id)
		d.mu.Unlock()
	} else if d.playSound != nil {
		go d.playSound()
	}
	return call.Err
}

func (d *Desktop) trackActionLocked(id uint32, chatJID string) {
	if _, exists := d.actions[id]; !exists {
		d.actionOrder = append(d.actionOrder, id)
	}
	d.actions[id] = chatJID
	for len(d.actionOrder) > maxTrackedNotifications {
		delete(d.actions, d.actionOrder[0])
		d.actionOrder = d.actionOrder[1:]
	}
}

func (d *Desktop) forgetActionLocked(id uint32) {
	if _, exists := d.actions[id]; !exists {
		return
	}
	delete(d.actions, id)
	for i, tracked := range d.actionOrder {
		if tracked == id {
			d.actionOrder = append(d.actionOrder[:i], d.actionOrder[i+1:]...)
			break
		}
	}
}

func (d *Desktop) trackPortalActionLocked(id, chatJID string) {
	if _, exists := d.portalActions[id]; !exists {
		d.portalActionOrder = append(d.portalActionOrder, id)
	}
	d.portalActions[id] = chatJID
	for len(d.portalActionOrder) > maxTrackedNotifications {
		delete(d.portalActions, d.portalActionOrder[0])
		d.portalActionOrder = d.portalActionOrder[1:]
	}
}

func (d *Desktop) forgetPortalActionLocked(id string) {
	if _, exists := d.portalActions[id]; !exists {
		return
	}
	delete(d.portalActions, id)
	for i, tracked := range d.portalActionOrder {
		if tracked == id {
			d.portalActionOrder = append(d.portalActionOrder[:i], d.portalActionOrder[i+1:]...)
			break
		}
	}
}

func (d *Desktop) watchActions() {
	for {
		select {
		case <-d.done:
			return
		case signal, ok := <-d.signals:
			if !ok {
				return
			}
			if len(signal.Body) < 1 {
				continue
			}
			if signal.Name == portalInterface+".ActionInvoked" {
				if !d.portalSignalIsTrusted(signal.Sender) {
					continue
				}
				id, ok := signal.Body[0].(string)
				if !ok {
					continue
				}
				d.mu.Lock()
				chatJID, owned := d.portalActions[id]
				if owned {
					d.forgetPortalActionLocked(id)
				}
				d.mu.Unlock()
				if owned {
					d.launchChat(chatJID)
				}
				continue
			}
			if !d.signalIsTrusted(signal.Sender) {
				continue
			}
			id, ok := signal.Body[0].(uint32)
			if !ok {
				continue
			}
			invoked := signal.Name == notificationInterface+".ActionInvoked"
			d.mu.Lock()
			chatJID, owned := d.actions[id]
			// A notification id is acted on once. Forgetting it here also stops
			// a repeated signal from reopening the window again and again.
			if owned {
				d.forgetActionLocked(id)
			}
			d.mu.Unlock()
			if owned && invoked {
				d.launchChat(chatJID)
			}
		}
	}
}

// signalIsTrusted reports whether a signal came from the bus name that owns
// the notification service. The owner is resolved once, when the connection is
// established; an unknown owner rejects every signal.
func (d *Desktop) signalIsTrusted(sender string) bool {
	return d.notificationOwner != "" && sender == d.notificationOwner
}

func (d *Desktop) portalSignalIsTrusted(sender string) bool {
	return d.portalOwner != "" && sender == d.portalOwner
}

func (d *Desktop) launchChat(chatJID string) {
	if openRunningDesktop(d.uiSocket, chatJID) {
		return
	}
	launcher := d.launcher
	// The path is re-validated immediately before running it so a binary that
	// was replaced after startup is not executed.
	if launcher == "" || !isTrustedExecutable(launcher) {
		return
	}
	args := []string{}
	if d.profile != "" && d.profile != "default" {
		args = []string{"--profile", d.profile}
	}
	args = append(args, "--chat", chatJID)
	command := exec.Command(launcher, args...)
	if command.Start() == nil {
		_ = command.Process.Release()
	}
}

func (d *Desktop) canOpenChat() bool {
	return d.uiSocket != "" || d.launcher != ""
}

func desktopSocketPath(profile string) string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join("/run/user", strconv.Itoa(os.Geteuid()))
	}
	if profile == "" {
		profile = "default"
	}
	return filepath.Join(runtimeDir, "whatsappgo", "ui-"+profile+".sock")
}

func openRunningDesktop(socketPath, chatJID string) bool {
	if socketPath == "" || chatJID == "" || strings.ContainsAny(chatJID, "\r\n") {
		return false
	}
	conn, err := net.DialTimeout("unix", socketPath, 250*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
	_, err = conn.Write([]byte(chatJID + "\n"))
	return err == nil
}

// resolveDesktopExecutable locates the desktop application for notification
// actions. The backend is installed and started beside the desktop
// executable, so that sibling is preferred over a PATH lookup, which fails for
// development builds that were never installed.
func resolveDesktopExecutable(executable func() (string, error), lookPath func(string) (string, error)) string {
	if self, err := executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), desktopExecutableName)
		if isTrustedExecutable(sibling) {
			return sibling
		}
	}
	if path, err := lookPath(desktopExecutableName); err == nil && isTrustedExecutable(path) {
		return path
	}
	return ""
}

// isTrustedExecutable reports whether a path is an executable file that only
// its owner can replace. A notification click runs this program, so a file or
// parent directory that other local accounts may write to is rejected: on a
// shared machine that would let another user choose what this daemon executes.
func isTrustedExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return false
	}
	return ownedAndProtected(info) && ownedAndProtected(directoryInfo(filepath.Dir(path)))
}

func directoryInfo(path string) os.FileInfo {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil
	}
	return info
}

// ownedAndProtected reports whether the current user or root owns the entry
// and no other account can write to it.
func ownedAndProtected(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	if info.Mode().Perm()&0o022 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return stat.Uid == 0 || stat.Uid == uint32(os.Geteuid())
}

func (d *Desktop) Close() error {
	var err error
	d.closeOnce.Do(func() { close(d.done); d.conn.RemoveSignal(d.signals); err = d.conn.Close() })
	return err
}

type Noop struct{}

func (Noop) Notify(context.Context, Message) error { return nil }
func (Noop) Close() error                          { return nil }
