package claude

import (
	"context"
	"log"
	"sync"
	"time"
)

// questionState tracks the state machine for detecting when Claude is asking a question.
//
// State transitions:
//
//	idle → [assistant text received] → maybeQuestion
//	maybeQuestion → [tool_use_start received] → idle (not a question)
//	maybeQuestion → [result received] → questionConfirmed (turn ended with text, no tools)
//	maybeQuestion → [idle timeout] → questionConfirmed (Claude waiting for input)
//	questionConfirmed → [send user response] → idle
type questionState int

const (
	qsIdle questionState = iota
	qsMaybeQuestion
)

const questionIdleTimeout = 5 * time.Second

// RunInteractive runs an interactive Claude Code session with human-in-the-loop support.
// It uses StartInteractiveStreaming (bidirectional stream-json) and a state machine to
// detect when Claude is asking a question vs autonomously using tools.
//
// When Claude emits assistant text followed by no tool_use_start (detected via result
// event or idle timeout), onQuestion is called with the question text. The caller
// must return the user's response, which is sent back to Claude.
//
// The session continues until Claude finishes a turn without asking a question
// (i.e., emits a result event after tool use, or the process exits).
func (c *Client) RunInteractive(ctx context.Context, prompt string, opts InteractiveOpts, onEvent func(StreamEvent), onQuestion func(question string) string) (*Response, error) {
	var (
		mu                sync.Mutex
		state             = qsIdle
		lastAssistantText string
		idleTimer         *time.Timer
		questionCh        = make(chan string, 1) // receives detected question text
	)

	// resetTimer stops any pending idle timer.
	resetTimer := func() {
		if idleTimer != nil {
			idleTimer.Stop()
			idleTimer = nil
		}
	}

	// startIdleTimer starts a timer that fires questionCh if Claude goes idle
	// after emitting assistant text without a tool call.
	startIdleTimer := func(text string) {
		resetTimer()
		idleTimer = time.AfterFunc(questionIdleTimeout, func() {
			mu.Lock()
			defer mu.Unlock()
			if state == qsMaybeQuestion {
				state = qsIdle
				select {
				case questionCh <- text:
				default:
				}
			}
		})
	}

	// wrappedOnEvent tracks events for question detection and forwards to caller.
	wrappedOnEvent := func(ev StreamEvent) {
		// Forward to caller first.
		if onEvent != nil {
			onEvent(ev)
		}

		mu.Lock()
		defer mu.Unlock()

		switch ev.Type {
		case "assistant":
			if ev.Text != "" {
				lastAssistantText = ev.Text
				state = qsMaybeQuestion
				// Start idle timer — if no tool_use_start arrives within the
				// timeout, we conclude Claude is waiting for user input.
				startIdleTimer(ev.Text)
			}

		case "tool_use_start", "tool_use":
			// Claude is working autonomously — not a question.
			state = qsIdle
			resetTimer()

		case "result":
			// Turn ended. If we were in maybeQuestion state, Claude ended
			// its turn with text and no tool call → it's a question.
			if state == qsMaybeQuestion && lastAssistantText != "" {
				state = qsIdle
				resetTimer()
				select {
				case questionCh <- lastAssistantText:
				default:
				}
			} else {
				state = qsIdle
				resetTimer()
			}
		}
	}

	session, err := c.StartInteractiveStreaming(ctx, prompt, opts.GenerateOpts, wrappedOnEvent)
	if err != nil {
		return nil, err
	}

	// HITL loop: wait for questions or session completion.
	for {
		select {
		case <-ctx.Done():
			session.CloseInput()
			_, _ = session.Wait()
			return nil, ctx.Err()

		case question := <-questionCh:
			log.Printf("[interactive] HITL: Claude is asking for input")
			userResponse := onQuestion(question)
			log.Printf("[interactive] HITL: user responded")

			// Reset state for next round.
			mu.Lock()
			lastAssistantText = ""
			state = qsIdle
			mu.Unlock()

			if err := session.SendUserMessage(userResponse); err != nil {
				log.Printf("[interactive] failed to send user message: %v", err)
				session.CloseInput()
				return session.Wait()
			}

		case result := <-session.responseCh:
			// Session completed. But check if there's a pending question
			// that arrived just before the result.
			mu.Lock()
			pendingState := state
			pendingText := lastAssistantText
			state = qsIdle
			resetTimer()
			mu.Unlock()

			if pendingState == qsMaybeQuestion && pendingText != "" {
				// The result arrived but there's pending assistant text that
				// didn't get a tool call — this was likely a final question.
				// However, since the process has ended, we just return the result.
			}

			// Put the result back so Wait() can return it.
			session.responseCh <- result
			return session.Wait()
		}
	}
}
