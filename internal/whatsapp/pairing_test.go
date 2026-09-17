package whatsapp

import (
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"

	"github.com/shukiv/whatsappgo/internal/gateway"
)

// Pairing is answered in one call and finished minutes later, on the phone. A
// client that asks to pair and closes its connection - the command line does,
// and so does the MCP server - must not take the exchange down with it.
func TestPairingOutlivesTheRequestThatAskedForIt(t *testing.T) {
	daemon, shutdown := context.WithCancel(context.Background())
	defer shutdown()
	client := &Client{baseCtx: daemon}

	_, answered := context.WithCancel(context.Background())
	lifetime := client.pairingLifetime()
	answered()

	if err := lifetime.Err(); err != nil {
		t.Fatalf("answering the request ended pairing: %v", err)
	}
	shutdown()
	if lifetime.Err() == nil {
		t.Fatal("pairing outlived the daemon it belongs to")
	}
}

// A daemon built without a lifetime still has to be able to pair.
func TestPairingHasALifetimeEvenWithoutADaemonContext(t *testing.T) {
	client := &Client{}
	if lifetime := client.pairingLifetime(); lifetime == nil || lifetime.Err() != nil {
		t.Fatalf("pairing had no context to run under: %#v", lifetime)
	}
}

// What the exchange produces is reported: the code to scan, and the phone
// accepting it.
func TestPairingReportsEachThingTheExchangeProduces(t *testing.T) {
	client := &Client{subs: map[uint64]func(gateway.Event){}, pairing: true}
	var seen []gateway.Event
	client.Subscribe(func(evt gateway.Event) { seen = append(seen, evt) })

	codes := make(chan whatsmeow.QRChannelItem, 3)
	codes <- whatsmeow.QRChannelItem{Event: "code", Code: "2@abcd", Timeout: 20 * time.Second}
	codes <- whatsmeow.QRChannelItem{Event: "success"}
	close(codes)
	client.consumeQR(context.Background(), codes)

	if len(seen) != 2 {
		t.Fatalf("the exchange was not reported: %#v", seen)
	}
	if seen[0].Name != "pairing.qr" {
		t.Fatalf("the code to scan was not reported: %#v", seen[0])
	}
	drawn, ok := seen[0].Data.(map[string]any)
	if !ok || drawn["code"] != "2@abcd" {
		t.Fatalf("the code was not the one the exchange produced: %#v", seen[0].Data)
	}
	if picture, ok := drawn["png_base64"].(string); !ok || picture == "" {
		t.Fatalf("the code was reported without a picture of it: %#v", drawn)
	}
	if drawn["expires_in"] != 20 {
		t.Fatalf("the code was reported without saying when it expires: %#v", drawn)
	}
	if seen[1].Name != "pairing.success" {
		t.Fatalf("the phone accepting was not reported: %#v", seen[1])
	}
	// Pairing is over, so another attempt is allowed to start.
	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.pairing {
		t.Fatal("a finished exchange still counts as one in progress")
	}
}

// An exchange that ends because the daemon is going down reports nothing: the
// events would have nowhere to go.
func TestPairingStopsWhenTheDaemonDoes(t *testing.T) {
	client := &Client{subs: map[uint64]func(gateway.Event){}}
	var seen []gateway.Event
	client.Subscribe(func(evt gateway.Event) { seen = append(seen, evt) })

	stopped, shutdown := context.WithCancel(context.Background())
	shutdown()
	codes := make(chan whatsmeow.QRChannelItem, 1)
	codes <- whatsmeow.QRChannelItem{Event: "code", Code: "2@abcd", Timeout: 20 * time.Second}
	client.consumeQR(stopped, codes)

	if len(seen) != 0 {
		t.Fatalf("a stopped exchange still reported: %#v", seen)
	}
}
