package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/shukiv/whatsappgo/internal/rpc"
)

// The interface asks the daemon for several things at once as a matter of
// course, each on a goroutine of its own, and they all go down one connection.
// That connection carries one request at a time: rpc.Client reads through a
// single decoder and throws away any answer whose id is not the one it is
// waiting for. Asking twice at once used to take the program down with a panic
// inside the reader on the first screen it drew.
//
// Over a real socket, rather than an in-memory pipe that would serialise the
// two halves by itself and hide the fault. Loopback and not a Unix socket
// because a socket path is limited to around a hundred characters and a
// temporary directory named after this test is longer than that. Run under the
// race detector, the unlocked version reports the race outright.
func TestAskingForSeveralThingsAtOnceGoesDownOneConnectionInTurn(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	// A daemon that answers every request with the method it was asked for,
	// so an answer delivered to the wrong caller is visible as such.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		decoder := json.NewDecoder(conn)
		encoder := json.NewEncoder(conn)
		for {
			var request struct {
				ID     string `json:"id"`
				Method string `json:"method"`
			}
			if decoder.Decode(&request) != nil {
				return
			}
			answer, _ := json.Marshal(map[string]string{"method": request.Method})
			if encoder.Encode(map[string]any{"version": 1, "id": request.ID,
				"result": json.RawMessage(answer)}) != nil {
				return
			}
		}
	}()

	ours, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ours.Close() })

	under := &daemon{calls: rpc.NewClient(ours)}
	const askers = 16
	problems := make(chan error, askers)
	var waiting sync.WaitGroup
	for index := range askers {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			method := fmt.Sprintf("thing.%d", index)
			var answer struct {
				Method string `json:"method"`
			}
			if err := under.call(context.Background(), method, nil, &answer); err != nil {
				problems <- fmt.Errorf("%s: %w", method, err)
				return
			}
			if answer.Method != method {
				problems <- fmt.Errorf("%s was answered with %q", method, answer.Method)
			}
		}()
	}
	waiting.Wait()
	close(problems)
	for problem := range problems {
		t.Error(problem)
	}
}
