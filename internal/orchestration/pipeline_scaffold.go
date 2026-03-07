package orchestration

import (
	"fmt"

	"github.com/moasq/nanowave/internal/terminal"
)

// scaffoldProject writes project_config.json, project.yml, asset catalogs,
// source directory stubs, .gitignore, and runs XcodeGen.
func (p *Pipeline) scaffoldProject(projectDir, appName string, plan *PlannerResult, needsAppleSignIn bool) error {
	// Write project_config.json first (source of truth for XcodeGen MCP server).
	if err := writeProjectConfig(projectDir, plan, appName); err != nil {
		return fmt.Errorf("failed to write project_config.json: %w", err)
	}

	// Auto-add Apple Sign-In entitlement when apple auth is detected.
	if needsAppleSignIn {
		if err := addAutoEntitlement(projectDir, "com.apple.developer.applesignin", []any{"Default"}, ""); err != nil {
			terminal.Warning(fmt.Sprintf("Could not add Apple Sign-In entitlement: %v", err))
		} else {
			terminal.Success("Apple Sign-In entitlement added to project config")
		}
	}

	// Read back entitlements from project_config.json so project.yml includes them.
	mainEntitlements := readConfigEntitlements(projectDir, "")

	if err := writeProjectYML(projectDir, plan, appName, mainEntitlements); err != nil {
		return fmt.Errorf("failed to write project.yml: %w", err)
	}

	if err := writeGitignore(projectDir); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	if err := p.writeAssetCatalogs(projectDir, appName, plan); err != nil {
		return err
	}

	if err := scaffoldSourceDirs(projectDir, appName, plan); err != nil {
		return fmt.Errorf("failed to scaffold source dirs: %w", err)
	}

	if err := runXcodeGen(projectDir); err != nil {
		return fmt.Errorf("failed to run xcodegen: %w", err)
	}

	return nil
}

// writeAssetCatalogs writes platform-appropriate asset catalogs.
func (p *Pipeline) writeAssetCatalogs(projectDir, appName string, plan *PlannerResult) error {
	if plan.IsMultiPlatform() {
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			dirName := appName + suffix
			if err := writeAssetCatalog(projectDir, dirName, plat); err != nil {
				return fmt.Errorf("failed to write %s asset catalog: %w", PlatformDisplayName(plat), err)
			}
		}
	} else if IsWatchOS(plan.GetPlatform()) && plan.GetWatchProjectShape() == WatchShapePaired {
		if err := writeAssetCatalog(projectDir, appName, PlatformIOS); err != nil {
			return fmt.Errorf("failed to write asset catalog: %w", err)
		}
		if err := writeAssetCatalog(projectDir, appName+"Watch", PlatformWatchOS); err != nil {
			return fmt.Errorf("failed to write watch asset catalog: %w", err)
		}
	} else {
		if err := writeAssetCatalog(projectDir, appName, plan.GetPlatform()); err != nil {
			return fmt.Errorf("failed to write asset catalog: %w", err)
		}
	}
	return nil
}
