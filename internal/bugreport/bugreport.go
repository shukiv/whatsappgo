// Package bugreport submits user-reviewed reports to the Jabali Bugs Intake.
package bugreport

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// Repository is the release source used by the updater, not the report intake.
const Repository = "shukiv/whatsappgo"

const (
	maxSubject = 120
	maxBody    = 8000
)

// Environment is the technical context attached to a report.
//
// Every field is listed explicitly and the desktop shows the whole block to the
// user before anything is sent. Nothing here identifies a person: no phone
// number, no chat identifier, no message text, no profile name.
type Environment struct {
	Version      string `json:"version"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Go           string `json:"go"`
	Connected    bool   `json:"connected"`
	LoggedIn     bool   `json:"logged_in"`
	Accounts     int    `json:"accounts"`
	Uptime       string `json:"uptime"`
}

// Render formats the environment as the fenced block appended to a report.
func (e Environment) Render() string {
	var b strings.Builder
	b.WriteString("```\n")
	fmt.Fprintf(&b, "version:      %s\n", e.Version)
	fmt.Fprintf(&b, "os:           %s\n", e.OS)
	fmt.Fprintf(&b, "architecture: %s\n", e.Architecture)
	fmt.Fprintf(&b, "go:           %s\n", e.Go)
	fmt.Fprintf(&b, "connected:    %t\n", e.Connected)
	fmt.Fprintf(&b, "logged in:    %t\n", e.LoggedIn)
	fmt.Fprintf(&b, "accounts:     %d\n", e.Accounts)
	fmt.Fprintf(&b, "uptime:       %s\n", e.Uptime)
	b.WriteString("```")
	return b.String()
}

// Describe collects the environment for a running daemon.
func Describe(version string, connected, loggedIn bool, accounts int, started time.Time) Environment {
	return Environment{
		Version:      version,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Go:           runtime.Version(),
		Connected:    connected,
		LoggedIn:     loggedIn,
		Accounts:     accounts,
		Uptime:       time.Since(started).Round(time.Second).String(),
	}
}

// Submitter files a report and returns the URL of what it created.
type Submitter interface {
	Submit(ctx context.Context, subject, body string) (string, error)
}

var (
	ErrNoSubject = errors.New("a bug report needs a subject")
	ErrNoBody    = errors.New("a bug report needs a description")
)

// Validate reports whether the text a user typed can be sent, and returns it
// trimmed and truncated. It never rewrites the meaning of what they wrote.
func Validate(subject, body string) (string, string, error) {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if subject == "" {
		return "", "", ErrNoSubject
	}
	if body == "" {
		return "", "", ErrNoBody
	}
	// A title is a single line; fold whitespace without losing words.
	subject = strings.Join(strings.Fields(subject), " ")
	subject = truncateUTF8(subject, maxSubject)
	body = truncateUTF8(body, maxBody)
	return subject, body, nil
}

func truncateUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit]
}
