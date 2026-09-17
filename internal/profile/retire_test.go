package profile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/config"
)

// One linked-device identity belongs in one place. An export that hands the
// account over says so on disk, so the copy left behind cannot be opened by
// somebody who forgot.
func TestAHandedOverCopyRefusesToBeOpened(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")

	if _, retired := Retired(source); retired {
		t.Fatal("a profile nobody exported counts as handed over")
	}
	result, err := Export(context.Background(), source, archive, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Deactivated {
		t.Fatalf("the export did not say it had retired this copy: %#v", result)
	}
	retirement, retired := Retired(source)
	if !retired {
		t.Fatal("an exported-and-handed-over copy is still openable")
	}
	if retirement.Archive != result.Path {
		t.Fatalf("the retirement does not say where the account went: %#v", retirement)
	}
	if retirement.RetiredAt <= 0 {
		t.Fatalf("the retirement does not say when: %#v", retirement)
	}
	// The refusal has to say what to do about it, or it is an obstacle rather
	// than a safeguard.
	refusal := RetiredError(source.Profile, retirement).Error()
	for _, expected := range []string{"israeli", "unlinked", retiredFile} {
		if !strings.Contains(refusal, expected) {
			t.Fatalf("the refusal does not mention %q: %s", expected, refusal)
		}
	}
}

// An ordinary export is not a handover. Taking a copy for safekeeping must not
// lock the reader out of their own account.
func TestAnOrdinaryExportLeavesTheAccountAlone(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")

	result, err := Export(context.Background(), source, archive, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deactivated {
		t.Fatalf("an export that was not a handover retired the account: %#v", result)
	}
	if _, retired := Retired(source); retired {
		t.Fatal("an export that was not a handover locked the account")
	}
}

// The archive exists before the copy is retired. The other order would leave a
// window where the account is neither exported nor usable.
func TestNothingIsRetiredUntilTheArchiveExists(t *testing.T) {
	source := newProfile(t, "israeli")
	// A directory the archive cannot be written into.
	unwritable := filepath.Join(t.TempDir(), "missing", "israeli.wagprofile")

	if _, err := Export(context.Background(), source, unwritable, false, true); err == nil {
		t.Fatal("an export that could not be written reported success")
	}
	if _, retired := Retired(source); retired {
		t.Fatal("the account was retired even though no archive was written")
	}
}

// A move that did not happen has to be undoable, or one mistake costs the
// account.
func TestARetirementCanBeUndone(t *testing.T) {
	source := newProfile(t, "israeli")
	if err := Retire(source, "/tmp/somewhere.wagprofile"); err != nil {
		t.Fatal(err)
	}
	if _, retired := Retired(source); !retired {
		t.Fatal("the retirement did not take")
	}
	if err := Revive(source); err != nil {
		t.Fatal(err)
	}
	if _, retired := Retired(source); retired {
		t.Fatal("the account is still locked after being revived")
	}
	// Undoing twice is not an error: the reader cannot be expected to know
	// which state it was in.
	if err := Revive(source); err != nil {
		t.Fatalf("undoing a retirement that was not there failed: %v", err)
	}
}

// The marker says the account moved away. An imported profile is the account
// that arrived, so a marker left by whatever was here before does not apply.
func TestAnImportedProfileIsNotRetired(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")
	if _, err := Export(context.Background(), source, archive, false, false); err != nil {
		t.Fatal(err)
	}
	// A profile that was itself handed away once, now receiving an account.
	occupied := newProfile(t, "other")
	if err := Retire(occupied, "/tmp/elsewhere.wagprofile"); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(archive, occupied, true); err != nil {
		t.Fatal(err)
	}
	if _, retired := Retired(occupied); retired {
		t.Fatal("an account that just arrived is marked as having left")
	}
}

// A marker nobody can read is still a marker. Reading it as "not retired"
// would turn a corrupt file into permission to run the account twice.
func TestAnUnreadableMarkerStillRefuses(t *testing.T) {
	source := newProfile(t, "israeli")
	if err := os.WriteFile(filepath.Join(source.DataDir, retiredFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, retired := Retired(source); !retired {
		t.Fatal("a marker that could not be read was taken as permission to open the account")
	}
}

// The marker names where the account went, so it is kept as private as
// everything else in the profile.
func TestTheMarkerIsWrittenPrivate(t *testing.T) {
	source := newProfile(t, "israeli")
	if err := Retire(source, "/tmp/somewhere.wagprofile"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(source.DataDir, retiredFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("the marker is readable by others: %v", info.Mode().Perm())
	}
}

// A profile that was never exported opens normally.
func TestAFreshProfileOpens(t *testing.T) {
	fresh := config.Paths{Profile: "new", DataDir: t.TempDir()}
	if _, retired := Retired(fresh); retired {
		t.Fatal("a profile with nothing in it counts as handed over")
	}
}
