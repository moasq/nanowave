package asc

// AgentTools returns the tool allowlist for the ASC agent.
// Write/Edit are included because the agent needs to create ExportOptions.plist,
// fix Contents.json for icons, and update build settings when needed.
func AgentTools() []string {
	return []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}
}
