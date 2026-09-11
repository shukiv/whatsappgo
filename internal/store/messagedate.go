package store

import (
	"context"
	"database/sql"
	"errors"
)

// MessageOnDate locates the first locally stored message in a caller's local
// calendar day. Explicit bounds preserve 23/25-hour daylight-saving days.
func (s *Store) MessageOnDate(ctx context.Context, chatJID string, start, end int64) (string, error) {
	if chatJID == "" || start < 0 || end <= start || end-start > 26*60*60*1000 {
		return "", errors.New("a chat and valid day boundaries are required")
	}
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM messages
 WHERE chat_jid=? AND timestamp>=? AND timestamp<?
 ORDER BY timestamp ASC,id ASC LIMIT 1`, s.canonicalChatJID(ctx, chatJID), start, end).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
