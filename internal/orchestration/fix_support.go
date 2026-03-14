package orchestration

import "github.com/moasq/nanowave/internal/mcpregistry"

// ComposeFixerAppendPrompt exposes the existing fixer phase prompt for CLI flows
// that repair existing projects outside the full orchestration pipeline.
func ComposeFixerAppendPrompt(platform string) (string, error) {
	return composeCoderAppendPrompt("fixer", platform)
}

// DefaultAgenticTools returns the core Claude tools plus all registered MCP tools.
func DefaultAgenticTools() []string {
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)

	tools := make([]string, 0, len(coreAgenticTools)+len(reg.AllTools()))
	tools = append(tools, coreAgenticTools...)
	tools = append(tools, reg.AllTools()...)
	return tools
}
