package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

// Client is a small JSON-lines client for whatsappd's owner-only Unix socket.
// It is shared by command-line automation and tests; the desktop keeps its Qt
// implementation so it can integrate with the Qt event loop.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	next atomic.Uint64
}

func Dial(ctx context.Context, path string) (*Client, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(bufio.NewReaderSize(conn, 64*1024)),
	}
}

func (c *Client) Close() error { return c.conn.Close() }

// RemoteError is an error returned by the daemon rather than a transport
// failure. Code is stable enough for scripts to branch on.
type RemoteError struct {
	Code    string
	Message string
}

func (e *RemoteError) Error() string { return e.Code + ": " + e.Message }

// Call sends one request and waits for its matching response. Events can be
// interleaved by the server and are skipped here; event consumers use Watch.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := fmt.Sprintf("ctl-%d", c.next.Add(1))
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode params: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(deadline)
		defer c.conn.SetDeadline(time.Time{})
	}
	if err := c.enc.Encode(Request{Version: ProtocolVersion, ID: id, Method: method, Params: raw}); err != nil {
		return nil, err
	}
	for {
		var envelope struct {
			Version int             `json:"version"`
			ID      string          `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   *Error          `json:"error"`
			Event   string          `json:"event"`
		}
		if err := c.dec.Decode(&envelope); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if envelope.ID != id {
			continue
		}
		if envelope.Error != nil {
			return nil, &RemoteError{Code: envelope.Error.Code, Message: envelope.Error.Message}
		}
		return envelope.Result, nil
	}
}

// Watch receives daemon events until the context is cancelled or fn returns
// an error. Closing the client from a signal handler promptly unblocks Decode.
func (c *Client) Watch(ctx context.Context, fn func(Event) error) error {
	for {
		var evt Event
		if err := c.dec.Decode(&evt); err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return ctx.Err()
			}
			return err
		}
		if evt.Event == "" {
			continue
		}
		if err := fn(evt); err != nil {
			return err
		}
	}
}
