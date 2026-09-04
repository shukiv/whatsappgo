package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	appEvents "github.com/shukiv/whatsappgo/internal/events"
)

type Handler interface {
	Handle(context.Context, string, json.RawMessage) (any, error)
}

type Server struct {
	path      string
	handler   Handler
	broker    *appEvents.Broker
	listener  net.Listener
	wg        sync.WaitGroup
	closeOnce sync.Once
	connMu    sync.Mutex
	conns     map[net.Conn]struct{}

	// socketCheckInterval overrides how often Serve confirms it still owns
	// its socket. Zero means the default; only tests set it, and only before
	// Serve starts, so the watcher goroutine never races the writer.
	socketCheckInterval time.Duration
}

func NewServer(path string, handler Handler, broker *appEvents.Broker) *Server {
	return &Server{path: path, handler: handler, broker: broker, conns: make(map[net.Conn]struct{})}
}

func (s *Server) Listen() error {
	ln, err := listen(s.path)
	if err != nil {
		return err
	}
	s.listener = ln
	return nil
}

func (s *Server) Serve(ctx context.Context) error {
	if s.listener == nil {
		if err := s.Listen(); err != nil {
			return err
		}
	}
	go func() { <-ctx.Done(); s.Close() }()
	// A daemon whose socket was taken away can no longer be reached, but it
	// keeps running and keeps its databases open. Stop instead, so the next
	// client start-up replaces it rather than stacking another one on top.
	replaced := watchListener(ctx, s.path, s.socketCheckInterval)
	go func() {
		select {
		case <-ctx.Done():
		case <-replaced:
			log.Printf("stopping: another process replaced %s", s.path)
			s.Close()
		}
	}()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			return err
		}
		s.connMu.Lock()
		s.conns[conn] = struct{}{}
		s.connMu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { s.connMu.Lock(); delete(s.conns, conn); s.connMu.Unlock() }()
			s.serveConn(ctx, conn)
		}()
	}
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.listener != nil {
			err = s.listener.Close()
		}
		s.connMu.Lock()
		for conn := range s.conns {
			_ = conn.Close()
		}
		s.connMu.Unlock()
		s.wg.Wait()
		if s.path != "" {
			_ = os.Remove(s.path)
		}
	})
	return err
}

// maxConcurrentRequests bounds the work one connection can have running. It is
// large enough that a search never waits behind the avatar and media requests a
// scrolling list produces, and small enough to keep a runaway client from
// starting unbounded work.
const maxConcurrentRequests = 8

func (s *Server) serveConn(parent context.Context, conn net.Conn) {
	defer conn.Close()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	events, unsub := s.broker.Subscribe(128)
	defer unsub()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(bufio.NewReaderSize(conn, 64*1024))
	var writeMu sync.Mutex
	write := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return enc.Encode(v)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				if write(Event{Version: ProtocolVersion, Event: evt.Name, Data: evt.Data}) != nil {
					cancel()
					return
				}
			}
		}
	}()
	// Requests are handled off the read loop so that a slow one - a media
	// download, a history page - does not hold up every request behind it. The
	// desktop matches replies to requests by id, so order is free to change.
	// The slot count bounds the work a burst of requests can start at once.
	slots := make(chan struct{}, maxConcurrentRequests)
	var inFlight sync.WaitGroup
	defer inFlight.Wait()
	for {
		var req Request
		_ = conn.SetReadDeadline(time.Time{})
		if err := dec.Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				_ = write(Response{Version: ProtocolVersion, Error: &Error{Code: "invalid_request", Message: err.Error()}})
			}
			return
		}
		if req.Version != ProtocolVersion || req.ID == "" || req.Method == "" {
			resp := Response{Version: ProtocolVersion, ID: req.ID}
			if req.Version != ProtocolVersion {
				resp.Error = &Error{Code: "unsupported_version", Message: "unsupported protocol version"}
			} else {
				resp.Error = &Error{Code: "invalid_request", Message: "id and method are required"}
			}
			if err := write(resp); err != nil {
				return
			}
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		inFlight.Add(1)
		go func(req Request) {
			defer inFlight.Done()
			defer func() { <-slots }()
			resp := Response{Version: ProtocolVersion, ID: req.ID}
			result, err := s.handler.Handle(ctx, req.Method, req.Params)
			if err != nil {
				resp.Error = &Error{Code: "request_failed", Message: err.Error()}
			} else {
				resp.Result = result
			}
			if write(resp) != nil {
				cancel()
			}
		}(req)
	}
}
