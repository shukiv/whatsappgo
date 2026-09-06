package bugreport

import (
	"errors"
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

// Multiline titles are folded without silently discarding words.
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

// The environment leaves the device, so it carries nothing that
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
