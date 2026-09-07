package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/bugreport"
	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/store"
)

type captureReport struct {
	subject, body string
	err           error
}

func (r *captureReport) Submit(_ context.Context, subject, body string) (string, error) {
	r.subject, r.body = subject, body
	if r.err != nil {
		return "", r.err
	}
	return "https://tracker.example.test/WHATSAPPGO-1", nil
}

func TestBugReportDestinationAndDisclosure(t *testing.T) {
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "")
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN_FILE", "")
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, &fakeGateway{}, events.New())
	defer svc.Close()
	if _, ok := svc.reporter.(*bugreport.IntakeSubmitter); !ok {
		t.Fatal("default reporter is not the intake")
	}
	ctx := context.Background()
	result, err := svc.Handle(ctx, "bugreport.environment", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	env := result.(map[string]any)
	if env["program"] != "whatsappgo" || env["endpoint"] != bugreport.Endpoint || env["public_url"] != bugreport.PublicReportURL || env["authenticated_available"] != false {
		t.Fatalf("wrong destination: %v", env)
	}
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "test-secret-never-disclosed")
	result, err = svc.Handle(ctx, "bugreport.environment", json.RawMessage(`{}`))
	if err != nil || result.(map[string]any)["authenticated_available"] != true {
		t.Fatalf("configured intake unavailable: %v %v", result, err)
	}
	disclosure, _ := json.Marshal(result)
	if strings.Contains(string(disclosure), "test-secret-never-disclosed") {
		t.Fatal("credentials leaked in reporting capabilities")
	}
	reporter := &captureReport{}
	svc.reporter = reporter
	result, err = svc.Handle(ctx, "bugreport.submit", json.RawMessage(`{"subject":" A bug ","body":"Reproduction steps"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.(map[string]any)["url"] == "" || reporter.subject != "A bug" || !strings.HasPrefix(reporter.body, "Reproduction steps\n\n```") || !strings.Contains(reporter.body, "accounts:") {
		t.Fatalf("missing description or environment: %v %q", result, reporter.body)
	}
	reporter.err = errors.New("intake unavailable")
	result, err = svc.Handle(ctx, "bugreport.submit", json.RawMessage(`{"subject":"A bug","body":"Reproduction steps"}`))
	if !errors.Is(err, reporter.err) || result != nil {
		t.Fatalf("failure reported as success: %v %v", result, err)
	}
	// RPC clients cannot redirect reports or inject credentials.
	if _, err := svc.Handle(ctx, "bugreport.submit", json.RawMessage(`{"subject":"A bug","body":"steps","endpoint":"https://elsewhere.test"}`)); err == nil {
		t.Fatal("accepted arbitrary endpoint")
	}
}
