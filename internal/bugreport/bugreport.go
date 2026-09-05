// Package bugreport turns a user's description of a problem into a GitHub
// issue, with the environment details that make it diagnosable.
package bugreport

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Repository is where reports are filed. It is the project's own repository,
// not anything the caller can choose: a report is only ever sent to one place.
const Repository = "shukiv/whatsappgo"

const (
	maxSubject = 120
	maxBody    = 8000
	submitWait = 30 * time.Second
)

// Environment is the technical context attached to a report.
//
// Every field is listed explicitly and the desktop shows the whole block to the
// user before anything is sent. Nothing here identifies a person: no phone
// number, no chat identifier, no message text, no profile name. A report is a
// public issue on a public repository, so anything that leaks into it stays
// leaked.
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

// CLISubmitter files the issue with the GitHub CLI.
//
// The CLI holds the user's own credentials, so this application never sees,
// stores or transmits a token. That is the entire reason for the choice: a
// token embedded in the client would be a shared secret shipped to every
// installation.
type CLISubmitter struct {
	// Command runs an external program. Tests replace it; nothing else does.
	Command func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewCLISubmitter() *CLISubmitter {
	return &CLISubmitter{Command: exec.CommandContext}
}

var (
	ErrNoSubject = errors.New("a bug report needs a subject")
	ErrNoBody    = errors.New("a bug report needs a description")
	ErrNoCLI     = errors.New("the GitHub CLI is not installed; install gh and run 'gh auth login' to send reports")
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
	// A title is a single line. A newline in it would end the argument and the
	// rest would be lost silently, so it is folded rather than dropped.
	subject = strings.Join(strings.Fields(subject), " ")
	if len(subject) > maxSubject {
		subject = subject[:maxSubject]
	}
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	return subject, body, nil
}

func (s *CLISubmitter) Submit(ctx context.Context, subject, body string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", ErrNoCLI
	}
	ctx, cancel := context.WithTimeout(ctx, submitWait)
	defer cancel()
	// Arguments are passed as a vector, never as a command line: a subject or
	// body is arbitrary user text and must not be able to become a shell word.
	command := s.Command(ctx, "gh", "issue", "create",
		"--repo", Repository,
		"--title", subject,
		"--body", body)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("send the report: %s", firstMeaningfulLine(string(output), err))
	}
	return firstIssueURL(string(output)), nil
}

func firstMeaningfulLine(output string, fallback error) string {
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return fallback.Error()
}

func firstIssueURL(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "https://") {
			return trimmed
		}
	}
	return ""
}
