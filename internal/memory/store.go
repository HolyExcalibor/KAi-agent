// Package memory provides persistent conversation storage backed by SQLite.
package memory

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Message represents a single turn in a conversation.
type Message struct {
	ID           int64
	SessionID    string
	Role         string // "user" | "assistant" | "system" | "tool"
	Content      string
	ToolCallID   string
	ToolName     string
	CreatedAt    time.Time
}

// Store manages conversation history in SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath and runs migrations.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("memory: open db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("memory: migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id  TEXT    NOT NULL,
			role        TEXT    NOT NULL,
			content     TEXT    NOT NULL DEFAULT '',
			tool_call_id TEXT   NOT NULL DEFAULT '',
			tool_name   TEXT    NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id);
	`)
	return err
}

// Append saves a message to the store.
func (s *Store) Append(msg Message) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO messages (session_id, role, content, tool_call_id, tool_name)
		 VALUES (?, ?, ?, ?, ?)`,
		msg.SessionID, msg.Role, msg.Content, msg.ToolCallID, msg.ToolName,
	)
	if err != nil {
		return 0, fmt.Errorf("memory: append: %w", err)
	}
	return res.LastInsertId()
}

// GetHistory returns all messages for a session ordered by id.
func (s *Store) GetHistory(sessionID string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, tool_call_id, tool_name, created_at
		 FROM messages WHERE session_id = ? ORDER BY id ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: get history: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content,
			&m.ToolCallID, &m.ToolName, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ListSessions returns all distinct session IDs.
func (s *Store) ListSessions() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT session_id FROM messages ORDER BY MIN(id) DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteSession removes all messages for a session.
func (s *Store) DeleteSession(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID)
	return err
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
