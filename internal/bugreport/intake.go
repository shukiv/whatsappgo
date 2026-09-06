package bugreport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	Program          = "whatsappgo"
	Endpoint         = "https://bugs.jabali-panel.com/api/v1/intake"
	maxResponseBytes = 64 * 1024
)

var ErrNoToken = errors.New("bug reporting needs an intake key: configure WHATSAPPGO_BUGREPORT_TOKEN_FILE or WHATSAPPGO_BUGREPORT_TOKEN for the daemon")

// IntakeSubmitter has one fixed destination and project. Credentials come from
// the daemon's runtime configuration, never from RPC parameters or the binary.
type IntakeSubmitter struct {
	endpoint string
	client   *http.Client
}

func NewIntakeSubmitter() *IntakeSubmitter {
	return &IntakeSubmitter{endpoint: Endpoint, client: &http.Client{
		Timeout: 100 * time.Second,
		// Even a same-host redirect must not forward a report or its key.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
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
	// The service has already bounded the user's text and appended the reviewed
	// environment. Keep that block; do not truncate it back to the editor limit.
	payload, err := json.Marshal(map[string]any{
		"program": Program, "source": Program, "title": subject,
		"description": truncateUTF8(strings.TrimSpace(body), 20000), "severity": "medium",
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
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		switch res.StatusCode {
		case http.StatusUnauthorized:
			return "", errors.New("the intake rejected the key; ask the intake operator for a valid WhatsAppGo key")
		case http.StatusTooManyRequests:
			return "", errors.New("the intake is rate limiting reports; wait before trying again")
		default:
			// Do not echo arbitrary proxy/server bodies, credentials or HTML.
			return "", fmt.Errorf("the intake rejected the report (HTTP %d)", res.StatusCode)
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
	unconfirmed := errors.New("the intake response did not confirm the report; check the intake before retrying")
	if err != nil || len(data) > maxResponseBytes || json.Unmarshal(data, &envelope) != nil || !envelope.OK || envelope.Data.Program != Program || (envelope.Data.Action != "created" && envelope.Data.Action != "commented") {
		return "", unconfirmed
	}
	link, err := url.Parse(envelope.Data.URL)
	if err != nil || link.Hostname() == "" || link.User != nil || (link.Scheme != "https" && link.Scheme != "http") {
		return "", unconfirmed
	}
	return link.String(), nil
}
