package profile

import (
	"archive/tar"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/shukiv/whatsappgo/internal/config"
)

// A profile made of real SQLite files, so the snapshot is exercised against
// what it will meet rather than against a stand-in.
func newProfile(t *testing.T, name string) config.Paths {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Profile:   name,
		DataDir:   dir,
		DeviceDB:  filepath.Join(dir, deviceFile),
		MessageDB: filepath.Join(dir, messageFile),
		MediaDB:   filepath.Join(dir, mediaFile),
	}
	write(t, paths.DeviceDB, "CREATE TABLE device (jid TEXT)", "INSERT INTO device VALUES ('1@s.whatsapp.net')")
	write(t, paths.MessageDB, "CREATE TABLE messages (body TEXT)", "INSERT INTO messages VALUES ('what it said')")
	write(t, paths.MediaDB, "CREATE TABLE media (blob BLOB)", "INSERT INTO media VALUES ('picture')")
	return paths
}

func write(t *testing.T, path string, statements ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func read(t *testing.T, path, query string) string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(query).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func entries(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var names []string
	archive := tar.NewReader(file)
	for {
		header, err := archive.Next()
		if err != nil {
			return names
		}
		names = append(names, header.Name)
	}
}

func writeEntry(t *testing.T, archive *tar.Writer, name string, body []byte) {
	t.Helper()
	if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(body); err != nil {
		t.Fatal(err)
	}
}

// handmade builds an archive that says one thing and holds another, which is
// what every refusal below is about.
func handmade(t *testing.T, path string, manifest Manifest, holds map[string][]byte) string {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	described, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeEntry(t, writer, manifestID, described)
	for name, body := range holds {
		writeEntry(t, writer, name, body)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// An exported account arrives on another machine holding what it left with.
func TestAnExportedAccountCanBeImportedElsewhere(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")

	result, err := Export(context.Background(), source, archive, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes <= 0 || result.Warning == "" {
		t.Fatalf("the export said nothing about what it wrote: %#v", result)
	}

	target := config.Paths{Profile: "israeli", DataDir: filepath.Join(t.TempDir(), "profile")}
	manifest, err := Import(archive, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Profile != "israeli" {
		t.Fatalf("the archive forgot which profile it held: %#v", manifest)
	}
	if got := read(t, filepath.Join(target.DataDir, deviceFile), "SELECT jid FROM device"); got != "1@s.whatsapp.net" {
		t.Fatalf("the device identity did not survive the move: %q", got)
	}
	if got := read(t, filepath.Join(target.DataDir, messageFile), "SELECT body FROM messages"); got != "what it said" {
		t.Fatalf("the history did not survive the move: %q", got)
	}
}

// Attachments are the difference between an archive of tens of megabytes and
// one of several gigabytes, so they travel only when asked for.
func TestAttachmentsAreLeftBehindUnlessAskedFor(t *testing.T) {
	source := newProfile(t, "israeli")
	dir := t.TempDir()

	lean := filepath.Join(dir, "lean.wagprofile")
	if _, err := Export(context.Background(), source, lean, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range entries(t, lean) {
		if name == mediaFile {
			t.Fatal("attachments were exported without being asked for")
		}
	}

	full := filepath.Join(dir, "full.wagprofile")
	if _, err := Export(context.Background(), source, full, true); err != nil {
		t.Fatal(err)
	}
	var carried bool
	for _, name := range entries(t, full) {
		carried = carried || name == mediaFile
	}
	if !carried {
		t.Fatalf("attachments were asked for and left behind: %#v", entries(t, full))
	}
}

// A running daemon keeps writing while an export is taken, so the snapshot has
// to come from SQLite rather than from a file copy.
func TestAnExportRunsWhileTheDatabaseIsOpen(t *testing.T) {
	source := newProfile(t, "israeli")
	held, err := sql.Open("sqlite", "file:"+source.MessageDB+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	// Written after the connection is open and left in the write-ahead log,
	// which is what a hand copy of the .db file would miss.
	if _, err := held.Exec("INSERT INTO messages VALUES ('written while open')"); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "open.wagprofile")
	if _, err := Export(context.Background(), source, archive, false); err != nil {
		t.Fatalf("an export could not be taken from a database in use: %v", err)
	}
	target := config.Paths{Profile: "israeli", DataDir: filepath.Join(t.TempDir(), "profile")}
	if _, err := Import(archive, target, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(target.DataDir, messageFile),
		"SELECT body FROM messages WHERE body = 'written while open'"); got != "written while open" {
		t.Fatalf("the export missed what was written while the database was open: %q", got)
	}
}

// Importing replaces an identity. Doing that to the wrong profile would leave
// its history unreachable, so it takes saying so.
func TestImportingRefusesToReplaceAnAccountUnlessTold(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")
	if _, err := Export(context.Background(), source, archive, false); err != nil {
		t.Fatal(err)
	}
	occupied := newProfile(t, "other")

	_, err := Import(archive, occupied, false)
	if err == nil {
		t.Fatal("an account was replaced without being asked about")
	}
	if !strings.Contains(err.Error(), "already holds") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
	if got := read(t, occupied.DeviceDB, "SELECT jid FROM device"); got != "1@s.whatsapp.net" {
		t.Fatalf("the refused import wrote anyway: %q", got)
	}
	if _, err := Import(archive, occupied, true); err != nil {
		t.Fatalf("an account could not be replaced even when asked: %v", err)
	}
}

// Writing an export over an existing file would destroy it, and what is being
// written is a credential.
func TestExportingRefusesToOverwrite(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "taken.wagprofile")
	if err := os.WriteFile(archive, []byte("something else"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(context.Background(), source, archive, false); err == nil {
		t.Fatal("an existing file was overwritten")
	}
	if got, _ := os.ReadFile(archive); string(got) != "something else" {
		t.Fatalf("the refused export wrote anyway: %q", got)
	}
}

// An archive holds credentials, so it is written unreadable to anyone else,
// and arrives that way.
func TestAnArchiveIsWrittenPrivate(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")
	if _, err := Export(context.Background(), source, archive, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("the archive is readable by others: %v", info.Mode().Perm())
	}
	target := config.Paths{Profile: "israeli", DataDir: filepath.Join(t.TempDir(), "profile")}
	if _, err := Import(archive, target, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{deviceFile, messageFile} {
		info, err := os.Stat(filepath.Join(target.DataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s arrived readable by others: %v", name, info.Mode().Perm())
		}
	}
	directory, err := os.Stat(target.DataDir)
	if err != nil || directory.Mode().Perm() != 0o700 {
		t.Fatalf("the imported profile directory is not private: %v %v", directory.Mode().Perm(), err)
	}
}

// A name inside an archive is a string someone else wrote. Nothing may use it
// to name a path of its own.
func TestAnArchiveCannotNameAPathOfItsOwn(t *testing.T) {
	for _, name := range []string{"../escaped", "/etc/passwd", "nested/device.db"} {
		archive := handmade(t, filepath.Join(t.TempDir(), "hostile.wagprofile"),
			Manifest{Version: 1, Profile: "x", Files: []string{deviceFile}},
			map[string][]byte{name: []byte("payload")})
		target := config.Paths{Profile: "x", DataDir: filepath.Join(t.TempDir(), "profile")}
		if _, err := Import(archive, target, false); err == nil {
			t.Fatalf("an archive entry named %q was accepted", name)
		}
	}
}

// An archive that is not one, or that promises what it does not hold, is
// refused before anything is written.
func TestAnUnsoundArchiveIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	dir := t.TempDir()
	target := config.Paths{Profile: "x", DataDir: filepath.Join(dir, "profile")}

	notAnArchive := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notAnArchive, []byte("just some text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(notAnArchive, target, false); err == nil {
		t.Fatal("a file that is not an archive was imported")
	}

	incomplete := handmade(t, filepath.Join(dir, "incomplete.wagprofile"),
		Manifest{Version: 1, Profile: "x", Files: []string{deviceFile, messageFile}},
		map[string][]byte{deviceFile: []byte("identity")})
	if _, err := Import(incomplete, target, false); err == nil {
		t.Fatal("an archive missing what it promised was imported")
	}

	// History with no identity is not an account.
	headless := handmade(t, filepath.Join(dir, "headless.wagprofile"),
		Manifest{Version: 1, Profile: "x", Files: []string{messageFile}},
		map[string][]byte{messageFile: []byte("history")})
	if _, err := Import(headless, target, false); err == nil {
		t.Fatal("an archive with no device identity was imported as an account")
	}

	if _, err := os.Stat(target.DataDir); err == nil {
		t.Fatal("a refused import created the profile anyway")
	}
}

// An archive from a newer version is refused rather than half-understood.
func TestAnArchiveFromANewerVersionIsRefused(t *testing.T) {
	archive := handmade(t, filepath.Join(t.TempDir(), "future.wagprofile"),
		Manifest{Version: FormatVersion + 1, Profile: "x", Files: []string{deviceFile}},
		map[string][]byte{deviceFile: []byte("identity")})

	target := config.Paths{Profile: "x", DataDir: filepath.Join(t.TempDir(), "profile")}
	_, err := Import(archive, target, false)
	if err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Fatalf("an archive from a newer format was not refused clearly: %v", err)
	}
}

// Replacing a database must not leave the previous one's write-ahead log
// beside it, or SQLite reads the old contents back over the new file.
func TestReplacingAProfileClearsWhatTheOldOneLeftBehind(t *testing.T) {
	source := newProfile(t, "israeli")
	archive := filepath.Join(t.TempDir(), "israeli.wagprofile")
	if _, err := Export(context.Background(), source, archive, false); err != nil {
		t.Fatal(err)
	}
	occupied := newProfile(t, "other")
	if err := os.WriteFile(occupied.MessageDB+"-wal", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(archive, occupied, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(occupied.MessageDB + "-wal"); err == nil {
		t.Fatal("the replaced database kept the previous one's write-ahead log")
	}
}
