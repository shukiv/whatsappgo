// Command whatsapptui is WhatsAppGo in a terminal. It is a second front end
// on the same daemon the desktop client uses, not a second implementation of
// WhatsApp: everything it shows and everything it does arrives over the same
// socket, so a fix in the daemon reaches both at once.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shukiv/whatsappgo/internal/config"
)

var validProfile = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func main() {
	profile := flag.String("profile", "default", "account profile to open")
	socket := flag.String("socket", "", "daemon socket (default: the profile's own)")
	flag.Parse()

	if err := run(*profile, *socket); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(profile, socket string) error {
	// The profile names a directory and a socket, so it is checked here
	// rather than handed to the filesystem as typed.
	if !validProfile.MatchString(profile) {
		return fmt.Errorf("not a profile name: %q", profile)
	}
	if socket == "" {
		paths, err := config.ResolveProfile(profile)
		if err != nil {
			return err
		}
		socket = paths.Socket
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connection, err := dialDaemon(ctx, socket)
	if err != nil {
		return fmt.Errorf("connect to %s: %w (start WhatsAppGo, or pick another profile with --profile)", socket, err)
	}
	defer connection.Close()

	// The mouse is read as well as the keys: a click on a conversation, a
	// message or an item of a list does what the keys would do with it.
	program := tea.NewProgram(newApp(ctx, connection, profile),
		tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx))
	_, err = program.Run()
	return err
}
