// Package mediastore keeps message attachments in a database so they survive
// clearing the media cache.
//
// Attachments live in their own database file rather than beside the message
// index. Photos and videos are large, and mixing multi-megabyte blobs into
// messages.db would inflate its write-ahead log and make the backup advice in
// the user guide impractical. The cache directory stays the form the desktop
// application reads: files are materialised from here on demand.
package mediastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// chunkSize bounds how much of an attachment is held in memory at once. Rows
// of this size keep a 2 GiB video from being read or written as one value.
const chunkSize = 2 << 20

type Store struct {
	db *sql.DB
}

// Info describes a stored attachment.
type Info struct {
	MIME string
	Name string
	Size int64
}

func Open(path string) (*Store, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	db, err := sql.Open("sqlite", "file:"+path+separator+"_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
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
	return Open("file:whatsappgo-media-test?mode=memory&cache=shared")
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS media (
  chat_jid TEXT NOT NULL,
  message_id TEXT NOT NULL,
  mime TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (chat_jid, message_id)
);
CREATE TABLE IF NOT EXISTS media_chunks (
  chat_jid TEXT NOT NULL,
  message_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  data BLOB NOT NULL,
  PRIMARY KEY (chat_jid, message_id, seq),
  FOREIGN KEY (chat_jid, message_id) REFERENCES media(chat_jid, message_id) ON DELETE CASCADE
);`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Put stores an attachment, replacing any previous copy for the same message.
func (s *Store) Put(ctx context.Context, chatJID, messageID string, info Info, source io.Reader) error {
	if chatJID == "" || messageID == "" {
		return errors.New("chat_jid and message_id are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM media WHERE chat_jid=? AND message_id=?`, chatJID, messageID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO media(chat_jid,message_id,mime,name,size,created_at) VALUES(?,?,?,?,0,?)`,
		chatJID, messageID, info.MIME, info.Name, time.Now().UnixMilli()); err != nil {
		return err
	}
	insert, err := tx.PrepareContext(ctx, `INSERT INTO media_chunks(chat_jid,message_id,seq,data) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	buffer := make([]byte, chunkSize)
	var total int64
	for seq := 0; ; seq++ {
		read, readErr := io.ReadFull(source, buffer)
		if read > 0 {
			if _, err := insert.ExecContext(ctx, chatJID, messageID, seq, buffer[:read]); err != nil {
				return err
			}
			total += int64(read)
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE media SET size=? WHERE chat_jid=? AND message_id=?`, total, chatJID, messageID); err != nil {
		return err
	}
	return tx.Commit()
}

// PutFile stores a file that is already on disk.
func (s *Store) PutFile(ctx context.Context, chatJID, messageID string, info Info, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if info.Size == 0 {
		if stat, err := file.Stat(); err == nil {
			info.Size = stat.Size()
		}
	}
	if info.Name == "" {
		info.Name = filepath.Base(path)
	}
	return s.Put(ctx, chatJID, messageID, info, file)
}

// Lookup reports what is stored for a message.
func (s *Store) Lookup(ctx context.Context, chatJID, messageID string) (Info, bool, error) {
	var info Info
	err := s.db.QueryRowContext(ctx, `SELECT mime,name,size FROM media WHERE chat_jid=? AND message_id=?`,
		chatJID, messageID).Scan(&info.MIME, &info.Name, &info.Size)
	if errors.Is(err, sql.ErrNoRows) {
		return Info{}, false, nil
	}
	if err != nil {
		return Info{}, false, err
	}
	return info, true, nil
}

// WriteTo streams a stored attachment, one chunk at a time.
func (s *Store) WriteTo(ctx context.Context, chatJID, messageID string, sink io.Writer) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM media_chunks WHERE chat_jid=? AND message_id=? ORDER BY seq`,
		chatJID, messageID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var written int64
	for rows.Next() {
		var chunk []byte
		if err := rows.Scan(&chunk); err != nil {
			return written, err
		}
		n, err := sink.Write(chunk)
		written += int64(n)
		if err != nil {
			return written, err
		}
	}
	return written, rows.Err()
}

// Materialise writes a stored attachment to a path so it can be opened or
// played, and reports whether the database held it.
func (s *Store) Materialise(ctx context.Context, chatJID, messageID, path string) (bool, error) {
	if _, found, err := s.Lookup(ctx, chatJID, messageID); err != nil || !found {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "restore-*")
	if err != nil {
		return false, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := s.WriteTo(ctx, chatJID, messageID, temporary); err != nil {
		temporary.Close()
		return false, err
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return false, err
	}
	if err := temporary.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return false, fmt.Errorf("restore attachment: %w", err)
	}
	return true, nil
}

// Delete permanently removes a stored attachment.
func (s *Store) Delete(ctx context.Context, chatJID, messageID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM media WHERE chat_jid=? AND message_id=?`, chatJID, messageID)
	return err
}

// TotalSize reports how much attachment data is stored.
func (s *Store) TotalSize(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT SUM(size) FROM media`).Scan(&total); err != nil {
		return 0, err
	}
	return total.Int64, nil
}
