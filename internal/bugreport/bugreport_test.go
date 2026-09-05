package bugreport

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsEmptyReports(t *testing.T) {
	if _, _, err := Validate("   ", "something is wrong"); !errors.Is(err, ErrNoSubject) {
		t.Fatalf("err=%v, want ErrNoSubject", err)
	}
	if _, _, err := Validate("Crash on start", "\n\t "); !errors.Is(err, ErrNoBody) {
		t.Fatalf("err=%v, want ErrNoBody", err)
	}
}

// A newline in the title would end the argument and silently discard the rest,
// so the whole subject is folded onto one line instead.
func TestValidateFoldsTheSubject(t *testing.T) {
	subject, _, err := Validate("Crash\non\tstart", "steps")
	if err != nil {
		t.Fatal(err)
	}
	if subject != "Crash on start" {
		t.Fatalf("subject=%q", subject)
	}
}

func TestValidateBoundsLength(t *testing.T) {
	subject, body, err := Validate(strings.Repeat("a", maxSubject+50), strings.Repeat("b", maxBody+50))
	if err != nil {
		t.Fatal(err)
	}
	if len(subject) != maxSubject {
		t.Fatalf("subject length=%d, want %d", len(subject), maxSubject)
	}
	if len(body) != maxBody {
		t.Fatalf("body length=%d, want %d", len(body), maxBody)
	}
}

// The environment goes into a public issue, so it carries nothing that
// identifies a person: no phone number, no chat identifier, no message text.
func TestEnvironmentCarriesNoIdentity(t *testing.T) {
	rendered := Describe("1.2.3", true, true, 2, time.Now().Add(-90*time.Second)).Render()
	for _, field := range []string{"version:", "os:", "architecture:", "accounts:", "uptime:"} {
		if !strings.Contains(rendered, field) {
			t.Fatalf("rendered environment is missing %q:\n%s", field, rendered)
		}
	}
	if strings.Contains(rendered, "@s.whatsapp.net") || strings.Contains(rendered, "@lid") {
		t.Fatalf("the environment carries a chat identifier:\n%s", rendered)
	}
	if !strings.Contains(rendered, "accounts:     2") {
		t.Fatalf("account count missing:\n%s", rendered)
	}
}

// The subject and body are arbitrary user text. They are passed as separate
// arguments so nothing in them can become a shell word.
func TestSubmitPassesUserTextAsArguments(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh is not installed")
	}
	var got []string
	submitter := &CLISubmitter{Command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
		got = append([]string{name}, args...)
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperEcho")
		command.Env = append(os.Environ(), "BUGREPORT_HELPER=1")
		return command
	}}
	if _, err := submitter.Submit(context.Background(), "Title", "Body; rm -rf /"); err != nil {
		t.Fatal(err)
	}
	want := []string{"gh", "issue", "create", "--repo", Repository,
		"--title", "Title", "--body", "Body; rm -rf /"}
	if len(got) != len(want) {
		t.Fatalf("argv=%q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestHelperEcho(t *testing.T) {
	if os.Getenv("BUGREPORT_HELPER") != "1" {
		t.Skip("helper process")
	}
	os.Stdout.WriteString("https://github.com/" + Repository + "/issues/1\n")
}
