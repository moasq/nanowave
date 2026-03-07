package terminal

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// ChecklistStatus represents the completion status of a checklist item.
type ChecklistStatus int

const (
	ChecklistRunning ChecklistStatus = iota
	ChecklistSuccess
	ChecklistWarning
	ChecklistSkipped
	ChecklistError
)

// Checklist renders a static, growing list of items with status markers.
// Unlike ProgressDisplay, each completed item is permanent and never redrawn.
type Checklist struct {
	spinner     *Spinner
	hasActive   bool
	activeLabel string
	interactive bool
}

// NewChecklist creates a new checklist display.
func NewChecklist() *Checklist {
	return &Checklist{
		interactive: term.IsTerminal(int(os.Stdout.Fd())),
	}
}

// StartItem begins a new checklist item with a spinner.
// If a previous item is still running, it is auto-completed as success.
func (c *Checklist) StartItem(label string) {
	if c.hasActive {
		c.CompleteItem(ChecklistSuccess, c.activeLabel)
	}

	c.activeLabel = label
	c.hasActive = true

	if c.interactive {
		c.spinner = NewSpinner(label)
		c.spinner.Start()
	}
}

// CompleteItem stops the active spinner and prints the final status line.
func (c *Checklist) CompleteItem(status ChecklistStatus, detail string) {
	if c.spinner != nil {
		c.spinner.Stop()
		c.spinner = nil
	}
	c.hasActive = false
	c.activeLabel = ""

	var marker, color string
	switch status {
	case ChecklistSuccess:
		marker = "\u2713" // checkmark
		color = Green
	case ChecklistWarning:
		marker = "!"
		color = Yellow
	case ChecklistSkipped:
		marker = "\u2014" // em dash
		color = Yellow
	case ChecklistError:
		marker = "\u2717" // ballot x
		color = Red
	default:
		marker = "\u2022" // bullet
		color = Cyan
	}

	fmt.Printf("  %s%s%s%s %s%s\n", Bold, color, marker, Reset, detail, Reset)
}

// Finish stops any active spinner and prints a blank separator line.
func (c *Checklist) Finish() {
	if c.hasActive {
		c.CompleteItem(ChecklistSuccess, c.activeLabel)
	}
	fmt.Println()
}
