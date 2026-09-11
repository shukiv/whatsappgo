package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

func TestMarkAllReadIncludesEveryPageAndArchivedChat(t *testing.T) {
	ctx := context.Background()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for i := 0; i < 1202; i++ {
		jid := fmt.Sprintf("%d@lid", i+1)
		if err := st.UpsertMessage(ctx, model.Message{ID: "m", ChatJID: jid, Timestamp: int64(i + 1), Kind: "text", Status: "received"}, "", true); err != nil {
			t.Fatal(err)
		}
		if i >= 601 {
			if err := st.UpdateChatArchived(ctx, jid, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	wantErr := errors.New("one chat failed")
	seen := map[string]bool{}
	cleared, err := markAllChatsRead(ctx, st, func(ctx context.Context, jid string, read bool) error {
		if !read || seen[jid] {
			t.Fatalf("invalid/duplicate action: %s %v", jid, read)
		}
		seen[jid] = true
		if jid == "1@lid" {
			return wantErr
		}
		return st.MarkChatRead(ctx, jid)
	})
	if !errors.Is(err, wantErr) || cleared != 1201 || len(seen) != 1202 {
		t.Fatalf("cleared=%d visited=%d error=%v", cleared, len(seen), err)
	}
}

func TestExportRefusesToReplaceAFileUnlessAsked(t *testing.T) {
	ctx := context.Background()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: "alice@lid", Timestamp: 1000, Kind: "text", Body: "exported line", Status: "received"}, "Alice", true); err != nil {
		t.Fatal(err)
	}
	client := &Client{store: st}
	destination := filepath.Join(t.TempDir(), "transcript.txt")
	if err := os.WriteFile(destination, []byte("someone else's file"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := client.ExportChat(ctx, "alice@lid", destination, false); err == nil {
		t.Fatal("an existing file was replaced without being asked")
	}
	kept, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "someone else's file" {
		t.Fatalf("the existing file was written over: %q", kept)
	}

	if _, err := client.ExportChat(ctx, "alice@lid", destination, true); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "exported line") {
		t.Fatalf("an explicit replacement did not write the transcript: %q", written)
	}
}

func TestExportWritesANewFileWithOwnerOnlyPermissions(t *testing.T) {
	ctx := context.Background()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.UpsertMessage(ctx, model.Message{ID: "m1", ChatJID: "alice@lid", Timestamp: 1000, Kind: "text", Body: "exported line", Status: "received"}, "Alice", true); err != nil {
		t.Fatal(err)
	}
	client := &Client{store: st}
	destination := filepath.Join(t.TempDir(), "new-transcript.txt")
	if _, err := client.ExportChat(ctx, "alice@lid", destination, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("transcript permissions = %v, want 0600", info.Mode().Perm())
	}
}
