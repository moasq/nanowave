package claude

import "context"

// ClaudeAgent abstracts Claude Code interactions for pipeline phases.
type ClaudeAgent interface {
	// Generate runs a one-shot prompt and returns the response.
	Generate(ctx context.Context, userMessage string, opts GenerateOpts) (*Response, error)

	// GenerateStreaming runs a prompt with real-time event streaming.
	GenerateStreaming(ctx context.Context, userMessage string, opts GenerateOpts, onEvent func(StreamEvent)) (*Response, error)

	// RunInteractive runs an interactive session with HITL support.
	// Claude streams events via onEvent. When Claude asks a question,
	// onQuestion is called with the question text; it must return the
	// user's response (blocking). The session continues until Claude
	// finishes without asking a question.
	RunInteractive(ctx context.Context, prompt string, opts InteractiveOpts, onEvent func(StreamEvent), onQuestion func(question string) string) (*Response, error)
}

// InteractiveOpts extends GenerateOpts with interactive session settings.
type InteractiveOpts struct {
	GenerateOpts
}
