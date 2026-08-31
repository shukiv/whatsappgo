package mediastore

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPutAndWriteRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	payload := bytes.Repeat([]byte("whatsappgo"), 1000)
	info := Info{MIME: "audio/ogg", Name: "voice.ogg", Size: int64(len(payload))}
	if err := s.Put(ctx, "alice@s.whatsapp.net", "voice-1", info, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	stored, found, err := s.Lookup(ctx, "alice@s.whatsapp.net", "voice-1")
	if err != nil || !found {
		t.Fatalf("attachment not found: %v", err)
	}
	if stored.MIME != "audio/ogg" || stored.Name != "voice.ogg" || stored.Size != int64(len(payload)) {
		t.Fatalf("unexpected metadata: %#v", stored)
	}
	var restored bytes.Buffer
	written, err := s.WriteTo(ctx, "alice@s.whatsapp.net", "voice-1", &restored)
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(len(payload)) || !bytes.Equal(restored.Bytes(), payload) {
		t.Fatalf("payload changed: wrote %d of %d bytes", written, len(payload))
	}
}

func TestPutSplitsLargeAttachmentsIntoChunks(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	// Larger than one chunk, so the reader path is exercised more than once.
	payload := bytes.Repeat([]byte{0xAB}, chunkSize+1024)
	if err := s.Put(ctx, "alice@s.whatsapp.net", "video-1", Info{MIME: "video/mp4"}, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	var chunks int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_chunks WHERE message_id=?`, "video-1").Scan(&chunks); err != nil {
		t.Fatal(err)
	}
	if chunks != 2 {
		t.Fatalf("expected the attachment to be split into 2 chunks, got %d", chunks)
	}
	var restored bytes.Buffer
	if _, err := s.WriteTo(ctx, "alice@s.whatsapp.net", "video-1", &restored); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored.Bytes(), payload) {
		t.Fatal("chunked attachment did not round-trip")
	}
}

func TestPutReplacesPreviousCopy(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.Put(ctx, "c@s.whatsapp.net", "m", Info{}, bytes.NewReader([]byte("first"))); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, "c@s.whatsapp.net", "m", Info{}, bytes.NewReader([]byte("second"))); err != nil {
		t.Fatal(err)
	}
	var restored bytes.Buffer
	if _, err := s.WriteTo(ctx, "c@s.whatsapp.net", "m", &restored); err != nil {
		t.Fatal(err)
	}
	if restored.String() != "second" {
		t.Fatalf("stale copy survived: %q", restored.String())
	}
	total, err := s.TotalSize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len("second")) {
		t.Fatalf("total size = %d, want %d", total, len("second"))
	}
}

func TestMaterialiseRestoresAClearedCache(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	cache := filepath.Join(t.TempDir(), "media", "audio")
	source := filepath.Join(t.TempDir(), "voice.ogg")
	payload := []byte("opus frames")
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.PutFile(ctx, "c@s.whatsapp.net", "voice-1", Info{MIME: "audio/ogg"}, source); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(cache, "voice-1.ogg")
	restored, err := s.Materialise(ctx, "c@s.whatsapp.net", "voice-1", target)
	if err != nil || !restored {
		t.Fatalf("attachment was not restored: restored=%v err=%v", restored, err)
	}
	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, payload) {
		t.Fatalf("restored file differs: %q", written)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("restored file permissions are %v, want 0600", info.Mode().Perm())
	}
	if stored, _, err := s.Lookup(ctx, "c@s.whatsapp.net", "voice-1"); err != nil || stored.Name != "voice.ogg" {
		t.Fatalf("PutFile did not record the file name: %#v err=%v", stored, err)
	}

	missing, err := s.Materialise(ctx, "c@s.whatsapp.net", "absent", target)
	if err != nil || missing {
		t.Fatalf("unknown attachment reported as restored: %v %v", missing, err)
	}
}

func TestDeleteRemovesAttachmentAndChunks(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.Put(ctx, "c@s.whatsapp.net", "m", Info{}, bytes.NewReader([]byte("data"))); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "c@s.whatsapp.net", "m"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.Lookup(ctx, "c@s.whatsapp.net", "m"); err != nil || found {
		t.Fatalf("attachment still present: found=%v err=%v", found, err)
	}
	var chunks int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_chunks`).Scan(&chunks); err != nil {
		t.Fatal(err)
	}
	if chunks != 0 {
		t.Fatalf("%d orphaned chunks remained", chunks)
	}
}
