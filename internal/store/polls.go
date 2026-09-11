package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/model"
)

func (s *Store) SavePollVote(ctx context.Context, v model.PollVote) error {
	chat := s.canonicalChatJID(ctx, v.ChatJID)
	if chat == "" || v.PollID == "" || v.Sender == "" || v.Timestamp <= 0 {
		return errors.New("incomplete poll update")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO poll_votes(chat_jid,poll_id,sender,timestamp,pending,data)
 SELECT ?,?,?,?,?,? WHERE NOT EXISTS(SELECT 1 FROM messages WHERE chat_jid=? AND id=? AND (revoked=1 OR kind='view_once'))
 ON CONFLICT(chat_jid,poll_id,sender) DO UPDATE SET timestamp=excluded.timestamp,pending=excluded.pending,data=excluded.data
 WHERE excluded.timestamp>poll_votes.timestamp OR (excluded.timestamp=poll_votes.timestamp AND poll_votes.pending=1 AND excluded.pending=0)`,
		chat, v.PollID, v.Sender, v.Timestamp, v.Pending, data, chat, v.PollID)
	return err
}

func (s *Store) PollVotes(ctx context.Context, chat, id string) ([]model.PollVote, error) {
	chat = s.canonicalChatJID(ctx, chat)
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM poll_votes WHERE chat_jid=? AND poll_id=? ORDER BY sender`, chat, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.PollVote{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var vote model.PollVote
		if err := json.Unmarshal(data, &vote); err != nil {
			return nil, err
		}
		vote.ChatJID = chat
		result = append(result, vote)
	}
	return result, rows.Err()
}
