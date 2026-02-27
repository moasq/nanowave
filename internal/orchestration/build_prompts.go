package orchestration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/terminal"
)

// appearanceModeDescription returns a human-readable description of the app's
// appearance mode for the build prompt, so the LLM knows the appearance context
// and uses UIKit adaptive colors (Color(.label), etc.) for text.
func appearanceModeDescription(plan *PlannerResult) string {
	if plan != nil && plan.HasRuleKey("dark-mode") {
		return "adaptive (supports light/dark/system via user preference — use Color(.label) adaptive text colors)"
	}
	if plan != nil && isDarkPalette(plan.Design.Palette) {
		return "locked to Dark (dark palette — use Color(.label) for text, system chrome is dark)"
	}
	return "locked to Light (use Color(.label) for text, system chrome is light)"
}

func appendBuildPlanFileEntry(b *strings.Builder, f FilePlan) {
	platformTag := ""
	if f.Platform != "" {
		platformTag = fmt.Sprintf(" [%s]", f.Platform)
	}
	fmt.Fprintf(b, "- %s (%s)%s: %s\n  Components: %s\n  Data access: %s\n",
		f.Path, f.TypeName, platformTag, f.Purpose, f.Components, f.DataAccess)
}

// buildPrompts constructs the system and user prompts for the build phase.
func (p *Pipeline) buildPrompts(_ string, appName string, _ string, analysis *AnalysisResult, plan *PlannerResult, backendProvisioned bool) (string, string, error) {
	destination := canonicalBuildDestinationForShape(plan.GetPlatform(), plan.GetWatchProjectShape())
	// Build the append system prompt with coder rules + plan context
	basePrompt, err := composeCoderAppendPrompt("builder", plan.GetPlatform())
	if err != nil {
		return "", "", err
	}
	var appendPrompt strings.Builder
	appendPrompt.WriteString(basePrompt)

	// Add plan context
	appendPrompt.WriteString("\n\n<build-plan>\n")

	appendPrompt.WriteString("## Design\n")
	appendPrompt.WriteString(fmt.Sprintf("Navigation: %s\n", plan.Design.Navigation))
	appendPrompt.WriteString(fmt.Sprintf("Palette: primary=%s, secondary=%s, accent=%s, background=%s, surface=%s\n",
		plan.Design.Palette.Primary, plan.Design.Palette.Secondary, plan.Design.Palette.Accent,
		plan.Design.Palette.Background, plan.Design.Palette.Surface))
	appendPrompt.WriteString(fmt.Sprintf("Appearance: %s\n", appearanceModeDescription(plan)))
	appendPrompt.WriteString(fmt.Sprintf("Font: %s, Corner radius: %d, Density: %s, Surfaces: %s, Mood: %s\n",
		plan.Design.FontDesign, plan.Design.CornerRadius, plan.Design.Density, plan.Design.Surfaces, plan.Design.AppMood))

	appendPrompt.WriteString("\n### Models\n")
	for _, m := range plan.Models {
		appendPrompt.WriteString(fmt.Sprintf("- %s (%s):\n", m.Name, m.Storage))
		for _, prop := range m.Properties {
			if prop.DefaultValue != "" {
				appendPrompt.WriteString(fmt.Sprintf("  - %s: %s = %s\n", prop.Name, prop.Type, prop.DefaultValue))
			} else {
				appendPrompt.WriteString(fmt.Sprintf("  - %s: %s\n", prop.Name, prop.Type))
			}
		}
	}

	appendPrompt.WriteString("\n## Files (build in this order)\n")
	filesByPath := make(map[string]FilePlan, len(plan.Files))
	for _, f := range plan.Files {
		filesByPath[f.Path] = f
	}

	inBuildOrder := make(map[string]bool, len(plan.BuildOrder))
	for _, path := range plan.BuildOrder {
		inBuildOrder[path] = true
		if f, ok := filesByPath[path]; ok {
			appendBuildPlanFileEntry(&appendPrompt, f)
		}
	}

	// Include any files not in build_order, preserving original plan order.
	for _, f := range plan.Files {
		if inBuildOrder[f.Path] {
			continue
		}
		appendBuildPlanFileEntry(&appendPrompt, f)
	}

	if len(plan.Permissions) > 0 {
		appendPrompt.WriteString("\n### Permissions\n")
		for _, perm := range plan.Permissions {
			appendPrompt.WriteString(fmt.Sprintf("- %s: \"%s\" (framework: %s)\n", perm.Key, perm.Description, perm.Framework))
		}
	}

	if len(plan.Extensions) > 0 {
		appendPrompt.WriteString("\n### Extensions\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			appendPrompt.WriteString(fmt.Sprintf("- %s (kind: %s): %s\n  Source path: Targets/%s/\n", name, ext.Kind, ext.Purpose, name))
		}
	}

	if len(plan.Localizations) > 0 {
		appendPrompt.WriteString(fmt.Sprintf("\n### Localizations: %s\n", strings.Join(plan.Localizations, ", ")))
	}

	// SPM package instructions
	appendPrompt.WriteString("\n### SPM Packages\n")
	appendBuildSPMSection(&appendPrompt, plan.Packages, appName)

	appendPrompt.WriteString("\n</build-plan>\n")

	// Inject rule content for each rule_key from embedded skill files
	if len(plan.RuleKeys) > 0 {
		appendPrompt.WriteString("\n<feature-rules>\n")
		for _, key := range plan.RuleKeys {
			content := loadRuleContent(key)
			if content != "" {
				appendPrompt.WriteString("\n")
				appendPrompt.WriteString(content)
				appendPrompt.WriteString("\n")
			}
		}
		appendPrompt.WriteString("</feature-rules>\n")
	}

	// Inject integration config if any integrations are active
	if len(plan.Integrations) > 0 {
		var authMethods []string
		if analysis.BackendNeeds != nil {
			authMethods = analysis.BackendNeeds.AuthMethods
			// Default auth methods when auth is needed but analyzer didn't specify methods
			if analysis.BackendNeeds.Auth && len(authMethods) == 0 {
				authMethods = []string{"email", "anonymous"}
				terminal.Detail("Auth methods", "analyzer returned empty — defaulting to [email, anonymous]")
			}
		}
		appendIntegrationConfig(&appendPrompt, plan.Integrations, plan.Models, appName, authMethods)
	}

	// Build user message
	var featureList strings.Builder
	for _, f := range analysis.Features {
		featureList.WriteString(fmt.Sprintf("- %s: %s\n", f.Name, f.Description))
	}

	// Backend instructions injected into the user message when Supabase is active
	hasSupabase := false
	for _, id := range plan.Integrations {
		if id == "supabase" {
			hasSupabase = true
			break
		}
	}
	backendFirstBlock := ""
	if hasSupabase && backendProvisioned {
		terminal.Info("Supabase backend already provisioned — injecting VERIFY block into user message")
		backendFirstBlock = `
SUPABASE BACKEND (already provisioned by nanowave):
Tables, RLS policies, and storage buckets have been created automatically.
1. Use mcp__supabase__list_tables to see the available tables.
2. If you need additional tables, indexes, or policies, use mcp__supabase__execute_sql.
3. Proceed directly to writing Swift code — the backend is ready.

`
	} else if hasSupabase {
		terminal.Info("Supabase detected but NOT provisioned — injecting BACKEND FIRST block into user message")
		backendFirstBlock = `
CRITICAL — BACKEND FIRST (before writing ANY Swift code):
1. Read the <backend-setup> section in the system prompt — it has the exact SQL.
2. Use mcp__supabase__execute_sql to create ALL tables defined there (run every CREATE TABLE statement).
3. Use mcp__supabase__execute_sql to enable RLS on every table and create RLS policies.
4. If the app has file uploads, create storage buckets and policies.
5. Use mcp__supabase__list_tables to VERIFY all tables exist.
6. Only after tables are confirmed — proceed to write Swift code.
DO NOT skip this. The app CANNOT function without a backend schema.

`
	} else {
		terminal.Detail("Build prompt", "No Supabase integration — skipping backend block")
	}

	var userMsg string
	if plan.IsMultiPlatform() {
		buildCmds := multiPlatformBuildCommands(appName, plan.GetPlatforms())
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}

		var sourceDirsList strings.Builder
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			dirName := appName + suffix
			fmt.Fprintf(&sourceDirsList, "- %s/ — %s source (files with platform:%q)\n", dirName, PlatformDisplayName(plat), plat)
		}
		sourceDirsList.WriteString("- Shared/ — cross-platform code (files with platform:\"\")\n")

		userMsg = fmt.Sprintf(`Build the %s app following the plan in the system prompt.

App description: %s

Features:
%s
Core flow: %s

BEFORE WRITING CODE:
1. Use Glob to list all files in the project directory to understand the existing structure
2. Read the CLAUDE.md file to understand design tokens and architecture
3. Read project_config.json to understand the project configuration
%sThen proceed with writing code.

MULTI-PLATFORM SOURCE DIRECTORIES:
%s
INSTRUCTIONS:
1. The Xcode project is already configured with targets for all platforms.
2. Write ALL Swift files under the correct platform source directory based on the file's platform tag.
3. Extension files go under Targets/{ExtensionName}/.
4. Shared cross-platform types go under Shared/.
5. If you need additional permissions, extensions, or entitlements beyond the plan, use the xcodegen MCP tools.
6. After writing ALL files for ALL platforms, build each scheme in sequence:
%s7. If any build fails, read the errors, fix the Swift code, and rebuild.
8. Repeat until all builds succeed.
9. If Xcode says a scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed schemes.

IMPORTANT:
- Write files in the build order specified in the plan
- Use the exact type names and file paths from the plan
- Every View must have a #Preview block
- Each platform has its own @main App entry point`,
			analysis.AppName, analysis.Description, featureList.String(), analysis.CoreFlow,
			backendFirstBlock, sourceDirsList.String(), buildCmdStr.String(), appName)
	} else {
		userMsg = fmt.Sprintf(`Build the %s app following the plan in the system prompt.

App description: %s

Features:
%s
Core flow: %s

BEFORE WRITING CODE:
1. Use Glob to list all files in the project directory to understand the existing structure
2. Read the CLAUDE.md file to understand design tokens and architecture
3. Read project_config.json to understand the project configuration
%sThen proceed with writing code.

INSTRUCTIONS:
1. The Xcode project is already configured. Write ALL Swift files under %s/ following the plan file paths exactly.
2. Extension files go under Targets/{ExtensionName}/.
3. Shared types (e.g. ActivityAttributes) go under Shared/.
4. If you need additional permissions, extensions, or entitlements beyond the plan, use the xcodegen MCP tools.
5. After writing ALL files, run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
6. If the build fails, read the errors, fix the Swift code, and rebuild.
7. Repeat until the build succeeds.
8. If Xcode says the scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed app scheme.

IMPORTANT:
- Write files in the build order specified in the plan
- Use the exact type names and file paths from the plan
- Every View must have a #Preview block`,
			analysis.AppName, analysis.Description, featureList.String(), analysis.CoreFlow,
			backendFirstBlock, appName, appName, appName, destination, appName)
	}

	return appendPrompt.String(), userMsg, nil
}

// completionPrompts builds targeted prompts for unresolved planned files.
func (p *Pipeline) completionPrompts(appName string, projectDir string, plan *PlannerResult, report *FileCompletionReport) (string, string, error) {
	destination := canonicalBuildDestinationForShape(plan.GetPlatform(), plan.GetWatchProjectShape())
	basePrompt, err := composeCoderAppendPrompt("completion-recovery", plan.GetPlatform())
	if err != nil {
		return "", "", err
	}
	var appendPrompt strings.Builder
	appendPrompt.WriteString(basePrompt)
	appendPrompt.WriteString("\n\n## Completion Recovery Mode\n")
	appendPrompt.WriteString("Only complete the unresolved planned files listed in the user message.\n")
	appendPrompt.WriteString("Do not mark work done until every listed file exists, contains its expected type, and the build succeeds.\n")

	plannedByPath := make(map[string]FilePlan, len(plan.Files))
	for _, f := range plan.Files {
		plannedByPath[f.Path] = f
	}

	unresolved := unresolvedStatuses(report)

	var fileList strings.Builder
	for _, status := range unresolved {
		filePlan := plannedByPath[status.PlannedPath]
		relPath := status.ResolvedPath
		if rel, err := filepath.Rel(projectDir, status.ResolvedPath); err == nil {
			relPath = filepath.ToSlash(rel)
		}
		fileList.WriteString(fmt.Sprintf("- Planned path: %s\n", status.PlannedPath))
		fileList.WriteString(fmt.Sprintf("  Disk path: %s\n", relPath))
		if filePlan.Platform != "" {
			fileList.WriteString(fmt.Sprintf("  Platform: %s\n", filePlan.Platform))
		}
		if filePlan.TypeName != "" {
			fileList.WriteString(fmt.Sprintf("  Expected type: %s\n", filePlan.TypeName))
		}
		if filePlan.Purpose != "" {
			fileList.WriteString(fmt.Sprintf("  Purpose: %s\n", filePlan.Purpose))
		}
		if status.Reason != "" {
			fileList.WriteString(fmt.Sprintf("  Current issue: %s\n", status.Reason))
		}
	}

	var userMsg string
	if plan.IsMultiPlatform() {
		buildCmds := multiPlatformBuildCommands(appName, plan.GetPlatforms())
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}
		userMsg = fmt.Sprintf(`Complete the unresolved files from the original build plan.

Unresolved files:
%s
Required process:
1. Create/fix ONLY the unresolved files listed above.
2. Ensure each file contains the expected type name exactly.
3. Place files in the correct platform source directory based on the Platform field.
4. Keep existing already-valid files unchanged unless required for imports/signatures.
5. Build each scheme in sequence:
%s6. If any build fails, fix issues and rebuild.
7. If a scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed schemes.
8. Stop only when every unresolved file is complete and all builds succeed.`, fileList.String(), buildCmdStr.String(), appName)
	} else {
		userMsg = fmt.Sprintf(`Complete the unresolved files from the original build plan.

Unresolved files:
%s
Required process:
1. Create/fix ONLY the unresolved files listed above.
2. Ensure each file contains the expected type name exactly.
3. Keep existing already-valid files unchanged unless required for imports/signatures.
4. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
5. If build fails, fix issues and rebuild.
6. If the scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed app scheme.
7. Stop only when every unresolved file is complete and the build succeeds.`, fileList.String(), appName, appName, destination, appName)
	}

	return appendPrompt.String(), userMsg, nil
}

// appendBuildSPMSection writes the SPM package instructions into the build prompt.
// It resolves planned packages against the curated registry for exact details,
// and falls back to internet-search instructions for unrecognized packages.
func appendBuildSPMSection(b *strings.Builder, packages []PackagePlan, appName string) {
	var resolved []*CuratedPackage
	var unresolved []PackagePlan

	for _, pkg := range packages {
		if curated := LookupPackageByName(pkg.Name); curated != nil {
			resolved = append(resolved, curated)
		} else {
			unresolved = append(unresolved, pkg)
		}
	}

	// Emit resolved packages with full integration details
	if len(resolved) > 0 {
		b.WriteString("These packages are approved for this project. Use the exact details below to integrate each one.\n\n")
		for _, pkg := range resolved {
			b.WriteString(fmt.Sprintf("**%s** — %s\n", pkg.Name, pkg.Description))
			b.WriteString(fmt.Sprintf("- Repository: %s\n", pkg.RepoURL))
			b.WriteString(fmt.Sprintf("- XcodeGen package key: `%s`\n", pkg.RepoName))
			b.WriteString(fmt.Sprintf("- Minimum version: `\"%s\"`\n", pkg.MinVersion))
			if len(pkg.Products) == 1 {
				b.WriteString(fmt.Sprintf("- Import: `import %s`\n", pkg.Products[0]))
			} else {
				b.WriteString("- Products (import each one you use):\n")
				for _, p := range pkg.Products {
					b.WriteString(fmt.Sprintf("  - `import %s`\n", p))
				}
			}
			// Write the project.yml snippet
			b.WriteString("- project.yml:\n")
			b.WriteString("```yaml\n")
			b.WriteString("packages:\n")
			b.WriteString(fmt.Sprintf("  %s:\n", pkg.RepoName))
			b.WriteString(fmt.Sprintf("    url: %s\n", pkg.RepoURL))
			b.WriteString(fmt.Sprintf("    from: \"%s\"\n", pkg.MinVersion))
			b.WriteString("targets:\n")
			b.WriteString(fmt.Sprintf("  %s:\n", appName))
			b.WriteString("    dependencies:\n")
			if len(pkg.Products) == 1 && pkg.Products[0] == pkg.RepoName {
				b.WriteString(fmt.Sprintf("      - package: %s\n", pkg.RepoName))
			} else if len(pkg.Products) == 1 {
				b.WriteString(fmt.Sprintf("      - package: %s\n", pkg.RepoName))
				b.WriteString(fmt.Sprintf("        product: %s\n", pkg.Products[0]))
			} else {
				b.WriteString(fmt.Sprintf("      - package: %s\n", pkg.RepoName))
				b.WriteString("        products:\n")
				for _, p := range pkg.Products {
					b.WriteString(fmt.Sprintf("          - %s\n", p))
				}
			}
			b.WriteString("```\n")
			b.WriteString(fmt.Sprintf("- README: %s#readme\n\n", pkg.RepoURL))
		}
	}

	// Emit unresolved packages with search instructions
	if len(unresolved) > 0 {
		b.WriteString("The following packages are not in the curated registry. Search the internet to find and validate them:\n\n")
		for _, pkg := range unresolved {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", pkg.Name, pkg.Reason))
		}
		b.WriteString("\n")
		b.WriteString(`For each unresolved package:
1. Use WebSearch to find the GitHub repository.
2. Confirm it has >500 stars, was updated within 12 months, and has an MIT or Apache 2.0 license.
3. Open the repository's Package.swift to find the exact product name(s).
4. Read the README for SwiftUI integration instructions.
5. Use the add_package MCP tool to add it, or edit project.yml directly.
6. If validation fails, implement the feature with native frameworks instead.

`)
	}

	// Always include the XcodeGen format reference
	b.WriteString(`#### XcodeGen project.yml format

The packages: section goes at the top level. Each key is the repository name (last path component of the URL).

Format rules:
- The packages: key is the repository name (last URL path component), not the product name.
- Use from: for minimum version (semver). Do not use minVersion:, version:, or exactVersion:.
- Every dependency must include package:. Add product: when the product name differs from the package key, or use products: for multiple products.
- Version strings must be quoted (from: "4.5.0").

If a feature would benefit from a package not listed above, search the internet to discover and validate one before adding it.
`)
}

// appendIntegrationConfig injects backend integration configuration into the build prompt.
// models are passed from the planner so we can generate the table→model mapping.
// appName scopes the config lookup to the specific app being built.
// authMethods lists auth methods auto-configured by the pipeline (e.g. "email", "apple").
func appendIntegrationConfig(b *strings.Builder, integrationIDs []string, models []ModelPlan, appName string, authMethods []string) {
	terminal.Info(fmt.Sprintf("Appending integration config to system prompt (integrations: %s)", strings.Join(integrationIDs, ", ")))

	home, err := os.UserHomeDir()
	if err != nil {
		terminal.Warning("Could not get home directory — integration config skipped")
		return
	}
	store := integrations.NewIntegrationStore(filepath.Join(home, ".nanowave"))
	if err := store.Load(); err != nil {
		terminal.Warning(fmt.Sprintf("Could not load integration store: %v — integration config skipped", err))
		return
	}

	for _, id := range integrationIDs {
		switch id {
		case "supabase":
			cfg, _ := store.GetProvider(integrations.ProviderSupabase, appName)
			b.WriteString("\n<integration-config>\n")

			// Credentials
			if cfg != nil && cfg.ProjectURL != "" {
				terminal.Detail("Supabase config", fmt.Sprintf("URL=%s, ref=%s, has_anon_key=%t, has_PAT=%t",
					cfg.ProjectURL, cfg.ProjectRef, cfg.AnonKey != "", cfg.PAT != ""))
				fmt.Fprintf(b, "Supabase Project URL: %s\n", cfg.ProjectURL)
				fmt.Fprintf(b, "Supabase Anon Key: %s\n", cfg.AnonKey)
				b.WriteString("Store these in Config/AppConfig.swift as static constants.\n\n")
			} else {
				terminal.Warning("No Supabase config found for app — using placeholders")
				b.WriteString("Supabase Project URL: https://YOUR_PROJECT_REF.supabase.co\n")
				b.WriteString("Supabase Anon Key: YOUR_ANON_KEY\n")
				b.WriteString("Store these in Config/AppConfig.swift as static constants. The user will replace the placeholders.\n\n")
			}

			// MANDATORY backend-first instructions
			hasMCP := cfg != nil && cfg.PAT != ""
			terminal.Detail("MCP available", fmt.Sprintf("%t (PAT present: %t)", hasMCP, cfg != nil && cfg.PAT != ""))
			if hasMCP {
				terminal.Info("MCP is available — injecting <backend-setup> block into system prompt")

				// Inject auth provider status
				if len(authMethods) > 0 {
					terminal.Detail("Auth methods", strings.Join(authMethods, ", "))
					b.WriteString("## Auth Providers (auto-configured by nanowave)\n\n")
					fmt.Fprintf(b, "Auth providers already configured: %s.\n", strings.Join(authMethods, ", "))
					b.WriteString("Do NOT configure auth providers manually — they are already enabled on the Supabase project.\n\n")
				} else {
					terminal.Detail("Auth methods", "none specified")
				}

				b.WriteString("<backend-setup>\n")
				b.WriteString("## MANDATORY: Backend-First Execution Order\n\n")
				b.WriteString("The Supabase MCP server is connected. You MUST set up the backend BEFORE writing any Swift code.\n\n")
				b.WriteString("### Step 1: Create ALL tables (use mcp__supabase__execute_sql)\n")
				b.WriteString("Create every table needed by the app's models. Include columns, types, foreign keys, constraints, and indexes.\n")
				b.WriteString("Use IF NOT EXISTS for idempotency. Always use snake_case column names.\n\n")

				// Generate model→table mapping from planner models
				if len(models) > 0 {
					var tableNames []string
					for _, m := range models {
						tableNames = append(tableNames, modelToTableName(m.Name))
					}
					terminal.Detail("Models → Tables", fmt.Sprintf("%d models → tables: %s", len(models), strings.Join(tableNames, ", ")))
					b.WriteString("### Required Tables (derived from planned models)\n\n")
					for _, m := range models {
						tableName := modelToTableName(m.Name)
						fmt.Fprintf(b, "**Table: `%s`** (from model `%s`)\n", tableName, m.Name)
						b.WriteString("```sql\nCREATE TABLE IF NOT EXISTS public." + tableName + " (\n")
						for i, prop := range m.Properties {
							colName := camelToSnake(prop.Name)
							pgType := swiftTypeToPG(prop.Type)
							constraints := inferConstraints(prop, i == 0, m.Name)
							fmt.Fprintf(b, "  %s %s%s", colName, pgType, constraints)
							if i < len(m.Properties)-1 {
								b.WriteString(",")
							}
							b.WriteString("\n")
						}
						b.WriteString(");\n```\n\n")
					}
				}

				b.WriteString("### Step 2: Enable RLS on every table\n")
				b.WriteString("```sql\n")
				for _, m := range models {
					fmt.Fprintf(b, "ALTER TABLE public.%s ENABLE ROW LEVEL SECURITY;\n", modelToTableName(m.Name))
				}
				b.WriteString("```\n\n")

				b.WriteString("### Step 3: Create RLS policies for every table\n")
				b.WriteString("See the supabase skill's RLS reference for patterns: public-read + owner-write for content tables, ")
				b.WriteString("actor-write for join tables, owner-only for private data.\n\n")

				b.WriteString("### Step 4: Create storage buckets and policies\n")
				b.WriteString("If the app uploads files (images, documents), create the storage bucket and policies.\n")
				b.WriteString("See the supabase skill's storage-setup reference for patterns.\n\n")

				b.WriteString("### Step 5: Verify\n")
				b.WriteString("Use mcp__supabase__list_tables to confirm all tables exist before proceeding.\n\n")

				b.WriteString("### Step 6: STOP and verify before writing Swift code\n")
				b.WriteString("Call `mcp__supabase__list_tables` NOW. Only proceed to Swift code after confirming tables exist.\n\n")
				b.WriteString("</backend-setup>\n\n")
				terminal.Success("Backend-setup block written to system prompt (with SQL for all tables)")
			} else {
				terminal.Warning("MCP NOT available (no PAT) — backend-setup block SKIPPED from system prompt")
			}

			b.WriteString("Models use Codable (NOT @Model) — Supabase is the persistence layer.\n")
			b.WriteString("</integration-config>\n")
			terminal.Detail("Integration config", "Supabase section complete")
		}
	}
}

// modelToTableName converts a PascalCase model name to a snake_case plural table name.
// e.g. "AppUser" → "app_users", "Post" → "posts", "FollowRelation" → "follow_relations"
func modelToTableName(name string) string {
	snake := camelToSnake(name)
	// Simple pluralization
	if strings.HasSuffix(snake, "s") || strings.HasSuffix(snake, "x") || strings.HasSuffix(snake, "z") {
		return snake + "es"
	}
	if strings.HasSuffix(snake, "y") && len(snake) > 1 {
		prev := snake[len(snake)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return snake[:len(snake)-1] + "ies"
		}
	}
	return snake + "s"
}

// camelToSnake converts camelCase/PascalCase to snake_case.
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // toLower
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// swiftTypeToPG maps Swift type strings to PostgreSQL types.
func swiftTypeToPG(swiftType string) string {
	// Strip optional marker
	t := strings.TrimSuffix(swiftType, "?")

	switch t {
	case "UUID":
		return "UUID"
	case "String":
		return "TEXT"
	case "Int":
		return "INTEGER"
	case "Double", "Float":
		return "DOUBLE PRECISION"
	case "Bool":
		return "BOOLEAN"
	case "Date":
		return "TIMESTAMPTZ"
	case "URL":
		return "TEXT"
	case "[String]":
		return "TEXT[]"
	default:
		// If it looks like a model reference (PascalCase), it's a UUID foreign key
		if len(t) > 0 && t[0] >= 'A' && t[0] <= 'Z' {
			return "UUID"
		}
		return "TEXT"
	}
}

// inferConstraints generates SQL constraints for a property based on its position and type.
func inferConstraints(prop PropertyPlan, isFirst bool, modelName string) string {
	var parts []string
	isOptional := strings.HasSuffix(prop.Type, "?")

	// First property named "id" is the primary key
	if isFirst && strings.ToLower(prop.Name) == "id" {
		parts = append(parts, "PRIMARY KEY")
		if prop.Type == "UUID" {
			parts = append(parts, "DEFAULT gen_random_uuid()")
		}
	}

	if !isOptional && !isFirst {
		parts = append(parts, "NOT NULL")
	}

	if prop.DefaultValue != "" {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", prop.DefaultValue))
	}

	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}
