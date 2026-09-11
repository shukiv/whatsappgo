package store

import (
	"context"
	"database/sql"
)

// Clear content retained by older builds once an explicit view-once marker
// becomes known. IDs, ordering, senders and receipts remain in the timeline.
func redactViewOnceTx(ctx context.Context, tx *sql.Tx, chatJID string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE messages SET body='',media_mime='',media_name='',media_path='',media_thumbnail='',
 media_size=0,media_duration=0,audio_waveform=NULL,gif_playback=0,link_url='',link_title='',link_description='',link_thumbnail='',
 contact_name='',contact_phone='',contact_count=0,latitude=0,longitude=0,starred=0,mentions=''
 WHERE chat_jid=? AND kind='view_once'`, chatJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM media_payloads WHERE chat_jid=? AND message_id IN
 (SELECT id FROM messages WHERE chat_jid=? AND kind='view_once')`, chatJID, chatJID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE chats SET last_message_preview='View once message' WHERE jid=?
 AND last_message_id IN (SELECT id FROM messages WHERE chat_jid=? AND kind='view_once')`, chatJID, chatJID)
	return err
}

// MediaPayloadPage supplies bounded local pages for metadata-only repairs.
// It does not request media, and the stable cursor survives removing a payload.
func (s *Store) MediaPayloadPage(ctx context.Context, after MediaCursor) ([]PendingThumbnail, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT chat_jid,message_id,payload FROM media_payloads
 WHERE (chat_jid,message_id)>(?,?) ORDER BY chat_jid,message_id LIMIT 32`, after.ChatJID, after.MessageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingThumbnail
	for rows.Next() {
		var item PendingThumbnail
		if err := rows.Scan(&item.ChatJID, &item.MessageID, &item.Payload); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
