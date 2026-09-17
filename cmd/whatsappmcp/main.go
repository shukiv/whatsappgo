// Command whatsappmcp exposes the daemon's RPC surface to Model Context
// Protocol clients.
//
// It defines no tool list of its own. On the first listing it asks the daemon
// what it serves through rpc.discover and turns each answer into a tool, so a
// method added to the daemon is callable here without touching this file, and
// one removed stops being offered.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shukiv/whatsappgo/internal/config"
	"github.com/shukiv/whatsappgo/internal/rpc"
)

var version = "dev"

// The revision of the protocol this server speaks. A client asking for another
// revision is answered with this one, which is what the specification asks a
// server to do when it cannot speak what was requested.
const protocolVersion = "2025-06-18"

type caller interface {
	Call(context.Context, string, any) (json.RawMessage, error)
	// Watch delivers the daemon's events. Call skips them, so pairing - the
	// one exchange whose answer arrives as an event - is read from here.
	Watch(context.Context, func(rpc.Event) error) error
	Close() error
}

type methodDescription struct {
	Name     string         `json:"name"`
	Summary  string         `json:"summary"`
	Mutating bool           `json:"mutating"`
	Params   map[string]any `json:"params_example"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`

	method string
}

type server struct {
	dial func(context.Context) (caller, error)

	mu     sync.Mutex
	tools  []tool
	byTool map[string]string
}

func main() {
	profile := flag.String("profile", "default", "account profile to control")
	socket := flag.String("socket", "", "override the daemon Unix socket")
	daemon := flag.String("daemon", "", "path to whatsappd (default: beside this binary, else PATH)")
	importFrom := flag.String("import", "", "import a profile archive into --profile, then exit")
	force := flag.Bool("force", false, "with --import, replace a profile that already holds databases")
	noStart := flag.Bool("no-start", false, "never start a daemon; fail if none is listening")
	timeout := flag.Duration("timeout", 30*time.Second, "RPC timeout for a single call")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *importFrom != "" {
		if err := importProfile(*importFrom, *profile, *force); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	address := *socket
	if address == "" {
		paths, err := config.ResolveProfile(*profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		address = paths.Socket
	}
	// The notice goes to stderr: stdout carries the protocol and nothing else.
	fmt.Fprintf(os.Stderr, "whatsappgo mcp %s: profile %q, socket %s\n", version, *profile, address)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	launch := newLauncher(address, *profile, *daemon)
	s := newServer(func(ctx context.Context) (caller, error) {
		if *noStart {
			client, err := dialOnce(ctx, address, *timeout)
			if err != nil {
				return nil, fmt.Errorf("connect to %s: %w (start WhatsAppGo and select profile %q)", address, err, *profile)
			}
			return client, nil
		}
		// Nothing listening means no daemon for this profile, so one is
		// started headless. A profile the desktop already runs is reached, not
		// duplicated: two daemons would fight over the same files.
		return launch.connect(ctx, *timeout)
	})
	if err := s.serve(ctx, os.Stdin, os.Stdout, *timeout); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newServer(dial func(context.Context) (caller, error)) *server {
	return &server{dial: dial, byTool: map[string]string{}}
}

// A message on the wire. A request carries an id and expects an answer; a
// notification carries none and must not be answered at all.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// serve reads one JSON object per line, which is how the stdio transport
// frames messages, and answers on the same stream.
func (s *server) serve(ctx context.Context, in io.Reader, out io.Writer, timeout time.Duration) error {
	reader := bufio.NewReaderSize(in, 1<<20)
	encoder := json.NewEncoder(out)
	for {
		line, err := reader.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			if reply := s.handle(ctx, line, timeout); reply != nil {
				if writeErr := encoder.Encode(reply); writeErr != nil {
					return writeErr
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// handle answers one message, or returns nil for a notification, which the
// specification says must never be answered.
func (s *server) handle(ctx context.Context, line []byte, timeout time.Duration) *response {
	var msg message
	if err := json.Unmarshal(line, &msg); err != nil {
		return &response{JSONRPC: "2.0", ID: json.RawMessage("null"),
			Error: &rpcError{Code: -32700, Message: "invalid JSON: " + err.Error()}}
	}
	notification := len(msg.ID) == 0
	reply := func(result any, failure *rpcError) *response {
		if notification {
			return nil
		}
		return &response{JSONRPC: "2.0", ID: msg.ID, Result: result, Error: failure}
	}
	switch msg.Method {
	case "initialize":
		return reply(map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "whatsappgo", "version": version},
			"instructions": "Every tool is one WhatsApp RPC method, named with underscores where the " +
				"method uses dots. Tools marked [mutating] change the account: they send, delete or " +
				"reconfigure. Ask before calling one. If status_get reports logged_in false, the " +
				"profile is not linked to an account yet: pair it with pairing_qr (scan the code on " +
				"the phone) or pairing_phone (type the code on the phone), then pairing_wait.",
		}, nil)
	case "tools/list":
		tools, err := s.listTools(ctx, timeout)
		if err != nil {
			return reply(nil, &rpcError{Code: -32603, Message: err.Error()})
		}
		return reply(map[string]any{"tools": tools}, nil)
	case "tools/call":
		result, err := s.callTool(ctx, msg.Params, timeout)
		if err != nil {
			return reply(nil, &rpcError{Code: -32602, Message: err.Error()})
		}
		return reply(result, nil)
	case "ping":
		return reply(map[string]any{}, nil)
	default:
		if notification {
			// Initialized, cancellations and progress need no answer.
			return nil
		}
		return reply(nil, &rpcError{Code: -32601, Message: "unknown method " + msg.Method})
	}
}

// listTools asks the daemon what it serves. The answer is kept once it
// arrives, but a failure is not: a client that listed while the daemon was
// down would otherwise be left with an empty tool set for good.
func (s *server) listTools(ctx context.Context, timeout time.Duration) ([]tool, error) {
	s.mu.Lock()
	cached := s.tools
	s.mu.Unlock()
	if len(cached) > 0 {
		return cached, nil
	}
	raw, err := s.call(ctx, "rpc.discover", map[string]any{}, timeout)
	if err != nil {
		return nil, err
	}
	var discovery struct {
		Methods []methodDescription `json:"methods"`
	}
	if err := json.Unmarshal(raw, &discovery); err != nil {
		return nil, fmt.Errorf("read the method list: %w", err)
	}
	if len(discovery.Methods) == 0 {
		return nil, errors.New("the daemon listed no methods")
	}
	tools := make([]tool, 0, len(discovery.Methods)+len(builtinTools))
	byTool := make(map[string]string, len(discovery.Methods)+len(builtinTools))
	for _, builtin := range builtinTools {
		tools = append(tools, builtin)
		byTool[builtin.Name] = ""
	}
	for _, described := range discovery.Methods {
		name := toolName(described.Name)
		if _, taken := byTool[name]; taken {
			continue
		}
		byTool[name] = described.Name
		tools = append(tools, tool{
			Name:        name,
			Description: describe(described),
			InputSchema: schemaFor(described.Params),
			method:      described.Name,
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	s.mu.Lock()
	s.tools, s.byTool = tools, byTool
	s.mu.Unlock()
	return tools, nil
}

func (s *server) callTool(ctx context.Context, raw json.RawMessage, timeout time.Duration) (map[string]any, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("read the call: %w", err)
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, errors.New("name is required")
	}
	if _, err := s.listTools(ctx, timeout); err != nil {
		return nil, err
	}
	s.mu.Lock()
	method, known := s.byTool[params.Name]
	s.mu.Unlock()
	if !known {
		return nil, fmt.Errorf("unknown tool %q", params.Name)
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	// A built-in stands for no single method: it calls several, and waits on
	// the events between them.
	switch params.Name {
	case "pairing_qr":
		return s.pairingQR(ctx, params.Arguments, timeout), nil
	case "pairing_wait":
		return s.pairingWait(ctx, params.Arguments, timeout), nil
	}
	result, err := s.call(ctx, method, params.Arguments, timeout)
	if err != nil {
		// A refusal by the daemon is an answer to the caller, not a transport
		// failure: it is reported inside the result so the model can read what
		// went wrong and correct the call.
		return textResult(err.Error(), true), nil
	}
	return textResult(pretty(result), false), nil
}

func (s *server) call(ctx context.Context, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	client, err := s.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.Call(callCtx, method, params)
}

func textResult(text string, failed bool) map[string]any {
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
		"isError": failed,
	}
}

func pretty(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, raw, "", "  "); err != nil {
		return string(raw)
	}
	return indented.String()
}

// toolName is the method name in the shape tool names take. Dots are the only
// character a method uses that clients are not required to accept.
func toolName(method string) string {
	return strings.ReplaceAll(method, ".", "_")
}

func describe(described methodDescription) string {
	summary := strings.TrimSpace(described.Summary)
	if summary == "" {
		summary = described.Name
	}
	prefix := ""
	if described.Mutating {
		prefix = "[mutating] "
	}
	return fmt.Sprintf("%s%s (RPC method %s)", prefix, summary, described.Name)
}

// schemaFor describes the parameters from the example the daemon publishes.
// The example is the only description of a method's arguments there is, so the
// types come from the values in it; nothing is marked required, because the
// daemon validates and says what it wants in a message the caller can read.
func schemaFor(example map[string]any) map[string]any {
	properties := map[string]any{}
	for key, value := range example {
		properties[key] = schemaForValue(value)
	}
	return map[string]any{"type": "object", "properties": properties}
}

func schemaForValue(value any) map[string]any {
	switch typed := value.(type) {
	case string:
		return map[string]any{"type": "string"}
	case bool:
		return map[string]any{"type": "boolean"}
	case float64:
		if typed == float64(int64(typed)) {
			return map[string]any{"type": "integer"}
		}
		return map[string]any{"type": "number"}
	case int, int32, int64:
		return map[string]any{"type": "integer"}
	case []any:
		if len(typed) > 0 {
			return map[string]any{"type": "array", "items": schemaForValue(typed[0])}
		}
		return map[string]any{"type": "array"}
	case []string:
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	case map[string]any:
		return map[string]any{"type": "object"}
	default:
		// An example the daemon wrote as null says a value is expected without
		// saying of what kind. Describing it as any keeps the call possible.
		return map[string]any{}
	}
}
