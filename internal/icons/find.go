package icons

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// FindAppIconDir locates the AppIcon.appiconset directory in a project.
func FindAppIconDir(projectDir string) (string, error) {
	var found string
	err := filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == "AppIcon.appiconset" {
			found = path
			return filepath.SkipAll
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "DerivedData" || d.Name() == "build") {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to search for AppIcon.appiconset: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("AppIcon.appiconset not found in project")
	}
	return found, nil
}

// FindSourceIcon returns the path to the largest PNG/JPG in the appiconset directory,
// which is used as the source for generating all required icon sizes.
func FindSourceIcon(appIconDir string) string {
	entries, err := os.ReadDir(appIconDir)
	if err != nil {
		log.Printf("[asc][icon] FindSourceIcon: cannot read dir %s: %v", appIconDir, err)
		return ""
	}
	var best string
	var bestSize int64
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") {
			info, _ := e.Info()
			if info != nil {
				log.Printf("[asc][icon] FindSourceIcon: candidate %s (%d bytes)", e.Name(), info.Size())
				if info.Size() > bestSize {
					bestSize = info.Size()
					best = filepath.Join(appIconDir, e.Name())
				}
			}
		}
	}
	if best != "" {
		log.Printf("[asc][icon] FindSourceIcon: selected %s (%d bytes) as source", filepath.Base(best), bestSize)
	} else {
		log.Printf("[asc][icon] FindSourceIcon: no image files found in %s", appIconDir)
	}
	return best
}

// HasExisting checks if AppIcon.appiconset has an actual image file.
func HasExisting(appIconDir string) bool {
	entries, err := os.ReadDir(appIconDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") {
			info, _ := e.Info()
			if info != nil && info.Size() > 0 {
				return true
			}
		}
	}
	return false
}
