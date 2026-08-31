package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/shukiv/whatsappgo/internal/model"
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
  local_title TEXT NOT NULL DEFAULT '',
  avatar_path TEXT NOT NULL DEFAULT '',
  last_message_id TEXT NOT NULL DEFAULT '',
  last_message_at INTEGER NOT NULL DEFAULT 0,
  last_message_preview TEXT NOT NULL DEFAULT '',
  unread_count INTEGER NOT NULL DEFAULT 0,
  read_through_at INTEGER NOT NULL DEFAULT 0,
  muted_until INTEGER NOT NULL DEFAULT 0,
  pinned INTEGER NOT NULL DEFAULT 0,
  pinned_at INTEGER NOT NULL DEFAULT 0,
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
	// Existing profiles predate chat filters and link previews. Keep migrations
	// additive so their message history and device identity remain untouched.
	if err := s.ensureColumn(ctx, "chats", "favorite", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "chats", "pinned_at", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "chats", "read_through_at", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "chats", "local_title", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	for _, column := range []string{"link_url", "link_title", "link_description", "link_thumbnail"} {
		if err := s.ensureColumn(ctx, "messages", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "messages", "media_duration", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "messages", "audio_waveform", "BLOB"); err != nil {
		return err
	}
	for _, column := range []string{"contact_name", "contact_phone"} {
		if err := s.ensureColumn(ctx, "messages", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "messages", "contact_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	for _, column := range []string{"latitude", "longitude"} {
		if err := s.ensureColumn(ctx, "messages", column, "REAL NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	return s.repairHistoricalUnreadReplay(ctx)
}

const unreadReplayRepairMetadataKey = "unread_full_sync_repair_v1"

// v5 of the chat-settings backfill treated historical markChatAsRead app-state
// entries as the current unread list. WhatsApp replays the last explicit action
// there, even if a chat was later read normally, so that migration could create
// stale badges and synthetic read boundaries. Clear that derived cache once;
// missed messages and receipts are replayed by WhatsApp after connection and
// rebuild the live unread state.
func (s *Store) repairHistoricalUnreadReplay(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var repaired, affected bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM metadata WHERE key=?)`, unreadReplayRepairMetadataKey).Scan(&repaired); err != nil {
		return err
	}
	if repaired {
		return tx.Commit()
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM metadata WHERE key='chat_settings_app_state_backfill_v5')`).Scan(&affected); err != nil {
		return err
	}
	if affected {
		if _, err := tx.ExecContext(ctx, `UPDATE chats SET unread_count=0,read_through_at=0`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE messages SET status='received' WHERE from_me=0 AND status IN ('read','played')`); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata(key,value) VALUES(?,?)`, unreadReplayRepairMetadataKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// packWaveform stores the amplitude bars compactly. Each bar is a percentage,
// so one byte each is enough.
func packWaveform(bars []int) []byte {
	if len(bars) == 0 {
		return nil
	}
	packed := make([]byte, 0, len(bars))
	for _, bar := range bars {
		if bar < 0 {
			bar = 0
		}
		if bar > 100 {
			bar = 100
		}
		packed = append(packed, byte(bar))
	}
	return packed
}

func unpackWaveform(packed []byte) []int {
	if len(packed) == 0 {
		return nil
	}
	bars := make([]int, 0, len(packed))
	for _, value := range packed {
		bars = append(bars, int(value))
	}
	return bars
}

// ensureColumn adds a column when an older profile database lacks it.
func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	// The table and column names are constants in this file, never user input.
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition)
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
	 (jid,title,local_title,avatar_path,last_message_id,last_message_at,last_message_preview,unread_count,read_through_at,muted_until,pinned,pinned_at,favorite,archived,is_group)
	 SELECT ?,title,local_title,avatar_path,last_message_id,last_message_at,last_message_preview,unread_count,read_through_at,muted_until,pinned,pinned_at,favorite,archived,is_group
	 FROM chats WHERE jid=? ON CONFLICT(jid) DO NOTHING`, canonicalJID, aliasJID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chats SET
	 title=CASE WHEN title='' THEN COALESCE((SELECT NULLIF(title,'') FROM chats WHERE jid=?),'') ELSE title END,
	 local_title=CASE WHEN local_title='' THEN COALESCE((SELECT NULLIF(local_title,'') FROM chats WHERE jid=?),'') ELSE local_title END,
	 avatar_path=CASE WHEN avatar_path='' THEN COALESCE((SELECT NULLIF(avatar_path,'') FROM chats WHERE jid=?),'') ELSE avatar_path END,
	 last_message_id=CASE WHEN COALESCE((SELECT last_message_at FROM chats WHERE jid=?),0)>last_message_at THEN COALESCE((SELECT last_message_id FROM chats WHERE jid=?),'') ELSE last_message_id END,
	 last_message_preview=CASE WHEN COALESCE((SELECT last_message_at FROM chats WHERE jid=?),0)>last_message_at THEN COALESCE((SELECT last_message_preview FROM chats WHERE jid=?),'') ELSE last_message_preview END,
	 last_message_at=MAX(last_message_at,COALESCE((SELECT last_message_at FROM chats WHERE jid=?),0)),
	 unread_count=MAX(unread_count,COALESCE((SELECT unread_count FROM chats WHERE jid=?),0)),
	 read_through_at=MAX(read_through_at,COALESCE((SELECT read_through_at FROM chats WHERE jid=?),0)),
	 muted_until=MAX(muted_until,COALESCE((SELECT muted_until FROM chats WHERE jid=?),0)),
	 pinned=MAX(pinned,COALESCE((SELECT pinned FROM chats WHERE jid=?),0)),
	 pinned_at=MAX(pinned_at,COALESCE((SELECT pinned_at FROM chats WHERE jid=?),0)),
	 favorite=MAX(favorite,COALESCE((SELECT favorite FROM chats WHERE jid=?),0)),
	 archived=MIN(archived,COALESCE((SELECT archived FROM chats WHERE jid=?),archived)),
	 is_group=MAX(is_group,COALESCE((SELECT is_group FROM chats WHERE jid=?),0))
	 WHERE jid=?`, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID,
		aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, aliasJID, canonicalJID); err != nil {
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
  title=CASE WHEN ? AND (TRIM(chats.title)='' OR chats.title=substr(chats.jid,1,instr(chats.jid,'@')-1)) THEN excluded.title ELSE chats.title END,
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
 (id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size,link_url,link_title,link_description,link_thumbnail,media_duration,audio_waveform,contact_name,contact_phone,contact_count,latitude,longitude)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
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
  media_size=excluded.media_size,
  link_url=CASE WHEN excluded.link_url<>'' THEN excluded.link_url ELSE messages.link_url END,
  link_title=CASE WHEN excluded.link_url<>'' THEN excluded.link_title ELSE messages.link_title END,
  link_description=CASE WHEN excluded.link_url<>'' THEN excluded.link_description ELSE messages.link_description END,
  link_thumbnail=CASE WHEN excluded.link_thumbnail<>'' THEN excluded.link_thumbnail ELSE messages.link_thumbnail END,
  media_duration=CASE WHEN excluded.media_duration>0 THEN excluded.media_duration ELSE messages.media_duration END,
  audio_waveform=CASE WHEN excluded.audio_waveform IS NOT NULL THEN excluded.audio_waveform ELSE messages.audio_waveform END,
  contact_name=CASE WHEN excluded.contact_name<>'' THEN excluded.contact_name ELSE messages.contact_name END,
  contact_phone=CASE WHEN excluded.contact_phone<>'' THEN excluded.contact_phone ELSE messages.contact_phone END,
  contact_count=MAX(messages.contact_count,excluded.contact_count),
  latitude=CASE WHEN excluded.latitude<>0 THEN excluded.latitude ELSE messages.latitude END,
  longitude=CASE WHEN excluded.longitude<>0 THEN excluded.longitude ELSE messages.longitude END`,
		msg.ID, msg.ChatJID, msg.SenderJID, msg.SenderName, msg.Timestamp, msg.Kind, msg.Body, msg.FromMe, msg.Status,
		msg.ReplyTo, msg.Edited, msg.Revoked, msg.MediaMIME, msg.MediaName, msg.MediaPath, msg.MediaThumbnail, msg.MediaSize,
		msg.LinkURL, msg.LinkTitle, msg.LinkDescription, msg.LinkThumbnail, msg.MediaDuration, packWaveform(msg.AudioWaveform),
		msg.ContactName, msg.ContactPhone, msg.ContactCount, msg.Latitude, msg.Longitude)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListChats(ctx context.Context, limit, offset int, query string) ([]model.Chat, error) {
	return s.listChats(ctx, limit, offset, query, false)
}

// ListArchivedChats returns the conversations that were put away.
func (s *Store) ListArchivedChats(ctx context.Context, limit, offset int, query string) ([]model.Chat, error) {
	return s.listChats(ctx, limit, offset, query, true)
}

func (s *Store) listChats(ctx context.Context, limit, offset int, query string, archived bool) ([]model.Chat, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	args := []any{archived}
	where := `WHERE c.archived=? 
 AND c.jid NOT LIKE '%@broadcast'
	AND c.jid NOT LIKE '%@newsletter'
	AND (m.id IS NOT NULL OR c.pinned=1 OR c.favorite=1)`
	if strings.TrimSpace(query) != "" {
		where += " AND (c.local_title LIKE ? ESCAPE '\\' OR c.title LIKE ? ESCAPE '\\' OR c.jid LIKE ? ESCAPE '\\')"
		q := "%" + escapeLike(strings.TrimSpace(query)) + "%"
		args = append(args, q, q, q)
	}
	args = append(args, limit, offset)
	// A conversation that was only ever synced from history often has no name:
	// the address book never knew the number, and no live message has set one.
	// The name the other side publishes is far better than a placeholder built
	// from their identifier, so it is used when nothing better is stored.
	rows, err := s.db.QueryContext(ctx, `SELECT c.jid,
	CASE WHEN TRIM(c.local_title)<>'' THEN c.local_title
	     WHEN TRIM(c.title)<>'' AND c.title<>substr(c.jid,1,instr(c.jid,'@')-1) THEN c.title
      ELSE COALESCE((SELECT TRIM(s.sender_name) FROM messages s
                     WHERE s.chat_jid=c.jid AND s.from_me=0 AND TRIM(s.sender_name)<>''
                     ORDER BY s.timestamp DESC LIMIT 1), c.title) END,
 c.avatar_path,COALESCE(m.id,''),COALESCE(m.timestamp,c.last_message_at),
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
 c.unread_count,c.muted_until,c.pinned,c.pinned_at,c.favorite,c.archived,c.is_group
	FROM chats c
	LEFT JOIN messages m ON m.chat_jid=c.jid AND m.id=(
  SELECT latest.id FROM messages latest
  WHERE latest.chat_jid=c.jid AND latest.kind NOT IN ('unknown','system','')
  ORDER BY latest.timestamp DESC,latest.id DESC LIMIT 1
 ) `+where+`
	ORDER BY c.pinned DESC,c.pinned_at DESC,COALESCE(m.timestamp,c.last_message_at) DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Chat, 0)
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.JID, &c.Title, &c.AvatarPath, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview,
			&c.UnreadCount, &c.MutedUntil, &c.Pinned, &c.PinnedAt, &c.Favorite, &c.Archived, &c.IsGroup); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *Store) GetChat(ctx context.Context, jid string) (model.Chat, error) {
	jid = s.canonicalChatJID(ctx, jid)
	var c model.Chat
	err := s.db.QueryRowContext(ctx, `SELECT jid,COALESCE(NULLIF(local_title,''),title),avatar_path,last_message_id,last_message_at,last_message_preview,
	 unread_count,muted_until,pinned,pinned_at,favorite,archived,is_group FROM chats WHERE jid=?`, jid).Scan(&c.JID, &c.Title, &c.AvatarPath, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview, &c.UnreadCount, &c.MutedUntil, &c.Pinned, &c.PinnedAt, &c.Favorite, &c.Archived, &c.IsGroup)
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
 m.link_url,m.link_title,m.link_description,m.link_thumbnail,m.media_duration,m.audio_waveform,
 m.contact_name,m.contact_phone,m.contact_count,m.latitude,m.longitude,
 COALESCE(q.body,''),COALESCE(q.kind,''),COALESCE(q.sender_name,''),COALESCE(q.from_me,0)
 FROM messages m LEFT JOIN messages q ON q.chat_jid=m.chat_jid AND q.id=m.reply_to
 WHERE m.chat_jid=? AND m.timestamp<? AND m.kind NOT IN ('unknown','') ORDER BY m.timestamp DESC,m.id DESC LIMIT ?`, chatJID, before, limit+1)
	if err != nil {
		return model.MessagePage{}, err
	}
	defer rows.Close()
	items := make([]model.Message, 0, limit+1)
	for rows.Next() {
		var m model.Message
		var quotedBody, quotedKind string
		var waveform []byte
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe,
			&m.Status, &m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize,
			&m.LinkURL, &m.LinkTitle, &m.LinkDescription, &m.LinkThumbnail, &m.MediaDuration, &waveform,
			&m.ContactName, &m.ContactPhone, &m.ContactCount, &m.Latitude, &m.Longitude,
			&quotedBody, &quotedKind, &m.ReplySender, &m.ReplyFromMe); err != nil {
			return model.MessagePage{}, err
		}
		m.AudioWaveform = unpackWaveform(waveform)
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
	var waveform []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,
 reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size,
 link_url,link_title,link_description,link_thumbnail,media_duration,audio_waveform,
 contact_name,contact_phone,contact_count,latitude,longitude
 FROM messages WHERE chat_jid=? AND id=?`, chatJID, messageID).Scan(&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe, &m.Status, &m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize, &m.LinkURL, &m.LinkTitle, &m.LinkDescription, &m.LinkThumbnail, &m.MediaDuration, &waveform,
		&m.ContactName, &m.ContactPhone, &m.ContactCount, &m.Latitude, &m.Longitude)
	m.AudioWaveform = unpackWaveform(waveform)
	return m, err
}

func (s *Store) OldestMessage(ctx context.Context, chatJID string) (model.Message, error) {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	var m model.Message
	err := s.db.QueryRowContext(ctx, `SELECT id,chat_jid,sender_jid,sender_name,timestamp,kind,body,from_me,status,
 reply_to,edited,revoked,media_mime,media_name,media_path,media_thumbnail,media_size
 FROM messages WHERE chat_jid=? AND kind NOT IN ('unknown','') ORDER BY timestamp ASC,id ASC LIMIT 1`, chatJID).Scan(
		&m.ID, &m.ChatJID, &m.SenderJID, &m.SenderName, &m.Timestamp, &m.Kind, &m.Body, &m.FromMe, &m.Status,
		&m.ReplyTo, &m.Edited, &m.Revoked, &m.MediaMIME, &m.MediaName, &m.MediaPath, &m.MediaThumbnail, &m.MediaSize)
	return m, err
}

// NewestMessage is the anchor WhatsApp wants when a conversation is archived
// or marked read: its app-state patches carry the last message's key.
func (s *Store) NewestMessage(ctx context.Context, chatJID string) (model.Message, error) {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	var m model.Message
	err := s.db.QueryRowContext(ctx, `SELECT id,chat_jid,sender_jid,timestamp,from_me
 FROM messages WHERE chat_jid=? AND kind NOT IN ('unknown','') ORDER BY timestamp DESC,id DESC LIMIT 1`, chatJID).Scan(
		&m.ID, &m.ChatJID, &m.SenderJID, &m.Timestamp, &m.FromMe)
	return m, err
}

// ArchivedChatCount reports how many conversations are put away.
func (s *Store) ArchivedChatCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chats c WHERE c.archived=1
 AND c.jid NOT LIKE '%@broadcast' AND c.jid NOT LIKE '%@newsletter'`).Scan(&count)
	return count, err
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
	return s.MarkChatReadThrough(ctx, chatJID, time.Now().UnixMilli())
}

// MarkChatReadThrough applies a historical linked-device read action only when
// it covers the newest activity stored for the chat. Full app-state replay
// contains old actions which must not clear messages that arrived afterward.
func (s *Store) MarkChatReadThrough(ctx context.Context, chatJID string, through int64) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	if through <= 0 {
		through = time.Now().UnixMilli()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE messages SET status='read'
	 WHERE chat_jid=? AND from_me=0 AND timestamp<=? AND status NOT IN ('read','played')`, chatJID, through); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chats SET
	 unread_count=CASE
	  WHEN EXISTS(SELECT 1 FROM messages r WHERE r.chat_jid=chats.jid AND r.from_me=0 AND r.status IN ('read','played'))
	  THEN (SELECT COUNT(*) FROM messages m WHERE m.chat_jid=chats.jid AND m.from_me=0
	        AND m.kind NOT IN ('unknown','system','') AND m.timestamp>(
	          SELECT MAX(r.timestamp) FROM messages r WHERE r.chat_jid=chats.jid AND r.from_me=0 AND r.status IN ('read','played')))
	  WHEN last_message_at<=? THEN 0 ELSE unread_count END,
	 read_through_at=MAX(read_through_at,?)
	 WHERE jid=?`, through, through, chatJID); err != nil {
		return err
	}
	return tx.Commit()
}

// RecalculateUnreadCounts repairs cached chat totals from receipt state while
// preserving WhatsApp snapshot totals for chats that have no local incoming
// read boundary. Outgoing read receipts are deliberately ignored.
func (s *Store) RecalculateUnreadCounts(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET unread_count=(
	 SELECT COUNT(*) FROM messages m WHERE m.chat_jid=chats.jid AND m.from_me=0
	 AND m.kind NOT IN ('unknown','system','') AND m.timestamp>(
	  SELECT MAX(r.timestamp) FROM messages r WHERE r.chat_jid=chats.jid AND r.from_me=0 AND r.status IN ('read','played')))
	 WHERE EXISTS(SELECT 1 FROM messages r WHERE r.chat_jid=chats.jid AND r.from_me=0 AND r.status IN ('read','played'))`)
	return err
}

func (s *Store) RecalculateChatUnread(ctx context.Context, chatJID string) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET unread_count=(
	 SELECT COUNT(*) FROM messages m WHERE m.chat_jid=chats.jid AND m.from_me=0
	 AND m.kind NOT IN ('unknown','system','') AND m.timestamp>(
	  SELECT MAX(r.timestamp) FROM messages r WHERE r.chat_jid=chats.jid AND r.from_me=0 AND r.status IN ('read','played')))
	 WHERE jid=? AND EXISTS(SELECT 1 FROM messages r WHERE r.chat_jid=chats.jid AND r.from_me=0 AND r.status IN ('read','played'))`, chatJID)
	return err
}

// chatIdentityUpsertSQL creates a conversation row or merges its identity:
// title, avatar, latest activity time, and the group flag. The trailing
// parameter carries the caller's raw title so an empty title never replaces a
// known one. Conversation settings are inserted for new rows only.
const chatIdentityUpsertSQL = `INSERT INTO chats(jid,title,avatar_path,last_message_at,unread_count,muted_until,pinned,pinned_at,favorite,archived,is_group)
 VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(jid) DO UPDATE SET
 title=CASE WHEN ?<>'' THEN excluded.title ELSE chats.title END,
 avatar_path=CASE WHEN excluded.avatar_path<>'' THEN excluded.avatar_path ELSE chats.avatar_path END,
 last_message_at=MAX(chats.last_message_at,excluded.last_message_at),
 is_group=MAX(chats.is_group,excluded.is_group)`

// chatSnapshotUpsertSQL additionally applies WhatsApp's conversation settings.
// A snapshot may lower the unread count when it covers at least the newest
// activity already stored. An older snapshot must not erase newer live
// messages; favorite is synchronised separately through app state.
const chatSnapshotUpsertSQL = chatIdentityUpsertSQL + `,
 unread_count=CASE WHEN excluded.last_message_at>=chats.last_message_at THEN excluded.unread_count ELSE chats.unread_count END,
 muted_until=excluded.muted_until,pinned=excluded.pinned,pinned_at=excluded.pinned_at,archived=excluded.archived`

// UpsertChat ensures a conversation exists and merges its identity details.
// Callers such as directory sync, group metadata events, and contact
// resolution do not know the current mute, pin, archive, or unread state, so
// those settings are preserved on existing rows. Use ApplyChatSnapshot for
// authoritative conversation state from WhatsApp.
// MarkChatUnread puts one unread message back so the conversation shows as
// unread, which is what "mark as unread" means in the interface.
func (s *Store) MarkChatUnread(ctx context.Context, chatJID string) error {
	chatJID = s.canonicalChatJID(ctx, chatJID)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET unread_count=MAX(unread_count,1) WHERE jid=?`, chatJID)
	return err
}

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
		chat.JID, titleForInsert, chat.AvatarPath, chat.LastMessageAt, chat.UnreadCount, chat.MutedUntil, chat.Pinned, chat.PinnedAt, chat.Favorite, chat.Archived, chat.IsGroup, title)
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

// SetLocalChatTitle stores an operator-supplied label separately from names
// synchronized by WhatsApp, so future directory/history updates cannot replace
// it. The linked-device protocol cannot write the phone's address book.
func (s *Store) SetLocalChatTitle(ctx context.Context, jid, title string) error {
	title = strings.TrimSpace(title)
	if jid == "" || title == "" {
		return errors.New("jid and title are required")
	}
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET local_title=? WHERE jid=?`, title, jid)
	return err
}

// UpdateChatTitleIfPlaceholder applies a weak identity (for example a push or
// verified-business name) only while the stored title is still the raw JID.
// Saved address-book names are stronger and must never be replaced here.
func (s *Store) UpdateChatTitleIfPlaceholder(ctx context.Context, jid, title string) error {
	title = strings.TrimSpace(title)
	if jid == "" || title == "" {
		return nil
	}
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET title=?
	 WHERE jid=? AND (TRIM(title)='' OR title=substr(jid,1,instr(jid,'@')-1))`, title, jid)
	return err
}

// ListPlaceholderContactJIDs returns direct conversations whose only local
// identity is their protocol identifier. The WhatsApp client can enrich these
// in one batched user-info request after connecting.
func (s *Store) ListPlaceholderContactJIDs(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT jid FROM chats
	 WHERE is_group=0
	   AND (jid LIKE '%@lid' OR jid LIKE '%@s.whatsapp.net')
	   AND (TRIM(title)='' OR title=substr(jid,1,instr(jid,'@')-1))
	 ORDER BY last_message_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var jid string
		if err := rows.Scan(&jid); err != nil {
			return nil, err
		}
		result = append(result, jid)
	}
	return result, rows.Err()
}

func (s *Store) UpdateChatAvatar(ctx context.Context, jid, path string) error {
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `UPDATE chats SET avatar_path=? WHERE jid=?`, path, jid)
	return err
}

func (s *Store) UpdateChatPinned(ctx context.Context, jid string, pinned bool) error {
	pinnedAt := int64(0)
	if pinned {
		pinnedAt = time.Now().UnixMilli()
	}
	return s.UpdateChatPinnedAt(ctx, jid, pinned, pinnedAt)
}

// UpdateChatPinnedAt stores both the pin state and WhatsApp's action time so
// multiple pinned chats appear in the same order as on other linked devices.
func (s *Store) UpdateChatPinnedAt(ctx context.Context, jid string, pinned bool, pinnedAt int64) error {
	if jid == "" {
		return errors.New("chat jid is required")
	}
	jid = s.canonicalChatJID(ctx, jid)
	if !pinned {
		pinnedAt = 0
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO chats(jid,title,pinned,pinned_at) VALUES(?,?,?,?)
	 ON CONFLICT(jid) DO UPDATE SET pinned=excluded.pinned,pinned_at=excluded.pinned_at`, jid, displayJID(jid), pinned, pinnedAt)
	return err
}

func (s *Store) UpdateChatMuted(ctx context.Context, jid string, mutedUntil int64) error {
	if jid == "" {
		return errors.New("chat jid is required")
	}
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `INSERT INTO chats(jid,title,muted_until) VALUES(?,?,?)
 ON CONFLICT(jid) DO UPDATE SET muted_until=excluded.muted_until`, jid, displayJID(jid), mutedUntil)
	return err
}

func (s *Store) UpdateChatArchived(ctx context.Context, jid string, archived bool) error {
	if jid == "" {
		return errors.New("chat jid is required")
	}
	jid = s.canonicalChatJID(ctx, jid)
	_, err := s.db.ExecContext(ctx, `INSERT INTO chats(jid,title,archived) VALUES(?,?,?)
 ON CONFLICT(jid) DO UPDATE SET archived=excluded.archived`, jid, displayJID(jid), archived)
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
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.peer_jid,COALESCE(NULLIF(c.local_title,''),c.title,''),COALESCE(c.avatar_path,''),
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

// PendingLinkPreview is a stored text message whose YouTube card may need the
// thumbnail and metadata that were unavailable in the original history sync.
type PendingLinkPreview struct {
	ChatJID   string
	MessageID string
	Body      string
	LinkURL   string
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

// MessagesForLinkPreviewBackfill returns link-bearing text messages. The
// caller restricts the scan to YouTube and may replace an older low-resolution
// cached card as well as fill a missing one.
func (s *Store) MessagesForLinkPreviewBackfill(ctx context.Context, after MediaCursor, limit int) ([]PendingLinkPreview, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT chat_jid,id,body,link_url FROM messages
 WHERE kind='text' AND (link_url<>'' OR body LIKE '%http://%' OR body LIKE '%https://%')
  AND (chat_jid,id) > (?,?)
 ORDER BY chat_jid,id LIMIT ?`, after.ChatJID, after.MessageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PendingLinkPreview, 0, limit)
	for rows.Next() {
		var item PendingLinkPreview
		if err := rows.Scan(&item.ChatJID, &item.MessageID, &item.Body, &item.LinkURL); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// UpdateLinkPreview replaces a message's cached card without changing its
// body, timestamp, delivery state, or conversation ordering.
func (s *Store) UpdateLinkPreview(ctx context.Context, chatJID, messageID, rawURL, title, description, thumbnail string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET
 link_url=CASE WHEN ?<>'' THEN ? ELSE link_url END,
 link_title=CASE WHEN ?<>'' THEN ? ELSE link_title END,
 link_description=CASE WHEN ?<>'' THEN ? ELSE link_description END,
 link_thumbnail=CASE WHEN ?<>'' THEN ? ELSE link_thumbnail END
 WHERE chat_jid=? AND id=?`, rawURL, rawURL, title, title, description, description,
		thumbnail, thumbnail, chatJID, messageID)
	return err
}

// MessageCursor marks how far a newest-first scan has progressed.
type MessageCursor struct {
	Timestamp int64
	MessageID string
}

// PendingMedia is an attachment that has not been downloaded yet.
type PendingMedia struct {
	ChatJID   string
	MessageID string
	Timestamp int64
	Kind      string
	Size      int64
}

// MessagesMissingMedia returns attachments with no local file, newest first,
// so the most useful ones are collected before the oldest.
func (s *Store) MessagesMissingMedia(ctx context.Context, after MessageCursor, limit int) ([]PendingMedia, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if after.Timestamp <= 0 {
		after.Timestamp = 1 << 62
	}
	rows, err := s.db.QueryContext(ctx, `SELECT chat_jid,id,timestamp,kind,media_size FROM messages
 WHERE media_path='' AND kind IN ('image','video','audio','document','sticker')
  AND (timestamp,id) < (?,?)
 ORDER BY timestamp DESC,id DESC LIMIT ?`, after.Timestamp, after.MessageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PendingMedia, 0, limit)
	for rows.Next() {
		var item PendingMedia
		if err := rows.Scan(&item.ChatJID, &item.MessageID, &item.Timestamp, &item.Kind, &item.Size); err != nil {
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
