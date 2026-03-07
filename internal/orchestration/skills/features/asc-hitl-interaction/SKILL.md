---
name: asc-hitl-interaction
description: "Human-in-the-loop interaction patterns for ASC operations. Use when asking the user for metadata, settings, or confirmations during App Store Connect workflows."
---

# ASC Human-in-the-Loop Interaction

The terminal UI renders your questions with pickers based on the OPTIONS block format. **Every question MUST include an OPTIONS block.** Follow these patterns for all user-facing questions.

## Core Rules

1. **Ask ONE question at a time** — never dump multiple questions in a single message
2. **Always offer a suggestion** — propose a value based on app context so the user can accept or tweak it
3. **Every question MUST use an OPTIONS block** — the terminal renders these as navigable pickers
4. If the user responds with "yes", "ok", "sure", "y", or similar — use your suggested value
5. If the user responds with "Use your best judgment for this and all remaining fields" — enter auto mode (see below)
6. Keep questions concise — one or two sentences max, then the OPTIONS block

## Suggestion Fields (description, keywords, copyright, what's new, etc.)

When you have a suggested value, present it as the first option. Always include a text-entry option so the user can type a custom value.

Format:
```
**Description**: I'd suggest:

> "Your suggested description text here."

[OPTIONS]
- Use this suggestion | Accept the suggested description
- Enter my own | [INPUT] Type a custom value
[/OPTIONS]
```

Another example:
```
**Copyright**: I'd suggest: 2026 Mohammed Al-Quraini

[OPTIONS]
- Use this suggestion | Accept the suggested copyright
- Enter my own | [INPUT] Type a custom value
[/OPTIONS]
```

Rules:
- Show your suggestion ABOVE the OPTIONS block as a quote or inline text
- First option is always "Use this suggestion" (or a short label of the value if it's brief)
- Text-entry options MUST have `[INPUT]` at the start of their description — this tells the terminal to show a text input prompt
- The UI appends "Let AI decide" and "AI decides all" automatically — do NOT add them

## Fixed-Option Fields (category, age rating, content rights, etc.)

When a field has a **fixed set of valid values**, list all valid values as options.

Format:
```
What age rating fits your app? This app has no objectionable content.

[OPTIONS]
- 4+ | No objectionable content (recommended)
- 9+ | Mild cartoon or fantasy violence
- 12+ | Infrequent mild language, simulated gambling
- 17+ | Frequent intense violence, mature themes
[/OPTIONS]
```

Rules:
- Put your **recommended choice first**
- Keep descriptions under 60 characters
- Include ALL valid values
- The UI appends "Let AI decide" and "AI decides all" automatically — do NOT add them

Fields that MUST list all valid values:
- App category, age rating, content rights, encryption compliance
- Any API field that only accepts specific enum values
- Confirmations (Yes/No/Cancel style)

## URL Fields

URLs require the user to provide a value they control. AI cannot generate hosted URLs.

Format:
```
**Support URL**: A publicly accessible URL is required.

[OPTIONS]
- Enter URL | [INPUT] Type your support page URL
- Use GitHub profile | [INPUT] Enter your GitHub username
[/OPTIONS]
```

## Auto Mode — AI Decides All

When the user selects "AI decides all", auto-fill everything you can WITHOUT asking. But some items are **impossible for AI to resolve** — you MUST still ask about these:

### Always ask (even in auto mode)
- **URLs the user must own** — Support URL, Privacy Policy URL (AI cannot host pages)
- **Screenshots** — need actual images; offer upload or simulator capture
- **Incomplete agreements** — must be accepted in browser (see asc-manual-actions skill)
- **Content declarations** — content rights, encryption (legal liability on the user)

### Auto-fill without asking
- Description, keywords, what's new, promotional text
- Age rating, app category
- Copyright
- Build selection (use latest valid build)
- Version string

When auto-filling, proceed through all auto-fillable fields in one turn (no questions), then stop and ask about the first unresolvable item. After resolving it, continue auto-filling until the next unresolvable item or until done.

## Confirmation Before Destructive Actions

Even in auto mode, ALWAYS show a preview and ask for confirmation before submit/publish:

```
Ready to submit. Proceed?

[OPTIONS]
- Yes, submit | Submit for App Store review
- Review changes | Show the preview again
- Cancel | Stop without submitting
[/OPTIONS]
```
