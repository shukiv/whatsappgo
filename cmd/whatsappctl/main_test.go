package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeCaller struct {
	methods []string
	params  []any
	results map[string]json.RawMessage
}

func (f *fakeCaller) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	f.methods = append(f.methods, method)
	f.params = append(f.params, params)
	if result := f.results[method]; result != nil {
		return result, nil
	}
	return json.RawMessage(`{"ok":true}`), nil
}

func TestNormalizePhone(t *testing.T) {
	if got := normalizePhone("+1 (415) 555-0123"); got != "14155550123" {
		t.Fatalf("normalizePhone() = %q", got)
	}
}

func TestReadParamsSources(t *testing.T) {
	got, err := readParams("-", strings.NewReader(" {\"value\":true}\n"))
	if err != nil || string(got) != `{"value":true}` {
		t.Fatalf("readParams() = %q, %v", got, err)
	}
	if _, err := readParams(`[1,2]`, strings.NewReader("")); err == nil {
		t.Fatal("expected non-object params to fail")
	}
}

func TestSendResolvesPhoneAndAddsPreview(t *testing.T) {
	fake := &fakeCaller{results: map[string]json.RawMessage{
		"contact.resolve": json.RawMessage(`{"jid":"15551234567@s.whatsapp.net"}`),
		"link.preview":    json.RawMessage(`{"url":"https://example.com","title":"Example"}`),
		"message.send":    json.RawMessage(`{"id":"sent"}`),
	}}
	result, err := sendCommand(context.Background(), fake, []string{"--to", "+1 555 123 4567", "see", "https://example.com"}, strings.NewReader(""), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != `{"id":"sent"}` {
		t.Fatalf("unexpected result %s", result)
	}
	want := []string{"contact.resolve", "link.preview", "message.send"}
	if strings.Join(fake.methods, ",") != strings.Join(want, ",") {
		t.Fatalf("methods = %v, want %v", fake.methods, want)
	}
	params := fake.params[2].(map[string]any)
	if params["chat_jid"] != "15551234567@s.whatsapp.net" || params["text"] != "see https://example.com" {
		t.Fatalf("send params = %#v", params)
	}
}

func TestSendStatusReplyIncludesStatusQuoteChat(t *testing.T) {
	fake := &fakeCaller{results: map[string]json.RawMessage{
		"message.send": json.RawMessage(`{"id":"sent"}`),
	}}
	_, err := sendCommand(context.Background(), fake, []string{
		"--to", "alice@lid", "--text", "Great photo", "--reply-to", "status-42",
		"--reply-chat", "status@broadcast", "--no-preview",
	}, strings.NewReader(""), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	params := fake.params[0].(map[string]any)
	if params["chat_jid"] != "alice@lid" || params["reply_to"] != "status-42" || params["reply_chat_jid"] != "status@broadcast" {
		t.Fatalf("status reply params = %#v", params)
	}
}

func TestMessageTextPreservesMultilineInput(t *testing.T) {
	got, err := messageText("-", nil, strings.NewReader("first\nsecond\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "first\nsecond" {
		t.Fatalf("messageText() = %q", got)
	}
}
