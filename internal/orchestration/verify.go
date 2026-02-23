package orchestration

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// shouldRetryCompletion determines whether another completion pass is required.
func shouldRetryCompletion(report *FileCompletionReport, pass, maxPasses int) (bool, error) {
	if report == nil {
		return false, fmt.Errorf("file completion check failed: missing verification report")
	}

	if report.Complete {
		return false, nil
	}

	if pass >= maxPasses {
		return false, fmt.Errorf("file completion check failed after %d passes:\n%s", pass, formatIncompleteReport(report))
	}

	return true, nil
}

// verifyPlannedFiles checks whether all planned files exist and satisfy minimal validity requirements.
func verifyPlannedFiles(projectDir, appName string, plan *PlannerResult) (*FileCompletionReport, error) {
	if plan == nil {
		return nil, fmt.Errorf("cannot verify file completion without a build plan")
	}

	report := &FileCompletionReport{
		TotalPlanned: len(plan.Files),
	}
	if len(plan.Files) == 0 {
		report.Complete = true
		return report, nil
	}

	isMulti := plan.IsMultiPlatform()
	for _, planned := range plan.Files {
		status := PlannedFileStatus{
			PlannedPath:  planned.Path,
			ResolvedPath: resolvePlannedFilePathWithPlatform(projectDir, appName, planned, isMulti),
			ExpectedType: planned.TypeName,
		}

		info, err := os.Stat(status.ResolvedPath)
		if err != nil {
			if os.IsNotExist(err) {
				status.Reason = "file does not exist"
				report.Missing = append(report.Missing, status)
				continue
			}
			status.Exists = true
			status.Reason = fmt.Sprintf("unable to stat file: %v", err)
			report.Invalid = append(report.Invalid, status)
			continue
		}

		status.Exists = true
		if info.IsDir() {
			status.Reason = "path resolves to a directory, expected a file"
			report.Invalid = append(report.Invalid, status)
			continue
		}

		contentBytes, err := os.ReadFile(status.ResolvedPath)
		if err != nil {
			status.Reason = fmt.Sprintf("failed to read file: %v", err)
			report.Invalid = append(report.Invalid, status)
			continue
		}
		content := string(contentBytes)
		trimmed := strings.TrimSpace(content)

		if trimmed == "" {
			status.Reason = "file is empty"
			report.Invalid = append(report.Invalid, status)
			continue
		}

		if isPlaceholderOnlySwift(trimmed) {
			status.Reason = "file contains placeholder-only content"
			report.Invalid = append(report.Invalid, status)
			continue
		}

		if status.ExpectedType != "" && !strings.Contains(content, status.ExpectedType) {
			status.Reason = fmt.Sprintf("missing expected type %q", status.ExpectedType)
			report.Invalid = append(report.Invalid, status)
			continue
		}

		status.Valid = true
		report.ValidCount++
	}

	sort.Slice(report.Missing, func(i, j int) bool {
		return report.Missing[i].PlannedPath < report.Missing[j].PlannedPath
	})
	sort.Slice(report.Invalid, func(i, j int) bool {
		return report.Invalid[i].PlannedPath < report.Invalid[j].PlannedPath
	})

	report.Complete = report.ValidCount == report.TotalPlanned && len(report.Missing) == 0 && len(report.Invalid) == 0
	return report, nil
}

// resolvePlannedFilePath resolves a planner file path to an absolute file path.
func resolvePlannedFilePath(projectDir, appName, plannedPath string) string {
	cleanPath := filepath.Clean(filepath.FromSlash(plannedPath))

	// Try direct path first — the planner may already include the source dir prefix.
	direct := filepath.Join(projectDir, cleanPath)
	if _, err := os.Stat(direct); err == nil {
		return direct
	}

	return filepath.Join(projectDir, appName, cleanPath)
}

// resolvePlannedFilePathWithPlatform resolves a planner file path using AI-assigned platform.
// It tries the path directly under projectDir first (the planner may return full relative paths),
// then falls back to prepending the platform source directory.
func resolvePlannedFilePathWithPlatform(projectDir, appName string, planned FilePlan, isMultiPlatform bool) string {
	cleanPath := filepath.Clean(filepath.FromSlash(planned.Path))

	// Try the path directly under projectDir first — the planner may already
	// include the source directory in the path.
	direct := filepath.Join(projectDir, cleanPath)
	if _, err := os.Stat(direct); err == nil {
		return direct
	}

	// For multi-platform, use AI-assigned platform to determine base directory
	if isMultiPlatform && planned.Platform != "" {
		suffix := PlatformSourceDirSuffix(planned.Platform)
		return filepath.Join(projectDir, appName+suffix, cleanPath)
	}

	// Default: under {AppName}/
	return filepath.Join(projectDir, appName, cleanPath)
}

// unresolvedStatuses returns missing and invalid statuses in deterministic order.
func unresolvedStatuses(report *FileCompletionReport) []PlannedFileStatus {
	if report == nil {
		return nil
	}
	statuses := make([]PlannedFileStatus, 0, len(report.Missing)+len(report.Invalid))
	statuses = append(statuses, report.Missing...)
	statuses = append(statuses, report.Invalid...)
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].PlannedPath < statuses[j].PlannedPath
	})
	return statuses
}

// formatIncompleteReport builds a concise human-readable summary of unresolved files.
func formatIncompleteReport(report *FileCompletionReport) string {
	if report == nil {
		return "no completion report available"
	}

	var lines []string
	if len(report.Missing) > 0 {
		lines = append(lines, "Missing files:")
		for _, status := range report.Missing {
			lines = append(lines, fmt.Sprintf("- %s (%s)", status.PlannedPath, status.Reason))
		}
	}
	if len(report.Invalid) > 0 {
		lines = append(lines, "Invalid files:")
		for _, status := range report.Invalid {
			lines = append(lines, fmt.Sprintf("- %s (%s)", status.PlannedPath, status.Reason))
		}
	}
	if len(lines) == 0 {
		return "all files are complete"
	}
	return strings.Join(lines, "\n")
}

func isPlaceholderOnlySwift(content string) bool {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	trimmed := strings.TrimSpace(normalized)

	placeholderWithUnicodeDash := strings.TrimSpace("// Placeholder — replaced by generated code\nimport Foundation")
	placeholderWithAsciiDash := strings.TrimSpace("// Placeholder - replaced by generated code\nimport Foundation")
	if trimmed == placeholderWithUnicodeDash || trimmed == placeholderWithAsciiDash {
		return true
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "placeholder") {
		hasTypeDeclaration := strings.Contains(trimmed, "struct ") ||
			strings.Contains(trimmed, "class ") ||
			strings.Contains(trimmed, "enum ") ||
			strings.Contains(trimmed, "protocol ") ||
			strings.Contains(trimmed, "extension ") ||
			strings.Contains(trimmed, "actor ") ||
			strings.Contains(trimmed, "@main")
		if !hasTypeDeclaration {
			return true
		}
	}

	return false
}
