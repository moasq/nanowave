package commands

import (
	"fmt"

	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/service"
	"github.com/moasq/nanowave/internal/terminal"
)

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

func loadProjectService(opts ...service.ServiceOpts) (*service.Service, error) {
	cfg, err := loadConfigWithProject()
	if err != nil {
		return nil, err
	}
	return service.NewService(cfg, opts...)
}

func printNoProjectFoundCreateFirst() {
	terminal.Error("No project found.")
	terminal.Info("Run `nanowave` first to create a project.")
}
