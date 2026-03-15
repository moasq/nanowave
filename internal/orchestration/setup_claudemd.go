package orchestration

import (
	"fmt"
	"path/filepath"
)

func writeProjectMakefileWithShape(projectDir, appName, platform, watchProjectShape string) error {
	buildCmd := canonicalBuildCommandForShape(appName, platform, watchProjectShape)
	// Tests must run on simulator (device targets can't execute tests without provisioning)
	testDestination := canonicalSimulatorBuildDestination(platform, watchProjectShape)
	content := fmt.Sprintf(".PHONY: build test\n\nbuild:\n\t%s\n\ntest:\n\t@if [ -d Tests ]; then \\\n\t\techo \"Tests directory found; running xcodebuild test\"; \\\n\t\txcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet test; \\\n\telse \\\n\t\techo \"No Tests directory present; skipping tests\"; \\\n\tfi\n", buildCmd, appName, appName, testDestination)
	return writeTextFile(filepath.Join(projectDir, "Makefile"), content, 0o644)
}
