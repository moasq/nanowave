package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
)

func loadConfig() (*config.Config, error) {
	return config.Load()
}

// loadConfigWithProject loads config and selects the most recent project.
// Returns an error if no projects exist in the catalog.
func loadConfigWithProject() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	projects := cfg.ListProjects()
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects found. Run `nanowave` first to create a project")
	}

	// Use the most recent project
	cfg.SetProject(projects[0].Path)
	return cfg, nil
}

func openProject(cfg *config.Config) error {
	store := storage.NewProjectStore(cfg.NanowaveDir)
	project, err := store.Load()
	if err != nil || project == nil {
		return fmt.Errorf("no project found")
	}

	// Find .xcodeproj
	entries, err := os.ReadDir(project.ProjectPath)
	if err != nil {
		return fmt.Errorf("failed to read project directory: %w", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xcodeproj") {
			xcodeprojPath := filepath.Join(project.ProjectPath, entry.Name())
			terminal.Info(fmt.Sprintf("Opening %s...", entry.Name()))
			return exec.Command("open", xcodeprojPath).Run()
		}
	}

	// Fallback: open the directory
	terminal.Info(fmt.Sprintf("Opening %s...", project.ProjectPath))
	return exec.Command("open", project.ProjectPath).Run()
}
