package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Project represents a local project.
type Project struct {
	ID                  int32     `json:"id"`
	Name                *string   `json:"name,omitempty"`
	Status              string    `json:"status"`
	ProjectPath         string    `json:"project_path"`
	BundleID            string    `json:"bundle_id"`
	Platform            string    `json:"platform,omitempty"`
	Platforms           []string  `json:"platforms,omitempty"`
	DeviceFamily        string    `json:"device_family,omitempty"`
	SessionID           string    `json:"session_id,omitempty"`
	Simulator           string    `json:"simulator,omitempty"`
	ConversationSummary string    `json:"conversation_summary,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// ProjectStore implements project storage using a local JSON file.
type ProjectStore struct {
	mu   sync.Mutex
	dir  string // .nanowave/ directory
	data *Project
}

// NewProjectStore creates a project store at the given directory.
func NewProjectStore(dir string) *ProjectStore {
	return &ProjectStore{dir: dir}
}

func (s *ProjectStore) filePath() string {
	return filepath.Join(s.dir, "project.json")
}

// Load reads the project from disk.
func (s *ProjectStore) Load() (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read project: %w", err)
	}

	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	s.data = &p
	return &p, nil
}

// Save writes the project to disk.
func (s *ProjectStore) Save(p *Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(s.filePath(), data, 0o644); err != nil {
		return fmt.Errorf("failed to write project: %w", err)
	}

	s.data = p
	return nil
}

// GetByID returns the project if the ID matches.
func (s *ProjectStore) GetByID(_ context.Context, id int32) (*Project, error) {
	p, err := s.Load()
	if err != nil {
		return nil, err
	}
	if p == nil || p.ID != id {
		return nil, fmt.Errorf("project %d not found", id)
	}
	return p, nil
}

// UpdateWorkspace updates the project's workspace info.
func (s *ProjectStore) UpdateWorkspace(_ context.Context, id int32, status, projectPath, bundleID string) (*Project, error) {
	p, err := s.Load()
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("project %d not found", id)
	}

	p.Status = status
	p.ProjectPath = projectPath
	p.BundleID = bundleID

	if err := s.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateName updates the project's name.
func (s *ProjectStore) UpdateName(_ context.Context, id int32, name string) (*Project, error) {
	p, err := s.Load()
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("project %d not found", id)
	}

	p.Name = &name
	if err := s.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateConversationSummary updates the conversation summary.
func (s *ProjectStore) UpdateConversationSummary(_ context.Context, id int32, summary string) error {
	p, err := s.Load()
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("project %d not found", id)
	}

	p.ConversationSummary = summary
	return s.Save(p)
}

// Create creates a new project.
func (s *ProjectStore) Create(name string) (*Project, error) {
	p := &Project{
		ID:        1, // Local projects always have ID 1
		Name:      &name,
		Status:    "creating",
		CreatedAt: time.Now(),
	}
	if err := s.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}
