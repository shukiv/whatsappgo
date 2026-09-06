package bugreport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestIntakePayloadAndAuthentication(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "test-intake-key")
			const issueURL = "http://192.168.100.100:8090/bug-reports/browse/WHATSAPPGO-1/"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/intake" || r.Header.Get("Authorization") != "Bearer test-intake-key" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("incorrect method, route or authentication")
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if payload["program"] != "whatsappgo" || payload["source"] != "whatsappgo" || payload["title"] != "Test report" || payload["description"] != "Body; $(not a command) שלום" {
					t.Errorf("incorrect payload: %v", payload)
				}
				for _, key := range []string{"logs", "reporter", "fingerprint", "token"} {
					if _, exists := payload[key]; exists {
						t.Errorf("unexpected field %s", key)
					}
				}
				w.WriteHeader(status)
				action := "created"
				if status == http.StatusOK {
					action = "commented"
				}
				fmt.Fprintf(w, `{"ok":true,"data":{"action":%q,"program":"whatsappgo","url":%q,"warnings":["label skipped"]}}`, action, issueURL)
			}))
			defer server.Close()
			s := NewIntakeSubmitter()
			s.endpoint = server.URL + "/api/v1/intake"
			url, err := s.Submit(context.Background(), "Test report", "Body; $(not a command) שלום")
			if err != nil || url != issueURL {
				t.Fatalf("url=%q err=%v", url, err)
			}
		})
	}
}

func TestIntakeRejectsErrorsAndUnconfirmedResponses(t *testing.T) {
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "secret-not-for-errors")
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"auth", 401, `{"ok":false,"error":"secret-not-for-errors"}`, "key"},
		{"rate", 429, `{"ok":false,"error":"rate limited"}`, "rate"},
		{"upstream", 502, `{"ok":false,"error":"upstream failure"}`, "502"},
		{"false-success", 201, `{"ok":false}`, "confirm"},
		{"malformed", 201, `<html>proxy</html>`, "confirm"},
		{"wrong-project", 201, `{"ok":true,"data":{"action":"created","program":"jabali-panel","url":"https://example.test/1"}}`, "confirm"},
		{"unsafe-url", 201, `{"ok":true,"data":{"action":"created","program":"whatsappgo","url":"file:///tmp/anything"}}`, "confirm"},
		{"empty-url", 201, `{"ok":true,"data":{"action":"created","program":"whatsappgo"}}`, "confirm"},
		{"oversized", 201, strings.Repeat("x", 65537), "confirm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			s := NewIntakeSubmitter()
			s.endpoint = server.URL
			url, err := s.Submit(context.Background(), "Title", "Body")
			if err == nil || url != "" || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "secret-not-for-errors") || calls != 1 {
				t.Fatalf("url=%q err=%v calls=%d", url, err, calls)
			}
		})
	}
}

func TestIntakeDoesNotFollowRedirects(t *testing.T) {
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "test-intake-key")
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	s := NewIntakeSubmitter()
	s.endpoint = server.URL
	if _, err := s.Submit(context.Background(), "Title", "Body"); err == nil || leaked {
		t.Fatalf("redirect: err=%v leaked=%v", err, leaked)
	}
}

func TestIntakeKeyConfigurationAndCancellation(t *testing.T) {
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "")
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN_FILE", "")
	if _, err := NewIntakeSubmitter().Submit(context.Background(), "Title", "Body"); !errors.Is(err, ErrNoToken) {
		t.Fatalf("err=%v", err)
	}
	path := filepath.Join(t.TempDir(), "intake-key")
	if err := os.WriteFile(path, []byte(" key-from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN_FILE", path)
	key, err := intakeToken()
	if err != nil || key != "key-from-file" {
		t.Fatalf("key file error: %v", err)
	}
	t.Setenv("WHATSAPPGO_BUGREPORT_TOKEN", "key-from-environment")
	key, err = intakeToken()
	if err != nil || key != "key-from-environment" {
		t.Fatalf("environment precedence error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewIntakeSubmitter().Submit(ctx, "Title", "Body"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateKeepsUTF8Boundaries(t *testing.T) {
	subject, body, err := Validate(strings.Repeat("a", maxSubject-1)+"ש", strings.Repeat("b", maxBody-1)+"😀")
	if err != nil || !utf8.ValidString(subject) || !utf8.ValidString(body) {
		t.Fatalf("split UTF-8: %v", err)
	}
}
