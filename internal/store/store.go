package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/shuki/whatsappgo/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	dsn := "file:" + path + separator + "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func OpenMemory() (*Store, error) {
	return Open("file:whatsappgo-test?mode=memory&cache=shared")
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS chats (
  jid TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  avatar_path TEXT NOT NULL DEFAULT '',
  last_message_id TEXT NOT NULL DEFAULT '',
  last_message_at INTEGER NOT NULL DEFAULT 0,
  last_message_preview TEXT NOT NULL DEFAULT '',
  unread_count INTEGER NOT NULL DEFAULT 0,
  muted_until INTEGER NOT NULL DEFAULT 0,
  pinned INTEGER NOT NULL DEFAULT 0,
  favorite INTEGER NOT NULL DEFAULT 0,
  archived INTEGER NOT NULL DEFAULT 0,
  is_group INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS chat_aliases (
  alias_jid TEXT PRIMARY KEY,
  canonical_jid TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_aliases_canonical ON chat_aliases(canonical_jid);
CREATE TABLE IF NOT EXISTS messages (
  id TEXT NOT NULL,
  chat_jid TEXT NOT NULL,
  sender_jid TEXT NOT NULL DEFAULT '',
  sender_name TEXT NOT NULL DEFAULT '',
  timestamp INTEGER NOT NULL,
  kind TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  from_me INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'received',
  reply_to TEXT NOT NULL DEFAULT '',
  edited INTEGER NOT NULL DEFAULT 0,
  revoked INTEGER NOT NULL DEFAULT 0,
  media_mime TEXT NOT NULL DEFAULT '',
  media_name TEXT NOT NULL DEFAULT '',
  media_path TEXT NOT NULL DEFAULT '',
  media_thumbnail TEXT NOT NULL DEFAULT '',
  media_size INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (chat_jid, id),
  FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_messages_chat_time ON messages(chat_jid, timestamp DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_body ON messages(body) WHERE body <> '';
CREATE TABLE IF NOT EXISTS media_payloads (
  chat_jid TEXT NOT NULL,
  message_id TEXT NOT NULL,
  payload BLOB NOT NULL,
  PRIMARY KEY (chat_jid, message_id),
  FOREIGN KEY (chat_jid, message_id) REFERENCES messages(chat_jid, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS reactions (
  chat_jid TEXT NOT NULL,
  message_id TEXT NOT NULL,
  sender_jid TEXT NOT NULL,
  emoji TEXT NOT NULL,
  timestamp INTEGER NOT NULL,
  PRIMARY KEY (chat_jid, message_id, sender_jid),
  FOREIGN KEY (chat_jid, message_id) REFERENCES messages(chat_jid, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS call_logs (
  id TEXT PRIMARY KEY,
  peer_jid TEXT NOT NULL DEFAULT '',
  timestamp INTEGER NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  incoming INTEGER NOT NULL DEFAULT 0,
  video INTEGER NOT NULL DEFAULT 0,
  result TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_call_logs_time ON call_logs(timestamp DESC, id DESC);
CREATE TABLE IF NOT EXISTS metadata (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}
	// Existing profiles predate chat filters. Keep migrations additive so their
	// message history and device identity remain untouched.
	return s.ensureChatFavoriteColumn(ctx)
}

func (s *Store) ensureChatFavoriteColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(chats)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == "favorite" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE chats ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0`)
	return err
}

func (s *Store) canonicalChatJID(ctx context.Context, jid string) string {
	if jid == "" {
		return jid
	}
	var canonical string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_jid FROM chat_aliases WHERE alias_jid=?`, jid).Scan(&canonical); err == nil && canonical != "" {
		return canonical
	}
	return jid
}

// LinkChatAliases consolidates WhatsApp's privacy-preserving LID and legacy
// phone-number JID into one local conversation without deleting history.
func (s *Store) LinkChatAliases(ctx context.Context, canonicalJID, aliasJID string) error {
	canonicalJID = strings.TrimSpace(canonicalJID)
	aliasJID = strings.TrimSpace(aliasJID)
	if canonicalJID == "" || aliasJID == "" || canonicalJID == aliasJID {
		return nil
	}
	if s.canonicalChatJID(ctx, aliasJID) == canonicalJID {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO chats
	 (jid,title,avatar_path,last_message_id,last_message_at,last_message_preview,unread_count,muted_until,pinned,favorite,archived,is_group)
	 SELECT ?,title,avatar_path,last_message_id,last_message_at,last_message_preview,unread_count,muted_until,pinned,favorite,archived,is_group
	 FROM chats WHERE jid=? ON CONFLICT(jid) DO NOTHING`, canonicalJID, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chats SET
	 title=CASE WHEN title='' THEN COALESCE((SELECT NULLIF(title,'') FROM chats WHERE jid=?),'') ELSE title END,
	 avatar_path=CASE WHEN avatar_path='' THEN COALESCE((SELECT NULLIF(avatar_path,'') FROM chats WHERE jid=?),'') ELSE avatar_path END,
	 last_message_id=CASE WHEN COALESCE((SELECT last_message_at FROM chats WHERE jid=?),0)>last_message_at THEN COALESCE((SELECT last_message_id FROM chats WHERE jid=?),'') ELSE last_message_id END,
	 last_message_preview=CASE WHEN COALESCE((SELECT last_message_at FROM chats WHERE jid=?),0)>last_message_at THEN COALESCE((SELECT last_message_preview FROM chats WHERE jid=?),'') ELSE last_message_preview END,
	 last_message_at=MAX(last_message_at,COALESCE((SELECT last_message_at FROM chats WHERE jid=?),0)),
	 unread_count=MAX(unread_count,COALESCE((SELECT unread_count FROM chats WHERE jid=?),0)),
	 muted_until=MAX(muted_until,COALESCE((SELECT muted_until FROM chats WHERE jid=?),0)),
	 pinned=MAX(pinned,COALESCE((SELECT pinned FROM chats WHERE jid=?),0)),
	 favorite=MAX(favorite,COALESCE((SELECT favorite FROM chats WHERE jid=?),0)),
	 archived=MIN(archived,COALESCE((SELECT archived FROM chats WHERE jid=?),archived)),
	 is_group=MAX(is_group,COALESCE((SELECT is_group FROM chats WHERE jid=?),0))
	 WHERE jid=?`, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID,
		aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, canonicalJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO messages
	 (id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size)
	 SELECT id,?,sender_jid,sender_name,timestamp,kind,body,from_me,status,reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size
	 FROM messages WHERE chat_jid=?`, canonicalJID, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO media_payloads(chat_jid,message_id,payload)
	 SELECT ?,message_id,payload FROM media_payloads WHERE chat_jid=?`, canonicalJID, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO reactions(chat_jid,message_id,sender_jid,emoji,timestamp)
	 SELECT ?,message_id,sender_jid,emoji,timestamp FROM reactions WHERE chat_jid=?`, canonicalJID, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chats WHERE jid=?`, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chat_aliases SET canonical_jid=? WHERE canonical_jid=?`, canonicalJID, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_aliases(alias_jid,canonical_jid) VALUES(?,?)
	 ON CONFLICT(alias_jid) DO UPDATE SET canonical_jid=excluded.canonical_jid`, aliasJID, canonicalJID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpsertMessage(ctx context.Context, msg model.Message, chatTitle string, incrementUnread bool) error {
	msg.ChatJID = s.canonicalChatJID(ctx, msg.ChatJID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	isGroup := strings.HasSuffix(msg.ChatJID, "@g.us")
	var alreadyExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE chat_jid=? AND id=?)`, msg.ChatJID, msg.ID).Scan(&alreadyExists); err != nil {
		return err
	}
	titleForInsert := chatTitle
	if titleForInsert == "" {
		titleForInsert = displayJID(msg.ChatJID)
	}
	unread := 0
	if incrementUnread && !msg.FromMe && !alreadyExists {
		unread = 1
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO chats
 (jid,title,last_message_id,last_message_at,last_message_preview,unread_count,is_group)
 VALUES(?,?,?,?,?,?,?)
 ON CONFLICT(jid) DO UPDATE SET
  title=CASE WHEN ? THEN excluded.title ELSE chats.title END,
  last_message_id=CASE WHEN excluded.last_message_at>=chats.last_message_at THEN excluded.last_message_id ELSE chats.last_message_id END,
  last_message_at=MAX(chats.last_message_at,excluded.last_message_at),
  last_message_preview=CASE WHEN excluded.last_message_at>=chats.last_message_at THEN excluded.last_message_preview ELSE chats.last_message_preview END,
  unread_count=chats.unread_count+excluded.unread_count,
  is_group=excluded.is_group`,
		msg.ChatJID, titleForInsert, msg.ID, msg.Timestamp, preview(msg), unread, isGroup, chatTitle != "")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO messages
 (id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(chat_jid,id) DO UPDATE SET
  sender_jid=excluded.sender_jid,sender_name=excluded.sender_name,timestamp=excluded.timestamp,
	  kind=excluded.kind,body=excluded.body,from_me=excluded.from_me,
	  status=CASE
	   WHEN messages.status='played' THEN messages.status
	   WHEN messages.status='read' AND excluded.status IN ('sent','delivered','received') THEN messages.status
	   WHEN messages.status='delivered' AND excluded.status IN ('sent','received') THEN messages.status
	   ELSE excluded.status END,
  reply_to=excluded.reply_to,edited=excluded.edited,revoked=excluded.revoked,
  media_mime=excluded.media_mime,media_name=excluded.media_name,
  -- A message that arrives again, for example through a history page, does not
  -- carry the local cache paths. Keeping the stored ones avoids losing an
  -- already downloaded file or an extracted preview.
  media_path=CASE WHEN excluded.media_path<>'' THEN excluded.media_path ELSE messages.media_path END,
  media_thumbnail=CASE WHEN excluded.media_thumbnail<>'' THEN excluded.media_thumbnail ELSE messages.media_thumbnail END,
  media_size=excluded.media_size`,
		msg.ID, msg.ChatJID, msg.SenderJID, msg.SenderName, msg.Timestamp, msg.Kind, msg.Body, msg.FromMe, msg.Status,
		msg.ReplyTo, msg.Edited, msg.Revoked, msg.MediaMIME, msg.MediaName, msg.MediaPath, msg.MediaThumbnail, msg.MediaSize)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListChats(ctx context.Context, limit, offset int, query string) ([]model.Chat, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := `WHERE c.archived=0
 AND c.jid NOT LIKE '%@broadcast'
	AND c.jid NOT LIKE '%@newsletter'
	AND (m.id IS NOT NULL OR c.pinned=1 OR c.favorite=1)`
	if strings.TrimSpace(query) != "" {
		where += " AND (c.title LIKE ? ESCAPE '\\' OR c.jid LIKE ? ESCAPE '\\')"
		q := "%" + escapeLike(strings.TrimSpace(query)) + "%"
		args = append(args, q, q)
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT c.jid,c.title,c.avatar_path,COALESCE(m.id,''),COALESCE(m.timestamp,c.last_message_at),
 CASE
	  WHEN m.id IS NULL THEN ''
  WHEN m.revoked=1 THEN 'Message deleted'
  WHEN TRIM(m.body)<>'' THEN TRIM(m.body)
  WHEN m.kind='image' THEN 'Image'
  WHEN m.kind='video' THEN 'Video'
  WHEN m.kind='audio' THEN 'Audio'
  WHEN m.kind='document' THEN 'Document'
  WHEN m.kind='sticker' THEN 'Sticker'
  ELSE 'Message'
 END,
 c.unread_count,c.muted_until,c.pinned,c.favorite,c.archived,c.is_group
	FROM chats c
	LEFT JOIN messages m ON m.chat_jid=c.jid AND m.id=(
  SELECT latest.id FROM messages latest
  WHERE latest.chat_jid=c.jid AND latest.kind NOT IN ('unknown','system')
  ORDER BY latest.timestamp DESC,latest.id DESC LIMIT 1
 ) `+where+`
	ORDER BY c.pinned DESC,COALESCE(m.timestamp,c.last_message_at) DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Chat, 0)
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.JID, &c.Title, &c.AvatarPath, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview,
			&c.UnreadCount, &c.MutedUntil, &c.Pinned, &c.Favorite, &c.Archived, &c.IsGroup); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *Store) GetChat(ctx context.Context, jid string) (model.Chat, error) {
	jid = s.canonicalChatJID(ctx, jid)
	var c model.Chat
	err := s.db.QueryRowContext(ctx, `SELECT jid,title,avatar_path,last_message_id,last_message_at,last_message_preview,
 unread_count,muted_until,pinned,favorite,archived,is_group FROM chats WHERE jid=?`, jid).Scan(&c.JID, &c.Title, &c.AvatarPath, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview, &c.UnreadCount, &c.MutedUntil, &c.Pinned, &c.Favorite, &c.Archived, &c.IsGroup)
	return c, err
}

func (s *Store) ListMessages(ctx context.Context, chatJID string, before int64, limit int) (model.MessagePage, error) {
	if chatJID == "" {
		return model.MessagePage{}, errors.New("chat_jid is required")
	}
	chatJID = s.canonicalChatJID(ctx, chatJID)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if before <= 0 {
		before = time.Now().Add(24 * time.Hour).UnixMilli()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.chat_jid,m.sender_jid,m.sender_name,m.timestamp,m.kind,m.body,m.from_me,m.status,
 m.reply_to,m.edited,m.revoked,m.media_mime,m.media_name,m.media_path,m.media_thumbnail,m.media_size,
 COALESCE(q.body,''),COALESCE(q.kind,''),COALESCE(q.sender_name,''),COALESCE(q.from_me,0)
 FROM messages m LEFT JOIN messages q ON q.chat_jid=m.chat_jid AND q.id=m.reply_to
 WHERE m.chat_jid=? AND m.timestamp<? AND m.kind<>'unknown' ORDER BY m.timestamp DESC,m.id DESC LIMIT ?`, chatJID, before, limit+1)
	if err != nil {
		return model.MessagePage{}, err
	}
	defer rows.Close()
	items := make([]model.Message, 0, limit+1)
	for rows.Next() {
		var m model.Message
		var quotedBody, quotedKind string
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe,
			&m.Status, &m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize,
			&quotedBody, &quotedKind, &m.ReplySender, &m.ReplyFromMe); err != nil {
			return model.MessagePage{}, err
		}
		if m.ReplyTo != "" {
			m.ReplyPreview = preview(model.Message{Kind: quotedKind, Body: quotedBody})
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return model.MessagePage{}, err
	}
	if err := rows.Close(); err != nil {
		return model.MessagePage{}, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	next := int64(0)
	if len(items) > 0 {
		next = items[0].Timestamp
	}
	if err := s.attachReactions(ctx, chatJID, items); err != nil {
		return model.MessagePage{}, err
	}
	return model.MessagePage{Messages: items, HasMore: hasMore, NextBefore: next}, nil
}

func (s *Store) GetMessage(ctx context.Context, chatJID, messageID string) (model.Message, error) {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	var m model.Message
	err := s.db.QueryRowContext(ctx, `SELECT id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,
 reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size
 FROM messages WHERE chat_jid=? AND id=?`, chatJID, messageID).Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe, &m.Status, &m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize)
	return m, err
}

func (s *Store) OldestMessage(ctx context.Context, chatJID string) (model.Message, error) {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	var m model.Message
	err := s.db.QueryRowContext(ctx, `SELECT id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,
 reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size
 FROM messages WHERE chat_jid=? AND kind<>'unknown' ORDER BY timestamp ASC,id ASC LIMIT 1`, chatJID).Scan(
		&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe, &m.Status,
		&m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize)
	return m, err
}

func (s *Store) SaveMediaPayload(ctx context.Context, chatJID, messageID string, payload []byte) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	if chatJID == "" || messageID == "" || len(payload) == 0 {
		return errors.New("chat_jid, message_id, and payload are required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO media_payloads(chat_jid,message_id,payload) VALUES(?,?,?)
 ON CONFLICT(chat_jid,message_id) DO UPDATE SET payload=excluded.payload`, chatJID, messageID, payload)
	return err
}

func (s *Store) MediaPayload(ctx context.Context, chatJID, messageID string) ([]byte, bool, error) {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM media_payloads WHERE chat_jid=? AND message_id=?`, chatJID, messageID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return payload, err == nil, err
}

func (s *Store) MarkChatRead(ctx context.Context, chatJID string) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET unread_count=0 WHERE jid=?`, chatJID)
	return err
}

// chatIdentityUpsertSQL creates a conversation row or merges its identity:
// title, avatar, latest activity time, and the group flag. The trailing
// parameter carries the caller's raw title so an empty title never replaces a
// known one. Conversation settings are inserted for new rows only.
const chatIdentityUpsertSQL = `INSERT INTO chats(jid,title,avatar_path,last_message_at,unread_count,muted_until,pinned,favorite,archived,is_group)
 VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(jid) DO UPDATE SET
 title=CASE WHEN ?<>'' THEN excluded.title ELSE chats.title END,
 avatar_path=CASE WHEN excluded.avatar_path<>'' THEN excluded.avatar_path ELSE chats.avatar_path END,
 last_message_at=MAX(chats.last_message_at,excluded.last_message_at),
 is_group=MAX(chats.is_group,excluded.is_group)`

// chatSnapshotUpsertSQL additionally applies WhatsApp's conversation settings.
// Unread counts never decrease because live messages may already have
// arrived; favorite is synchronised separately through app state.
const chatSnapshotUpsertSQL = chatIdentityUpsertSQL + `,
 unread_count=MAX(chats.unread_count,excluded.unread_count),
 muted_until=excluded.muted_until,pinned=excluded.pinned,archived=excluded.archived`

// UpsertChat ensures a conversation exists and merges its identity details.
// Callers such as directory sync, group metadata events, and contact
// resolution do not know the current mute, pin, archive, or unread state, so
// those settings are preserved on existing rows. Use ApplyChatSnapshot for
// authoritative conversation state from WhatsApp.
func (s *Store) UpsertChat(ctx context.Context, chat model.Chat) error {
	return s.upsertChat(ctx, chat, chatIdentityUpsertSQL)
}

// ApplyChatSnapshot stores an authoritative conversation snapshot, as
// delivered by an initial, recent, or full WhatsApp history sync. Mute, pin,
// and archive state replace the local values.
func (s *Store) ApplyChatSnapshot(ctx context.Context, chat model.Chat) error {
	return s.upsertChat(ctx, chat, chatSnapshotUpsertSQL)
}

func (s *Store) upsertChat(ctx context.Context, chat model.Chat, query string) error {
	if chat.JID == "" {
		return errors.New("chat jid is required")
	}
	chat.JID = s.canonicalChatJID(ctx, chat.JID)
	title := strings.TrimSpace(chat.Title)
	titleForInsert := title
	if titleForInsert == "" {
		titleForInsert = displayJID(chat.JID)
	}
	_, err := s.db.ExecContext(ctx, query,
		chat.JID, titleForInsert, chat.AvatarPath, chat.LastMessageAt, chat.UnreadCount, chat.MutedUntil, chat.Pinned, chat.Favorite, chat.Archived, chat.IsGroup, title)
	return err
}

func (s *Store) UpdateChatTitle(ctx context.Context, jid, title string) error {
	if jid == "" || strings.TrimSpace(title) == "" {
		return nil
	}
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET title=? WHERE jid=?`, title, jid)
	return err
}

func (s *Store) UpdateChatAvatar(ctx context.Context, jid, path string) error {
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET avatar_path=? WHERE jid=?`, path, jid)
	return err
}

func (s *Store) UpdateChatPinned(ctx context.Context, jid string, pinned bool) error {
	if jid == "" {
		return errors.New("chat jid is required")
	}
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `INSERT INTO chats(jid,title,pinned) VALUES(?,?,?)
 ON CONFLICT(jid) DO UPDATE SET pinned=excluded.pinned`, jid, displayJID(jid), pinned)
	return err
}

func (s *Store) UpdateChatFavorites(ctx context.Context, jids []string) error {
	for i := range jids {
		jids[i] = s.canonicalChatJID(ctx, jids[i])
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE chats SET favorite=0 WHERE favorite<>0`); err != nil {
		return err
	}
	for _, jid := range jids {
		jid = strings.TrimSpace(jid)
		if jid == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO chats(jid,title,favorite) VALUES(?,?,1)
		 ON CONFLICT(jid) DO UPDATE SET favorite=1`, jid, displayJID(jid)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateReceipt(ctx context.Context, chatJID string, ids []string, status string) error {
	if len(ids) == 0 {
		return nil
	}
	chatJID = s.canonicalChatJID(ctx, chatJID)
	if status != "sent" && status != "delivered" && status != "read" && status != "played" {
		return fmt.Errorf("invalid receipt status %q", status)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	args = append(args, status, chatJID)
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET status=? WHERE chat_jid=? AND id IN (`+placeholders+`)`, args...)
	return err
}

func (s *Store) UpsertCallLog(ctx context.Context, call model.CallLog) error {
	if call.ID == "" {
		return errors.New("call id is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO call_logs(id,peer_jid,timestamp,duration,incoming,video,result)
 VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
 peer_jid=excluded.peer_jid,timestamp=excluded.timestamp,duration=excluded.duration,
 incoming=excluded.incoming,video=excluded.video,result=excluded.result`,
		call.ID, call.PeerJID, call.Timestamp, call.Duration, call.Incoming, call.Video, call.Result)
	return err
}

func (s *Store) ListCallLogs(ctx context.Context, limit int) ([]model.CallLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.peer_jid,COALESCE(c.title,''),COALESCE(c.avatar_path,''),
 l.timestamp,l.duration,l.incoming,l.video,l.result
 FROM call_logs l LEFT JOIN chats c ON c.jid=l.peer_jid
 ORDER BY l.timestamp DESC,l.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.CallLog, 0)
	for rows.Next() {
		var call model.CallLog
		if err := rows.Scan(&call.ID, &call.PeerJID, &call.PeerName, &call.PeerAvatarPath, &call.Timestamp, &call.Duration, &call.Incoming, &call.Video, &call.Result); err != nil {
			return nil, err
		}
		result = append(result, call)
	}
	return result, rows.Err()
}

func (s *Store) Metadata(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Store) SetMetadata(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO metadata(key,value) VALUES(?,?)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) UpsertReaction(ctx context.Context, r model.Reaction) error {
	r.ChatJID = s.canonicalChatJID(ctx, r.ChatJID)
	if r.Emoji == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM reactions WHERE chat_jid=? AND message_id=? AND sender_jid=?`, r.ChatJID, r.MessageID, r.SenderJID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO reactions(chat_jid,message_id,sender_jid,emoji,timestamp) VALUES(?,?,?,?,?)
 ON CONFLICT(chat_jid,message_id,sender_jid) DO UPDATE SET emoji=excluded.emoji,timestamp=excluded.timestamp`,
		r.ChatJID, r.MessageID, r.SenderJID, r.Emoji, r.Timestamp)
	return err
}

func (s *Store) MarkRevoked(ctx context.Context, chatJID, messageID string) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET revoked=1,body='',kind='revoked' WHERE chat_jid=? AND id=?`, chatJID, messageID)
	return err
}

func (s *Store) EditMessage(ctx context.Context, chatJID, messageID, body string) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	result, err := s.db.ExecContext(ctx, `UPDATE messages SET body=?,edited=1 WHERE chat_jid=? AND id=? AND revoked=0`, body, chatJID, messageID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("message not found or already revoked")
	}
	return nil
}

// attachReactions loads the reactions for one page of messages with a single
// query. chatJID must already be canonical, as it is inside ListMessages.
func (s *Store) attachReactions(ctx context.Context, chatJID string, messages []model.Message) error {
	if len(messages) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(messages)), ",")
	args := make([]any, 0, len(messages)+1)
	args = append(args, chatJID)
	index := make(map[string]int, len(messages))
	for i, message := range messages {
		args = append(args, message.ID)
		index[message.ID] = i
	}
	rows, err := s.db.QueryContext(ctx, `SELECT chat_jid,message_id,sender_jid,emoji,timestamp FROM reactions
 WHERE chat_jid=? AND message_id IN (`+placeholders+`) ORDER BY timestamp,sender_jid`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r model.Reaction
		if err := rows.Scan(&r.ChatJID, &r.MessageID, &r.SenderJID, &r.Emoji, &r.Timestamp); err != nil {
			return err
		}
		if i, ok := index[r.MessageID]; ok {
			messages[i].Reactions = append(messages[i].Reactions, r)
		}
	}
	return rows.Err()
}

func (s *Store) SearchMessages(ctx context.Context, query string, limit int) ([]model.Message, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []model.Message{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,
 reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size
 FROM messages WHERE body LIKE ? ESCAPE '\' ORDER BY timestamp DESC LIMIT ?`, "%"+escapeLike(query)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe, &m.Status, &m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func preview(m model.Message) string {
	if m.Revoked {
		return "Message deleted"
	}
	if strings.TrimSpace(m.Body) != "" {
		return strings.TrimSpace(m.Body)
	}
	switch m.Kind {
	case "image":
		return "Image"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	case "document":
		return "Document"
	case "sticker":
		return "Sticker"
	}
	return "Message"
}

func displayJID(jid string) string {
	if at := strings.IndexByte(jid, '@'); at > 0 {
		return jid[:at]
	}
	return jid
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	return strings.ReplaceAll(s, "_", "\\_")
}

// MediaCursor marks how far a media scan has progressed.
type MediaCursor struct {
	ChatJID   string
	MessageID string
}

// PendingThumbnail is a stored media message whose inline preview has not been
// extracted yet, together with the original message payload it came in.
type PendingThumbnail struct {
	ChatJID   string
	MessageID string
	Payload   []byte
}

// MessagesMissingThumbnails returns stored media messages that have no cached
// preview, ordered so that the returned cursor can continue the scan.
func (s *Store) MessagesMissingThumbnails(ctx context.Context, after MediaCursor, limit int) ([]PendingThumbnail, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.chat_jid,m.id,p.payload
 FROM messages m JOIN media_payloads p ON p.chat_jid=m.chat_jid AND p.message_id=m.id
 WHERE m.media_thumbnail='' AND m.kind IN ('image','video','sticker','document')
  AND (m.chat_jid,m.id) > (?,?)
 ORDER BY m.chat_jid,m.id LIMIT ?`, after.ChatJID, after.MessageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PendingThumbnail, 0, limit)
	for rows.Next() {
		var item PendingThumbnail
		if err := rows.Scan(&item.ChatJID, &item.MessageID, &item.Payload); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// UpdateMediaThumbnail records the cached preview picture for one message.
func (s *Store) UpdateMediaThumbnail(ctx context.Context, chatJID, messageID, path string) error {
	if chatJID == "" || messageID == "" {
		return errors.New("chat_jid and message_id are required")
	}
	chatJID = s.canonicalChatJID(ctx, chatJID)
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET media_thumbnail=? WHERE chat_jid=? AND id=?`, path, chatJID, messageID)
	return err
}
