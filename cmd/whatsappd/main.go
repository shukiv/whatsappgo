package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/shukiv/whatsappgo/internal/bugreport"
	"github.com/shukiv/whatsappgo/internal/config"
	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/mediastore"
	"github.com/shukiv/whatsappgo/internal/notify"
	"github.com/shukiv/whatsappgo/internal/parentwatch"
	"github.com/shukiv/whatsappgo/internal/rpc"
	"github.com/shukiv/whatsappgo/internal/service"
	"github.com/shukiv/whatsappgo/internal/store"
	"github.com/shukiv/whatsappgo/internal/updates"
	"github.com/shukiv/whatsappgo/internal/whatsapp"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	socketOverride := flag.String("socket", "", "override Unix socket path")
	profile := flag.String("profile", "default", "isolated account profile name")
	desktopNotifications := flag.Bool("notifications", true, "send native desktop notifications")
	exitWithParent := flag.Bool("exit-with-parent", false, "shut down when the process that started this one goes away")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := run(*socketOverride, *profile, *desktopNotifications, *exitWithParent); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

// restrictDatabase limits a database to its owner. Write-ahead logging creates
// two companion files that hold the same decrypted content as the database
// itself, and SQLite creates them using the process umask rather than the
// permissions of the database.
func restrictDatabase(path string) {
	for _, companion := range []string{path, path + "-wal", path + "-shm"} {
		if _, err := os.Stat(companion); err != nil {
			continue
		}
		if err := os.Chmod(companion, 0o600); err != nil {
			log.Printf("could not restrict %s: %v", filepath.Base(companion), err)
		}
	}
}

// countProfiles reports how many accounts exist on this machine, for the
// environment attached to a bug report. Only the count travels, never a name:
// a profile name is the user's own word for a phone number.
func countProfiles() int {
	// The default profile lives at the root of the data directory and the
	// others under profiles/, so the count starts at one for the root.
	root, err := config.Resolve()
	if err != nil {
		return 1
	}
	entries, err := os.ReadDir(filepath.Join(root.DataDir, "profiles"))
	if err != nil {
		return 1
	}
	count := 1
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

func run(socketOverride, profile string, desktopNotifications, exitWithParent bool) error {
	paths, err := config.ResolveProfile(profile)
	if err != nil {
		return err
	}
	if socketOverride != "" {
		paths.Socket = socketOverride
	}
	if err := paths.Ensure(); err != nil {
		return fmt.Errorf("create application directories: %w", err)
	}
	messageStore, err := store.Open(paths.MessageDB)
	if err != nil {
		return fmt.Errorf("open message store: %w", err)
	}
	defer messageStore.Close()
	restrictDatabase(paths.MessageDB)

	mediaStore, err := mediastore.Open(paths.MediaDB)
	if err != nil {
		return fmt.Errorf("open attachment store: %w", err)
	}
	defer mediaStore.Close()
	restrictDatabase(paths.MediaDB)

	// notifier must stay an interface value. Assigning a nil *notify.Desktop
	// to it would produce a non-nil interface wrapping a nil pointer, which
	// defeats the nil check in whatsapp.New and panics on the first
	// notification when the session bus is unavailable.
	var notifier notify.Notifier = notify.Noop{}
	if desktopNotifications {
		if desktop, err := notify.NewDesktop(paths.Profile); err != nil {
			// No service this daemon can post to. The notification event still
			// goes out, and the desktop client presents it with the platform's
			// own API. See notify.Notifier.Presents.
			log.Printf("presenting notifications through the desktop client: %v", err)
		} else {
			notifier = desktop
			defer desktop.Close()
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// The desktop client stops the daemons it started, but only if it exits
	// cleanly. Killed or crashed, it used to leave them running for hours with
	// no window left to stop them from. See internal/parentwatch.
	if exitWithParent {
		go parentwatch.Watch(ctx, stop)
	}
	wa, err := whatsapp.New(ctx, paths.DeviceDB, paths.MediaDir, messageStore, mediaStore, notifier)
	if err != nil {
		return fmt.Errorf("initialize WhatsApp: %w", err)
	}
	defer wa.Close()
	restrictDatabase(paths.DeviceDB)
	broker := events.New()
	app := service.New(messageStore, wa, broker)
	app.Describe(version, countProfiles())
	// Look for a newer release now and every few hours after that. Nothing is
	// downloaded here: the desktop is told what exists and the reader decides.
	app.WatchForUpdates(ctx, updates.Interval, func(ctx context.Context) (updates.Release, error) {
		return updates.Latest(ctx, nil, "", bugreport.Repository)
	})
	// Downloading is a separate decision, taken by the reader. The artifact
	// lands in the cache directory, which is the right place for a file that
	// can be fetched again.
	app.AllowUpdateDownloads(func(ctx context.Context, release updates.Release,
		progress func(received, total int64)) (string, error) {
		downloader := updates.Downloader{Dir: filepath.Join(paths.CacheDir, "updates")}
		return downloader.Fetch(ctx, release, runtime.GOOS, runtime.GOARCH, progress)
	})
	defer app.Close()
	server := rpc.NewServer(paths.Socket, app, broker)
	if err := server.Listen(); err != nil {
		return err
	}
	defer server.Close()
	if wa.Status().LoggedIn {
		go func() {
			connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := wa.Connect(connectCtx); err != nil {
				log.Printf("initial connection failed: %v", err)
			}
		}()
	}
	log.Printf("whatsappd %s listening on %s", version, paths.Socket)
	return server.Serve(ctx)
}
