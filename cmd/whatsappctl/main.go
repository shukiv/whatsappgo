package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/shukiv/whatsappgo/internal/config"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/rpc"
)

var version = "dev"

type options struct {
	profile string
	socket  string
	timeout time.Duration
	pretty  bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		writeError(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	global := flag.NewFlagSet("whatsappctl", flag.ContinueOnError)
	global.SetOutput(stderr)
	var opts options
	global.StringVar(&opts.profile, "profile", "default", "account profile to control")
	global.StringVar(&opts.socket, "socket", "", "override the daemon Unix socket")
	global.DurationVar(&opts.timeout, "timeout", 30*time.Second, "RPC timeout")
	global.BoolVar(&opts.pretty, "pretty", false, "indent JSON output")
	showVersion := global.Bool("version", false, "print version and exit")
	global.Usage = func() { printUsage(stderr) }
	if err := global.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(stdout, version)
		return err
	}
	rest := global.Args()
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "--help" || rest[0] == "-h" {
		printUsage(stdout)
		return nil
	}
	if opts.timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	paths, err := config.ResolveProfile(opts.profile)
	if err != nil {
		return err
	}
	if opts.socket == "" {
		opts.socket = paths.Socket
	}

	command, commandArgs := rest[0], rest[1:]
	if command == "events" {
		return runEvents(ctx, opts, commandArgs, stdout, stderr)
	}

	callCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	client, err := rpc.Dial(callCtx, opts.socket)
	if err != nil {
		return fmt.Errorf("connect to %s: %w (start WhatsAppGo and select profile %q)", opts.socket, err, opts.profile)
	}
	defer client.Close()

	result, err := dispatch(callCtx, client, command, commandArgs, stdin, stderr)
	if err != nil {
		return err
	}
	return writeJSON(stdout, result, opts.pretty)
}

type caller interface {
	Call(context.Context, string, any) (json.RawMessage, error)
}

func dispatch(ctx context.Context, client caller, command string, args []string, stdin io.Reader, stderr io.Writer) (json.RawMessage, error) {
	switch command {
	case "status":
		return noArgCall(ctx, client, command, args, "status.get")
	case "discover":
		return noArgCall(ctx, client, command, args, "rpc.discover")
	case "chats":
		fs := commandFlags(command, stderr)
		query := fs.String("query", "", "filter chats by title or message text")
		limit := fs.Int("limit", 100, "maximum chats")
		offset := fs.Int("offset", 0, "number of chats to skip")
		archived := fs.Bool("archived", false, "list archived chats")
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() != 0 {
			return nil, fmt.Errorf("chats: unexpected argument %q", fs.Arg(0))
		}
		return client.Call(ctx, "chats.list", map[string]any{"query": *query, "limit": *limit, "offset": *offset, "archived": *archived})
	case "messages":
		fs := commandFlags(command, stderr)
		chat := fs.String("chat", "", "chat JID")
		before := fs.Int64("before", 0, "only messages before this Unix millisecond timestamp")
		limit := fs.Int("limit", 50, "maximum messages")
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if strings.TrimSpace(*chat) == "" {
			return nil, errors.New("messages: --chat is required")
		}
		return client.Call(ctx, "messages.list", map[string]any{"chat_jid": *chat, "before": *before, "limit": *limit})
	case "search":
		fs := commandFlags(command, stderr)
		limit := fs.Int("limit", 50, "maximum matches")
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		query := strings.TrimSpace(strings.Join(fs.Args(), " "))
		if query == "" {
			return nil, errors.New("search: a query is required")
		}
		return client.Call(ctx, "messages.search", map[string]any{"query": query, "limit": *limit})
	case "contact":
		return contactCommand(ctx, client, args, stderr)
	case "send":
		return sendCommand(ctx, client, args, stdin, stderr)
	case "download":
		fs := commandFlags(command, stderr)
		chat := fs.String("chat", "", "chat JID")
		message := fs.String("message", "", "message ID")
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if *chat == "" || *message == "" {
			return nil, errors.New("download: --chat and --message are required")
		}
		return client.Call(ctx, "message.download", map[string]any{"chat_jid": *chat, "message_id": *message})
	case "call":
		if len(args) == 0 || len(args) > 2 {
			return nil, errors.New("usage: whatsappctl call METHOD [JSON|-|@FILE]")
		}
		params := json.RawMessage(`{}`)
		if len(args) == 2 {
			parsed, err := readParams(args[1], stdin)
			if err != nil {
				return nil, err
			}
			params = parsed
		}
		return client.Call(ctx, args[0], params)
	default:
		return nil, fmt.Errorf("unknown command %q; run whatsappctl help", command)
	}
}

func noArgCall(ctx context.Context, client caller, command string, args []string, method string) (json.RawMessage, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("%s: unexpected argument %q", command, args[0])
	}
	return client.Call(ctx, method, map[string]any{})
}

func contactCommand(ctx context.Context, client caller, args []string, stderr io.Writer) (json.RawMessage, error) {
	fs := commandFlags("contact", stderr)
	phone := fs.String("phone", "", "international phone number")
	name := fs.String("name", "", "local WhatsAppGo label")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	clean := normalizePhone(*phone)
	if clean == "" {
		return nil, errors.New("contact: --phone is required and must contain digits")
	}
	if strings.TrimSpace(*name) == "" {
		return client.Call(ctx, "contact.resolve", map[string]any{"phone": clean})
	}
	return client.Call(ctx, "contact.save", map[string]any{"phone": clean, "name": strings.TrimSpace(*name)})
}

func sendCommand(ctx context.Context, client caller, args []string, stdin io.Reader, stderr io.Writer) (json.RawMessage, error) {
	fs := commandFlags("send", stderr)
	target := fs.String("to", "", "phone number or chat JID")
	textFlag := fs.String("text", "", "message text; use - to read stdin")
	file := fs.String("file", "", "file, image, video, or audio path")
	caption := fs.String("caption", "", "media caption")
	replyTo := fs.String("reply-to", "", "message ID to reply to")
	voice := fs.Bool("voice", false, "send audio as a voice note")
	noPreview := fs.Bool("no-preview", false, "do not attach an Open Graph preview")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(*target) == "" {
		return nil, errors.New("send: --to is required")
	}
	chatJID, err := resolveTarget(ctx, client, *target)
	if err != nil {
		return nil, err
	}
	if *file != "" {
		if *textFlag != "" || fs.NArg() != 0 {
			return nil, errors.New("send: use --caption, not text, when sending --file")
		}
		absolute, err := filepath.Abs(*file)
		if err != nil {
			return nil, fmt.Errorf("resolve media path: %w", err)
		}
		if info, err := os.Stat(absolute); err != nil {
			return nil, fmt.Errorf("open media: %w", err)
		} else if !info.Mode().IsRegular() {
			return nil, errors.New("send: --file must be a regular file")
		}
		return client.Call(ctx, "message.send_media", map[string]any{
			"chat_jid": chatJID, "path": absolute, "caption": *caption, "reply_to": *replyTo, "voice": *voice,
		})
	}
	if *voice {
		return nil, errors.New("send: --voice requires --file")
	}
	text, err := messageText(*textFlag, fs.Args(), stdin)
	if err != nil {
		return nil, err
	}
	params := map[string]any{"chat_jid": chatJID, "text": text, "reply_to": *replyTo, "link_preview": model.LinkPreview{}}
	if !*noPreview {
		previewRaw, previewErr := client.Call(ctx, "link.preview", map[string]any{"text": text})
		if previewErr == nil {
			var preview model.LinkPreview
			if json.Unmarshal(previewRaw, &preview) == nil {
				params["link_preview"] = preview
			}
		}
	}
	return client.Call(ctx, "message.send", params)
}

func resolveTarget(ctx context.Context, client caller, target string) (string, error) {
	target = strings.TrimSpace(target)
	if strings.Contains(target, "@") {
		return target, nil
	}
	phone := normalizePhone(target)
	if phone == "" {
		return "", errors.New("send: --to must be a chat JID or international phone number")
	}
	raw, err := client.Call(ctx, "contact.resolve", map[string]any{"phone": phone})
	if err != nil {
		return "", fmt.Errorf("resolve recipient: %w", err)
	}
	var chat model.Chat
	if err := json.Unmarshal(raw, &chat); err != nil {
		return "", fmt.Errorf("decode resolved recipient: %w", err)
	}
	if chat.JID == "" {
		return "", errors.New("resolved recipient has no chat JID")
	}
	return chat.JID, nil
}

func messageText(flagText string, args []string, stdin io.Reader) (string, error) {
	if flagText != "" && len(args) != 0 {
		return "", errors.New("send: provide text with either --text or trailing arguments, not both")
	}
	text := flagText
	if len(args) != 0 {
		text = strings.Join(args, " ")
	}
	if text == "-" || text == "" {
		if text == "" {
			if file, ok := stdin.(*os.File); ok {
				if info, err := file.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
					return "", errors.New("send: message text is required (or pipe it on stdin)")
				}
			}
		}
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read message text: %w", err)
		}
		text = strings.TrimSuffix(string(raw), "\n")
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("send: message text is empty")
	}
	return text, nil
}

func normalizePhone(phone string) string {
	var out strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) && r <= unicode.MaxASCII {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func readParams(source string, stdin io.Reader) (json.RawMessage, error) {
	var raw []byte
	var err error
	switch {
	case source == "-":
		raw, err = io.ReadAll(stdin)
	case strings.HasPrefix(source, "@"):
		if len(source) == 1 {
			return nil, errors.New("call: @ must be followed by a JSON file path")
		}
		raw, err = os.ReadFile(source[1:])
	default:
		raw = []byte(source)
	}
	if err != nil {
		return nil, fmt.Errorf("read call params: %w", err)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, errors.New("call: params must be valid JSON")
	}
	if raw[0] != '{' {
		return nil, errors.New("call: params must be a JSON object")
	}
	return json.RawMessage(raw), nil
}

func runEvents(ctx context.Context, opts options, args []string, stdout, stderr io.Writer) error {
	fs := commandFlags("events", stderr)
	var filters stringList
	fs.Var(&filters, "event", "event name to include; repeat for several")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("events: unexpected argument %q", fs.Arg(0))
	}
	dialCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	client, err := rpc.Dial(dialCtx, opts.socket)
	cancel()
	if err != nil {
		return fmt.Errorf("connect to %s: %w (start WhatsAppGo and select profile %q)", opts.socket, err, opts.profile)
	}
	defer client.Close()
	streamCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		<-streamCtx.Done()
		_ = client.Close()
	}()
	allowed := make(map[string]bool, len(filters))
	for _, name := range filters {
		allowed[name] = true
	}
	enc := json.NewEncoder(stdout)
	if opts.pretty {
		enc.SetIndent("", "  ")
	}
	err = client.Watch(streamCtx, func(evt rpc.Event) error {
		if len(allowed) != 0 && !allowed[evt.Event] {
			return nil
		}
		return enc.Encode(evt)
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("event name cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

func commandFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func writeJSON(w io.Writer, raw json.RawMessage, pretty bool) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`null`)
	}
	if pretty {
		var out bytes.Buffer
		if err := json.Indent(&out, raw, "", "  "); err != nil {
			return err
		}
		raw = out.Bytes()
	}
	_, err := fmt.Fprintln(w, string(raw))
	return err
}

func writeError(w io.Writer, err error) {
	code := "client_error"
	message := err.Error()
	var remote *rpc.RemoteError
	if errors.As(err, &remote) {
		code = remote.Code
		message = remote.Message
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: whatsappctl [global flags] COMMAND [arguments]

Control the app-owned WhatsAppGo backend. Output is JSON unless noted.

Global flags:
  --profile NAME       Account profile (default: default)
  --socket PATH        Override the Unix socket
  --timeout DURATION   RPC timeout (default: 30s)
  --pretty             Indent JSON output
  --version            Print version

Commands:
  status                         Connection and login state
  discover                       Machine-readable methods and events
  chats [flags]                  List or search chats
  messages --chat JID [flags]    Page through chat history
  search [--limit N] QUERY       Search locally stored messages
  contact --phone N [--name N]   Resolve a number; optionally save local label
  send --to N|JID [TEXT]         Send text or piped stdin
  send --to N|JID --file PATH    Send media or a document
  download --chat JID --message ID
  call METHOD [JSON|-|@FILE]     Call any method directly
  events [--event NAME]          Stream daemon events as JSON Lines

Run whatsappctl --pretty discover or read docs/API.md for the full API.`)
}
