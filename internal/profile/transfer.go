// Package profile moves one account between machines.
//
// A profile is a linked WhatsApp device: the identity and keys in device.db,
// the history in messages.db, and the attachments in media.db. Copying those
// files by hand is what this exists to replace - SQLite in write-ahead mode
// keeps recent changes in a -wal companion, so a copy taken while the daemon
// is running is missing its newest writes, and one taken without the
// companions is silently older than it looks.
//
// VACUUM INTO is the answer: it asks SQLite itself for a consistent snapshot
// of a live database, companions folded in, with nothing stopped.
package profile

import (
	"archive/tar"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/shukiv/whatsappgo/internal/config"
)

// FormatVersion is the shape of the archive. An archive from a newer version
// is refused rather than half-understood.
const FormatVersion = 1

// Warning is what both ends print. An exported profile is not a backup: it is
// a second copy of one device's credentials.
const Warning = "This archive carries the account's linked-device credentials. " +
	"Anyone holding it can read and send as that account, so move it like a password and " +
	"delete it afterwards. Run the profile on exactly one machine: the same device identity " +
	"connected twice shares one message ratchet, and is expected to be unlinked by WhatsApp."

// The files a profile is made of. media.db is the large one and the only one
// a working account can do without: attachments can be downloaded again,
// identity and history cannot.
const (
	deviceFile  = "device.db"
	messageFile = "messages.db"
	mediaFile   = "media.db"
	manifestID  = "manifest.json"
	retiredFile = "retired.json"
)

// Retirement marks a profile that has been exported and handed over. One
// linked-device identity belongs in one place: the same identity connected
// from two machines shares a single message ratchet, and WhatsApp is expected
// to unlink the device. Remembering not to open the old copy is not a
// safeguard, so the old copy refuses to open.
type Retirement struct {
	RetiredAt int64  `json:"retired_at"`
	Archive   string `json:"archive"`
}

// Retire marks this copy of the profile as handed over. It is written after
// the archive exists, so there is never a moment where the account is neither
// exported nor usable.
func Retire(paths config.Paths, archive string) error {
	described, err := json.MarshalIndent(Retirement{
		RetiredAt: time.Now().UnixMilli(), Archive: archive,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(paths.DataDir, retiredFile), described, 0o600)
}

// Retired reports whether this copy has been handed over, and when.
func Retired(paths config.Paths) (Retirement, bool) {
	described, err := os.ReadFile(filepath.Join(paths.DataDir, retiredFile))
	if err != nil {
		return Retirement{}, false
	}
	var retirement Retirement
	if err := json.Unmarshal(described, &retirement); err != nil {
		// A marker that cannot be read is still a marker, and refusing to open
		// is the safe reading of it.
		return Retirement{}, true
	}
	return retirement, true
}

// Revive undoes a retirement, for the case where the move did not happen and
// this is once again the only copy. Whether that is true is the caller's
// business; nothing here can check it.
func Revive(paths config.Paths) error {
	err := os.Remove(filepath.Join(paths.DataDir, retiredFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// RetiredError is what a daemon says when asked to open a handed-over copy.
func RetiredError(profile string, retirement Retirement) error {
	when := "another machine"
	if retirement.RetiredAt > 0 {
		when = "another machine on " + time.UnixMilli(retirement.RetiredAt).Format("2006-01-02 15:04")
	}
	return fmt.Errorf("profile %q was exported to %s and this copy was retired. "+
		"Running one account in two places gets the device unlinked. "+
		"If the move did not happen, delete %s from the profile directory to use this copy again",
		profile, when, retiredFile)
}

// Manifest says what the archive holds, so an import can check before it
// writes anything.
type Manifest struct {
	Version    int      `json:"version"`
	Profile    string   `json:"profile"`
	ExportedAt int64    `json:"exported_at"`
	Media      bool     `json:"includes_media"`
	Files      []string `json:"files"`
}

// Result reports what an export produced.
type Result struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	Media       bool   `json:"includes_media"`
	Deactivated bool   `json:"deactivated"`
	Warning     string `json:"warning"`
}

// Export writes one profile to a tar archive at destination.
//
// The daemon does not have to stop: each database is snapshotted through
// VACUUM INTO, which is consistent by construction. Attachments are left out
// unless asked for, because they are the difference between an archive of
// tens of megabytes and one of several gigabytes, and a profile without them
// still holds every message.
// deactivate retires this copy once the archive exists, so the account cannot
// be opened here again. It is the difference between a safeguard and a note to
// self, and it is why the order matters: the archive is complete first.
// createArchive makes the file the account is written to. The file is created
// 0600 rather than fixed afterwards: between creating it and changing its mode
// it would be readable by anyone. It is a variable because a failure here is
// the one place the export can break after the snapshots and before the
// archive exists, and a test has no other way to reach it.
var createArchive = func(destination string) (*os.File, error) {
	return os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

func Export(ctx context.Context, paths config.Paths, destination string, includeMedia, deactivate bool) (Result, error) {
	if strings.TrimSpace(destination) == "" {
		return Result{}, errors.New("a path to write the archive to is required")
	}
	destination, err := filepath.Abs(destination)
	if err != nil {
		return Result{}, err
	}
	// Refusing to overwrite is deliberate: what is being written holds
	// credentials, and overwriting the wrong file cannot be undone.
	if _, err := os.Stat(destination); err == nil {
		return Result{}, fmt.Errorf("%s already exists", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}

	staging, err := os.MkdirTemp(filepath.Dir(destination), ".whatsappgo-export-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(staging)

	wanted := []string{deviceFile, messageFile}
	sources := map[string]string{deviceFile: paths.DeviceDB, messageFile: paths.MessageDB}
	if includeMedia {
		wanted = append(wanted, mediaFile)
		sources[mediaFile] = paths.MediaDB
	}
	for _, name := range wanted {
		if err := snapshot(ctx, sources[name], filepath.Join(staging, name)); err != nil {
			return Result{}, fmt.Errorf("snapshot %s: %w", name, err)
		}
	}

	manifest := Manifest{
		Version:    FormatVersion,
		Profile:    paths.Profile,
		ExportedAt: time.Now().UnixMilli(),
		Media:      includeMedia,
		Files:      wanted,
	}
	archive, err := createArchive(destination)
	if err != nil {
		return Result{}, err
	}
	if err := writeArchive(archive, staging, manifest); err != nil {
		archive.Close()
		os.Remove(destination)
		return Result{}, err
	}
	if err := archive.Close(); err != nil {
		os.Remove(destination)
		return Result{}, err
	}
	written, err := os.Stat(destination)
	if err != nil {
		return Result{}, err
	}
	if deactivate {
		if err := Retire(paths, destination); err != nil {
			return Result{}, fmt.Errorf("the archive was written but this copy could not be retired: %w", err)
		}
	}
	return Result{Path: destination, Bytes: written.Size(), Media: includeMedia,
		Deactivated: deactivate, Warning: Warning}, nil
}

func writeArchive(out io.Writer, staging string, manifest Manifest) error {
	described, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	archive := tar.NewWriter(out)
	if err := archive.WriteHeader(&tar.Header{
		Name: manifestID, Mode: 0o600, Size: int64(len(described)), ModTime: time.Now(),
	}); err != nil {
		return err
	}
	if _, err := archive.Write(described); err != nil {
		return err
	}
	for _, name := range manifest.Files {
		if err := appendFile(archive, filepath.Join(staging, name), name); err != nil {
			return err
		}
	}
	return archive.Close()
}

func appendFile(archive *tar.Writer, path, name string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := archive.WriteHeader(&tar.Header{
		Name: name, Mode: 0o600, Size: info.Size(), ModTime: info.ModTime(),
	}); err != nil {
		return err
	}
	_, err = io.Copy(archive, file)
	return err
}

// snapshot asks SQLite for a consistent copy of a database the daemon may
// have open and be writing to. A file copy cannot do this.
func snapshot(ctx context.Context, source, destination string) error {
	if _, err := os.Stat(source); err != nil {
		return err
	}
	separator := "?"
	if strings.Contains(source, "?") {
		separator = "&"
	}
	db, err := sql.Open("sqlite", "file:"+source+separator+"_pragma=busy_timeout(15000)")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// The destination travels as a parameter, so a path is never spliced into
	// the statement.
	_, err = db.ExecContext(ctx, "VACUUM INTO ?", destination)
	return err
}

// Import unpacks an archive into a profile's data directory.
//
// Nothing is written until the whole archive has been read and found sound: a
// half-written profile is worse than none, because the daemon would open it
// and report an account that has lost its history.
func Import(archivePath string, paths config.Paths, force bool) (Manifest, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()

	if err := os.MkdirAll(filepath.Dir(paths.DataDir), 0o700); err != nil {
		return Manifest{}, err
	}
	staging, err := os.MkdirTemp(filepath.Dir(paths.DataDir), ".whatsappgo-import-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(staging)

	manifest, err := unpack(file, staging)
	if err != nil {
		return Manifest{}, err
	}
	if err := checkTarget(paths, force); err != nil {
		return Manifest{}, err
	}
	if err := os.MkdirAll(paths.DataDir, 0o700); err != nil {
		return Manifest{}, err
	}
	for _, name := range manifest.Files {
		source := filepath.Join(staging, name)
		target := filepath.Join(paths.DataDir, name)
		// Write-ahead companions of whatever was there describe a database
		// that is being replaced. Left behind, SQLite would read them back
		// over the imported file.
		for _, companion := range []string{target + "-wal", target + "-shm"} {
			if err := os.Remove(companion); err != nil && !errors.Is(err, os.ErrNotExist) {
				return Manifest{}, err
			}
		}
		if err := os.Rename(source, target); err != nil {
			// A rename across filesystems is refused, so fall back to copying.
			if err := copyFile(source, target); err != nil {
				return Manifest{}, err
			}
		}
		if err := os.Chmod(target, 0o600); err != nil {
			return Manifest{}, err
		}
	}
	if err := os.Chmod(paths.DataDir, 0o700); err != nil {
		return Manifest{}, err
	}
	// An imported profile is the live copy by definition. A retirement left by
	// whatever was here before describes an account that is gone.
	if err := Revive(paths); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// checkTarget refuses to write over an account that is already there.
// Importing replaces an identity, and doing that to the wrong profile name
// would leave its history unreachable.
func checkTarget(paths config.Paths, force bool) error {
	for _, name := range []string{deviceFile, messageFile, mediaFile} {
		info, err := os.Stat(filepath.Join(paths.DataDir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Size() > 0 && !force {
			return fmt.Errorf("profile %q already holds %s; pass force to replace it", paths.Profile, name)
		}
	}
	return nil
}

// unpack reads the archive into a staging directory and returns what it says
// it holds. Every entry is checked against a fixed set of names: a name in a
// tar file is a string someone else wrote, not a path to be trusted, so
// nothing here can name a directory, an absolute path, or a way out of the
// staging directory.
func unpack(r io.Reader, staging string) (Manifest, error) {
	allowed := map[string]bool{deviceFile: true, messageFile: true, mediaFile: true}
	var manifest Manifest
	seen := map[string]bool{}
	archive := tar.NewReader(r)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, fmt.Errorf("read the archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			return Manifest{}, fmt.Errorf("the archive holds %q, which is not a file", header.Name)
		}
		if header.Name == manifestID {
			if err := json.NewDecoder(archive).Decode(&manifest); err != nil {
				return Manifest{}, fmt.Errorf("read the archive's manifest: %w", err)
			}
			continue
		}
		if !allowed[header.Name] {
			return Manifest{}, fmt.Errorf("the archive holds an unexpected entry %q", header.Name)
		}
		written, err := os.OpenFile(filepath.Join(staging, header.Name),
			os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return Manifest{}, err
		}
		if _, err := io.Copy(written, archive); err != nil {
			written.Close()
			return Manifest{}, err
		}
		if err := written.Close(); err != nil {
			return Manifest{}, err
		}
		seen[header.Name] = true
	}
	if manifest.Version == 0 {
		return Manifest{}, errors.New("this is not a WhatsAppGo profile archive")
	}
	if manifest.Version > FormatVersion {
		return Manifest{}, fmt.Errorf("the archive was written by a newer version (format %d, this understands %d)",
			manifest.Version, FormatVersion)
	}
	if len(manifest.Files) == 0 {
		return Manifest{}, errors.New("the archive's manifest lists no files")
	}
	for _, name := range manifest.Files {
		if !seen[name] {
			return Manifest{}, fmt.Errorf("the archive promises %s and does not hold it", name)
		}
	}
	// An account with no identity is not an account. Refusing here is what
	// keeps a partial archive from becoming a profile that opens and is empty.
	if !seen[deviceFile] {
		return Manifest{}, errors.New("the archive holds no device identity, so it cannot be an account")
	}
	return manifest, nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
