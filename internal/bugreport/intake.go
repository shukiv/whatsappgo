package bugreport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	Program          = "whatsappgo"
	Endpoint         = "https://bugs.jabali-panel.com/api/v1/intake"
	PublicReportURL  = "https://bugs.jabali-panel.com/report"
	maxResponseBytes = 64 * 1024
)

var ErrNoToken = errors.New("bug reporting needs an intake key: configure WHATSAPPGO_BUGREPORT_TOKEN_FILE or WHATSAPPGO_BUGREPORT_TOKEN for the daemon")

// AuthenticatedAvailable reveals only whether local credentials can be read,
// not their value or path. The public form remains usable without a daemon.
func AuthenticatedAvailable() bool {
	_, err := intakeToken()
	return err == nil
}

// IntakeSubmitter has one fixed destination and project. Credentials come from
// the daemon's runtime configuration, never from RPC parameters or the binary.
type IntakeSubmitter struct {
	endpoint string
	client   *http.Client
	now      func() time.Time
	mu       sync.Mutex
	retryAt  time.Time
	backoff  time.Duration
}

func NewIntakeSubmitter() *IntakeSubmitter {
	return &IntakeSubmitter{endpoint: Endpoint, now: time.Now, client: &http.Client{
		Timeout: 100 * time.Second,
		// Even a same-host redirect must not forward a report or its key.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func retryDeadline(value string, now time.Time) time.Time {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if deadline, err := http.ParseTime(value); err == nil {
		if deadline.After(now) {
			return deadline
		}
		return now
	}
	return now.Add(time.Minute)
}

func responseRequestID(res *http.Response, token string) string {
	id := res.Header.Get("X-Request-ID")
	if len(id) > 128 || strings.Contains(id, token) {
		return ""
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return ""
		}
	}
	return id
}

func intakeToken() (string, error) {
	token := strings.TrimSpace(os.Getenv("WHATSAPPGO_BUGREPORT_TOKEN"))
	if token == "" {
		if path := strings.TrimSpace(os.Getenv("WHATSAPPGO_BUGREPORT_TOKEN_FILE")); path != "" {
			file, err := os.Open(path)
			if err != nil {
				return "", errors.New("could not open the bug-report intake key file")
			}
			defer file.Close()
			data, err := io.ReadAll(io.LimitReader(file, 16385))
			if err != nil || len(data) > 16384 {
				return "", errors.New("could not read the bug-report intake key file (maximum 16 KiB)")
			}
			token = strings.TrimSpace(string(data))
		}
	}
	if token == "" {
		return "", ErrNoToken
	}
	if len(token) > 16384 || strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return "", errors.New("the bug-report intake key has an invalid format")
	}
	return token, nil
}

func (s *IntakeSubmitter) Submit(ctx context.Context, subject, body string) (string, error) {
	subject, _, err := Validate(subject, body)
	if err != nil {
		return "", err
	}
	token, err := intakeToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	remaining := s.retryAt.Sub(s.now())
	s.mu.Unlock()
	if remaining > 0 {
		return "", fmt.Errorf("wait %d seconds before retrying the report", int64(math.Ceil(remaining.Seconds())))
	}
	// The service has already bounded the user's text and appended the reviewed
	// environment. Keep that block; do not truncate it back to the editor limit.
	payload, err := json.Marshal(map[string]any{
		"program": Program, "source": Program, "title": truncateUTF8(strings.ReplaceAll(subject, token, "[REDACTED]"), 255),
		"description": truncateUTF8(strings.ReplaceAll(strings.TrimSpace(body), token, "[REDACTED]"), 20000), "severity": "medium",
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not confirm report delivery; check the intake before retrying: %w", err)
	}
	defer res.Body.Close()
	requestID := responseRequestID(res, token)
	failure := func(message string) (string, error) {
		// Do not log report text, credentials, arbitrary headers or proxy HTML.
		log.Printf("bug report intake failed: status=%d request_id=%q", res.StatusCode, requestID)
		if requestID != "" {
			message += " (request ID: " + requestID + ")"
		}
		return "", errors.New(message)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		switch res.StatusCode {
		case http.StatusBadRequest:
			return failure("the intake rejected the report fields (HTTP 400); review the title and description before sending again")
		case http.StatusUnauthorized:
			return failure("the intake rejected the key; ask the intake operator for a valid WhatsAppGo key or use the public web form")
		case http.StatusRequestEntityTooLarge:
			return failure("the report is too large (HTTP 413); shorten it before sending again")
		case http.StatusUnsupportedMediaType:
			return failure("the intake rejected the report format (HTTP 415); use the public web form")
		case http.StatusTooManyRequests:
			s.mu.Lock()
			deadline := retryDeadline(res.Header.Get("Retry-After"), s.now())
			if deadline.After(s.retryAt) {
				s.retryAt = deadline
			}
			seconds := int64(math.Ceil(s.retryAt.Sub(s.now()).Seconds()))
			s.mu.Unlock()
			return failure(fmt.Sprintf("the intake is rate limiting reports; wait %d seconds before retrying", seconds))
		case http.StatusBadGateway:
			s.mu.Lock()
			if s.backoff == 0 {
				s.backoff = 5 * time.Second
			} else {
				s.backoff = min(s.backoff*2, time.Minute)
			}
			deadline := s.now().Add(s.backoff)
			if deadline.After(s.retryAt) {
				s.retryAt = deadline
			}
			seconds := int64(math.Ceil(s.retryAt.Sub(s.now()).Seconds()))
			s.mu.Unlock()
			return failure(fmt.Sprintf("the tracker is unavailable (HTTP 502); nothing was filed. Wait %d seconds before retrying", seconds))
		default:
			// Do not echo arbitrary proxy/server bodies, credentials or HTML.
			return failure(fmt.Sprintf("the intake rejected the report (HTTP %d)", res.StatusCode))
		}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Action  string `json:"action"`
			Program string `json:"program"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	unconfirmed := "the intake response did not confirm the report; check the intake before retrying"
	if err != nil || len(data) > maxResponseBytes || json.Unmarshal(data, &envelope) != nil || !envelope.OK || envelope.Data.Program != Program || (envelope.Data.Action != "created" && envelope.Data.Action != "commented") {
		return failure(unconfirmed)
	}
	link, err := url.Parse(envelope.Data.URL)
	if err != nil || link.Hostname() == "" || link.User != nil || (link.Scheme != "https" && link.Scheme != "http") {
		return failure(unconfirmed)
	}
	s.mu.Lock()
	s.backoff = 0
	s.mu.Unlock()
	return link.String(), nil
}
