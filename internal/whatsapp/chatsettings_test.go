package whatsapp

import (
	"context"
	"errors"
	"fmt"
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
