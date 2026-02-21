package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryMessage represents a message in conversation history.
type HistoryMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// HistoryStore implements message history storage using a local JSON file.
type HistoryStore struct {
	mu  sync.Mutex
	dir string
}

// NewHistoryStore creates a history store at the given directory.
func NewHistoryStore(dir string) *HistoryStore {
	return &HistoryStore{dir: dir}
}

func (s *HistoryStore) filePath() string {
	return filepath.Join(s.dir, "history.json")
}

// Append adds a message to the history.
func (s *HistoryStore) Append(msg HistoryMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages, err := s.readUnsafe()
	if err != nil {
		messages = nil // Start fresh if file is corrupted
	}

	msg.CreatedAt = time.Now()
	messages = append(messages, msg)

	return s.writeUnsafe(messages)
}

// List returns all messages in the history.
func (s *HistoryStore) List() ([]HistoryMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readUnsafe()
}

// Recent returns the last N messages.
func (s *HistoryStore) Recent(n int) ([]HistoryMessage, error) {
	messages, err := s.List()
	if err != nil {
		return nil, err
	}

	if len(messages) <= n {
		return messages, nil
	}
	return messages[len(messages)-n:], nil
}

// Clear removes all messages.
func (s *HistoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeUnsafe(nil)
}

func (s *HistoryStore) readUnsafe() ([]HistoryMessage, error) {
	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read history: %w", err)
	}

	var messages []HistoryMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}
	return messages, nil
}

func (s *HistoryStore) writeUnsafe(messages []HistoryMessage) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(s.filePath(), data, 0o644)
}
