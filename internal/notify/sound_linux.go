//go:build linux

package notify

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

var soundSlot = make(chan struct{}, 1)

// PlaySound uses the installed desktop sound theme, never a downloaded command
// or user-provided path. One player at a time bounds bursts of sent messages.
func PlaySound(ctx context.Context, kind string) error {
	file := "message-new-instant.oga"
	switch kind {
	case "incoming":
	case "outgoing":
		file = "message.oga"
	default:
		return errors.New("unknown sound")
	}
	const player = "/usr/bin/paplay"
	path := "/usr/share/sounds/freedesktop/stereo/" + file
	if !isTrustedExecutable(player) || notificationImagePath(path) == "" {
		return errors.New("sound playback needs PulseAudio's paplay and the freedesktop sound theme")
	}
	select {
	case soundSlot <- struct{}{}:
		defer func() { <-soundSlot }()
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, player, "--client-name=WhatsAppGo", path).Run(); err != nil {
		return errors.New("could not play sound; check the desktop audio output and volume")
	}
	return nil
}
