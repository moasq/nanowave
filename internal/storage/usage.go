package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionUsage tracks cumulative token usage for the current session.
type SessionUsage struct {
	StartedAt    time.Time `json:"started_at"`
	TotalCostUSD float64   `json:"total_cost_usd"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CacheRead    int       `json:"cache_read_input_tokens"`
	CacheCreated int       `json:"cache_creation_input_tokens"`
	Requests     int       `json:"requests"`
}

// DailyUsage tracks aggregate usage for a single calendar day.
type DailyUsage struct {
	Date         string  `json:"date"` // YYYY-MM-DD
	TotalCostUSD float64 `json:"total_cost_usd"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Requests     int     `json:"requests"`
}

// UsageStore accumulates session usage and persists it to disk.
type UsageStore struct {
	mu      sync.Mutex
	dir     string // .nanowave/ directory
	current *SessionUsage
}

const maxHistoryDays = 30

// NewUsageStore creates a usage store at the given directory.
func NewUsageStore(dir string) *UsageStore {
	s := &UsageStore{dir: dir}
	// Load existing session if present
	if data, err := os.ReadFile(s.filePath()); err == nil {
		var usage SessionUsage
		if json.Unmarshal(data, &usage) == nil {
			s.current = &usage
		}
	}
	if s.current == nil {
		s.current = &SessionUsage{StartedAt: time.Now()}
	}
	return s
}

func (s *UsageStore) filePath() string {
	return filepath.Join(s.dir, "usage.json")
}

func (s *UsageStore) historyPath() string {
	return filepath.Join(s.dir, "usage_history.json")
}

// RecordUsage accumulates usage from a response's cost and token data.
func (s *UsageStore) RecordUsage(costUSD float64, inputTokens, outputTokens, cacheRead, cacheCreated int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current.TotalCostUSD += costUSD
	s.current.InputTokens += inputTokens
	s.current.OutputTokens += outputTokens
	s.current.CacheRead += cacheRead
	s.current.CacheCreated += cacheCreated
	s.current.Requests++

	s.persistUnsafe()
	s.updateDailyUnsafe(costUSD, inputTokens, outputTokens)
}

// Current returns the current session usage stats.
func (s *UsageStore) Current() *SessionUsage {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *s.current
	return &cp
}

// Reset clears usage counters for a new session.
func (s *UsageStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current = &SessionUsage{StartedAt: time.Now()}
	s.persistUnsafe()
}

// History returns the last N days of usage history, most recent first.
func (s *UsageStore) History(days int) []DailyUsage {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := s.loadHistoryUnsafe()
	if len(history) == 0 {
		return nil
	}

	if days <= 0 || days > len(history) {
		days = len(history)
	}

	// History is stored chronologically; return most recent N entries in reverse.
	start := len(history) - days
	if start < 0 {
		start = 0
	}
	result := make([]DailyUsage, 0, days)
	for i := len(history) - 1; i >= start; i-- {
		result = append(result, history[i])
	}
	return result
}

// TodayUsage returns usage for the current day, or nil if no usage recorded today.
func (s *UsageStore) TodayUsage() *DailyUsage {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	history := s.loadHistoryUnsafe()
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Date == today {
			cp := history[i]
			return &cp
		}
	}
	return nil
}

func (s *UsageStore) persistUnsafe() {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(s.current, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.filePath(), data, 0o644)
}

func (s *UsageStore) updateDailyUnsafe(costUSD float64, inputTokens, outputTokens int) {
	today := time.Now().Format("2006-01-02")
	history := s.loadHistoryUnsafe()

	// Find or create today's entry
	found := false
	for i := range history {
		if history[i].Date == today {
			history[i].TotalCostUSD += costUSD
			history[i].InputTokens += inputTokens
			history[i].OutputTokens += outputTokens
			history[i].Requests++
			found = true
			break
		}
	}
	if !found {
		history = append(history, DailyUsage{
			Date:         today,
			TotalCostUSD: costUSD,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			Requests:     1,
		})
	}

	// Trim to rolling window
	if len(history) > maxHistoryDays {
		history = history[len(history)-maxHistoryDays:]
	}

	s.saveHistoryUnsafe(history)
}

func (s *UsageStore) loadHistoryUnsafe() []DailyUsage {
	data, err := os.ReadFile(s.historyPath())
	if err != nil {
		return nil
	}
	var history []DailyUsage
	if json.Unmarshal(data, &history) != nil {
		return nil
	}
	return history
}

func (s *UsageStore) saveHistoryUnsafe(history []DailyUsage) {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.historyPath(), data, 0o644)
}

// FormatTokenCount formats a token count for display (e.g., 48543 → "48.5K").
func FormatTokenCount(tokens int) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	}
	if tokens >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	}
	return fmt.Sprintf("%d", tokens)
}
