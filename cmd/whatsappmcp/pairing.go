package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// Linking an account is the one thing the daemon cannot answer in a single
// call: pairing.start returns nothing but an acknowledgement, and the code to
// scan arrives afterwards as a pairing.qr event. A client that can only call
// methods would never see it, so these two tools call and wait, and hand back
// the code as both a picture and something a terminal can draw.
//
// pairing.phone needs none of this: it answers with the code to type, and is
// offered as it comes from the daemon.
var builtinTools = []tool{
	{
		Name: "pairing_qr",
		Description: "[mutating] Link this profile to a WhatsApp account: start QR pairing and " +
			"return the current code as a PNG and as text a terminal can draw. The code expires, " +
			"so call pairing_wait afterwards to be given the next one or told the phone accepted it. " +
			"Scanning happens on the phone: WhatsApp, Linked devices, Link a device.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"timeout_seconds": map[string]any{"type": "integer",
				"description": "How long to wait for the first code (default 60, at most 180)"},
		}},
	},
	{
		Name: "pairing_wait",
		Description: "[mutating] Wait for what pairing does next: the phone accepted the code " +
			"(paired), the code expired and was replaced (qr, with the new one), or pairing failed " +
			"(error). Call this after pairing_qr or pairing_phone.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"timeout_seconds": map[string]any{"type": "integer",
				"description": "How long to wait (default 120, at most 180)"},
		}},
	},
}

// waitCeiling bounds what a client can ask this server to block for. A tool
// call that never returns is worse than one that returns "nothing yet".
const waitCeiling = 180 * time.Second

func waitFor(arguments map[string]any, fallback time.Duration) time.Duration {
	seconds, ok := arguments["timeout_seconds"].(float64)
	if !ok || seconds <= 0 {
		return fallback
	}
	asked := time.Duration(seconds) * time.Second
	if asked > waitCeiling {
		return waitCeiling
	}
	return asked
}

// status reports whether the profile is already linked, so neither tool starts
// pairing an account that has nothing to pair.
func (s *server) status(ctx context.Context, timeout time.Duration) (bool, string, error) {
	raw, err := s.call(ctx, "status.get", map[string]any{}, timeout)
	if err != nil {
		return false, "", err
	}
	var reported struct {
		State    string `json:"state"`
		LoggedIn bool   `json:"logged_in"`
		UserJID  string `json:"user_jid"`
	}
	if err := json.Unmarshal(raw, &reported); err != nil {
		return false, "", fmt.Errorf("read the connection state: %w", err)
	}
	if reported.UserJID != "" {
		return reported.LoggedIn, reported.State + " as " + reported.UserJID, nil
	}
	return reported.LoggedIn, reported.State, nil
}

// watchPairing subscribes to the daemon's events before anything is started.
// Every connection receives every event, so subscribing is opening one; and it
// has to happen first, because a code emitted between the call and the
// subscription is a code nobody sees.
func (s *server) watchPairing(ctx context.Context) (<-chan rpc.Event, func(), error) {
	client, err := s.dial(ctx)
	if err != nil {
		return nil, nil, err
	}
	watched, cancel := context.WithCancel(ctx)
	events := make(chan rpc.Event, 8)
	go func() {
		defer close(events)
		defer client.Close()
		_ = client.Watch(watched, func(evt rpc.Event) error {
			if !strings.HasPrefix(evt.Event, "pairing.") {
				return nil
			}
			select {
			case events <- evt:
			case <-watched.Done():
				return watched.Err()
			}
			return nil
		})
	}()
	return events, func() {
		cancel()
		// Watch blocks in Decode until the connection is closed, which
		// cancelling alone does not do.
		client.Close()
	}, nil
}

func (s *server) pairingQR(ctx context.Context, arguments map[string]any, timeout time.Duration) map[string]any {
	linked, state, err := s.status(ctx, timeout)
	if err != nil {
		return textResult(err.Error(), true)
	}
	if linked {
		return textResult("This profile is already linked ("+state+"). Unlink it with account_logout before pairing again.", false)
	}
	events, stop, err := s.watchPairing(ctx)
	if err != nil {
		return textResult(err.Error(), true)
	}
	defer stop()
	if _, err := s.call(ctx, "pairing.start", map[string]any{}, timeout); err != nil {
		// Pairing already running is not a failure here: its next code is
		// exactly what this call is waiting for.
		if !strings.Contains(err.Error(), "already in progress") {
			return textResult(err.Error(), true)
		}
	}
	return awaitPairing(ctx, events, waitFor(arguments, 60*time.Second))
}

func (s *server) pairingWait(ctx context.Context, arguments map[string]any, timeout time.Duration) map[string]any {
	linked, state, err := s.status(ctx, timeout)
	if err != nil {
		return textResult(err.Error(), true)
	}
	if linked {
		return textResult("paired: this profile is linked ("+state+").", false)
	}
	events, stop, err := s.watchPairing(ctx)
	if err != nil {
		return textResult(err.Error(), true)
	}
	defer stop()
	return awaitPairing(ctx, events, waitFor(arguments, 120*time.Second))
}

// awaitPairing returns on the first pairing event that tells the caller
// something it can act on. A pairing.state event is progress, not an outcome,
// so it is not one of them.
func awaitPairing(ctx context.Context, events <-chan rpc.Event, wait time.Duration) map[string]any {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return textResult("waiting for pairing was cut short: "+ctx.Err().Error(), true)
		case <-deadline.C:
			return textResult(fmt.Sprintf("nothing happened within %s. Pairing is still running: "+
				"call pairing_wait again, or pairing_qr for a fresh code.", wait), false)
		case evt, open := <-events:
			if !open {
				return textResult("the connection to the daemon closed while waiting for pairing.", true)
			}
			switch evt.Event {
			case "pairing.qr":
				result, err := qrResult(evt.Data)
				if err != nil {
					return textResult(err.Error(), true)
				}
				return result
			case "pairing.success":
				return textResult("paired: the phone accepted the code. Call status_get to see the account.", false)
			case "pairing.error":
				return textResult("error: "+pairingMessage(evt.Data), true)
			}
		}
	}
}

func pairingMessage(data any) string {
	fields, ok := data.(map[string]any)
	if !ok {
		return "pairing failed"
	}
	if message, ok := fields["message"].(string); ok && message != "" {
		return message
	}
	return "pairing failed"
}

// qrResult turns one pairing.qr event into what a client can show: the PNG the
// daemon drew, and the same code as text, for a client that renders no images.
func qrResult(data any) (map[string]any, error) {
	fields, ok := data.(map[string]any)
	if !ok {
		return nil, errors.New("the daemon sent a QR code this could not read")
	}
	code, _ := fields["code"].(string)
	if code == "" {
		return nil, errors.New("the daemon sent a QR code with no content")
	}
	expires := ""
	if seconds, ok := fields["expires_in"].(float64); ok && seconds > 0 {
		expires = fmt.Sprintf(" It expires in %d seconds; pairing_wait returns the next one.", int(seconds))
	}
	content := []any{}
	if png, ok := fields["png_base64"].(string); ok && png != "" {
		content = append(content, map[string]any{"type": "image", "data": png, "mimeType": "image/png"})
	}
	drawn := ""
	if rendered, err := qrcode.New(code, qrcode.Medium); err == nil {
		drawn = "\n" + rendered.ToSmallString(false)
	}
	content = append(content, map[string]any{"type": "text", "text": fmt.Sprintf(
		"qr: scan this on the phone under WhatsApp, Linked devices, Link a device.%s%s\n\ncode: %s",
		expires, drawn, code)})
	return map[string]any{"content": content, "isError": false}, nil
}
