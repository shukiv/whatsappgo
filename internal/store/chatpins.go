package store

import (
	"context"
	"database/sql"
)

// Merge before deleting the alias row, inside the same transaction as the
// identity merge. A versioned unpin wins over an older pin. With only history
// hints on both sides, retain the legacy non-destructive pin/order merge.
func mergeChatPinState(ctx context.Context, tx *sql.Tx, canonicalJID, aliasJID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE chats AS c SET (pinned,pinned_at,pin_action_at)=(
	 SELECT
	 CASE WHEN a.pin_action_at>c.pin_action_at THEN a.pinned
	      WHEN c.pin_action_at>0 THEN c.pinned ELSE MAX(c.pinned,a.pinned) END,
	 CASE WHEN a.pin_action_at>c.pin_action_at THEN a.pinned_at
	      WHEN c.pin_action_at>0 THEN c.pinned_at ELSE MAX(c.pinned_at,a.pinned_at) END,
	 MAX(c.pin_action_at,a.pin_action_at)
	 FROM chats a WHERE a.jid=?
	) WHERE c.jid=? AND EXISTS(SELECT 1 FROM chats WHERE jid=?)`, aliasJID, canonicalJID, aliasJID)
	return err
}
