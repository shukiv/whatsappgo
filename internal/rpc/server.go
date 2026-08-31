package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	appEvents "github.com/shuki/whatsappgo/internal/events"
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
}

func NewServer(path string, handler Handler, broker *appEvents.Broker) *Server {
	return &Server{path: path, handler: handler, broker: broker, conns: make(map[net.Conn]struct{})}
}

func (s *Server) Listen() error {
	if err := removeStaleSocket(s.path); err != nil {
		return err
	}
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		ln.Close()
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
	for {
		var req Request
		_ = conn.SetReadDeadline(time.Time{})
		if err := dec.Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				_ = write(Response{Version: ProtocolVersion, Error: &Error{Code: "invalid_request", Message: err.Error()}})
			}
			return
		}
		resp := Response{Version: ProtocolVersion, ID: req.ID}
		if req.Version != ProtocolVersion {
			resp.Error = &Error{Code: "unsupported_version", Message: "unsupported protocol version"}
		} else if req.ID == "" || req.Method == "" {
			resp.Error = &Error{Code: "invalid_request", Message: "id and method are required"}
		} else {
			result, err := s.handler.Handle(ctx, req.Method, req.Params)
			if err != nil {
				resp.Error = &Error{Code: "request_failed", Message: err.Error()}
			} else {
				resp.Result = result
			}
		}
		if err := write(resp); err != nil {
			return
		}
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("refusing to replace non-socket path: " + path)
	}
	conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err == nil {
		conn.Close()
		return errors.New("daemon is already listening on " + path)
	}
	return os.Remove(path)
}
