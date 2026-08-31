package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shuki/whatsappgo/internal/config"
	"github.com/shuki/whatsappgo/internal/events"
	"github.com/shuki/whatsappgo/internal/notify"
	"github.com/shuki/whatsappgo/internal/rpc"
	"github.com/shuki/whatsappgo/internal/service"
	"github.com/shuki/whatsappgo/internal/store"
	"github.com/shuki/whatsappgo/internal/whatsapp"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	socketOverride := flag.String("socket", "", "override Unix socket path")
	profile := flag.String("profile", "default", "isolated account profile name")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := run(*socketOverride, *profile); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run(socketOverride, profile string) error {
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
	_ = os.Chmod(paths.MessageDB, 0o600)

	// notifier must stay an interface value. Assigning a nil *notify.Desktop
	// to it would produce a non-nil interface wrapping a nil pointer, which
	// defeats the nil check in whatsapp.New and panics on the first
	// notification when the session bus is unavailable.
	var notifier notify.Notifier = notify.Noop{}
	if desktop, err := notify.NewDesktop(paths.Profile); err != nil {
		log.Printf("desktop notifications disabled: %v", err)
	} else {
		notifier = desktop
		defer desktop.Close()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wa, err := whatsapp.New(ctx, paths.DeviceDB, paths.MediaDir, messageStore, notifier)
	if err != nil {
		return fmt.Errorf("initialize WhatsApp: %w", err)
	}
	defer wa.Close()
	_ = os.Chmod(paths.DeviceDB, 0o600)
	broker := events.New()
	app := service.New(messageStore, wa, broker)
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
