---
name: "evaluation-checklist"
description: "Visual evaluation checklist for simulator screenshots — what to look for when reviewing generated app UI."
---
# Screenshot Evaluation Checklist

When reading a screenshot, evaluate each category. Report only actual issues found — do not report passing checks.

## 1. Layout & Structure

- [ ] Content is visible and not clipped by safe area or navigation bar
- [ ] No overlapping text or elements
- [ ] No content pushed off-screen
- [ ] Scroll views have appropriate content (not empty)
- [ ] Tab bar / navigation bar is present and functional-looking
- [ ] Content fills the screen appropriately (no excessive empty space)

## 2. Text & Typography

- [ ] All text is readable (sufficient contrast against background)
- [ ] No placeholder text visible ("Lorem ipsum", "Title", "Description", "TODO")
- [ ] Font sizes follow visual hierarchy (headlines > body > captions)
- [ ] No text truncation cutting off meaningful content (ellipsis is OK for long content)
- [ ] Dynamic Type appears respected (text scales appropriately)

## 3. Colors & Theme

- [ ] Colors appear intentional and cohesive (not random)
- [ ] Sufficient contrast between foreground and background
- [ ] No hardcoded black/white that breaks dark mode compatibility
- [ ] Accent color is used consistently for interactive elements
- [ ] Surface colors distinguish different content levels

## 4. Images & Media

- [ ] No broken image placeholders (grey boxes with exclamation marks)
- [ ] Images are properly sized within their containers (not stretched or pixelated)
- [ ] Sample/placeholder images are present where expected (not empty frames)
- [ ] Icons are visible and appropriately sized

## 5. Components & Interaction

- [ ] Buttons look tappable (clear affordance)
- [ ] List items have consistent height and spacing
- [ ] Cards/tiles have uniform appearance
- [ ] Empty states show helpful messages (not blank)
- [ ] Loading states have indicators (if applicable)

## 6. Sample Data

- [ ] App shows sample content on first launch (not empty)
- [ ] Sample data looks realistic (not "Item 1", "Item 2")
- [ ] Numbers, dates, and counts look plausible
- [ ] At least 3-5 sample items in any list view

## Severity Classification

| Severity | Definition | Example |
|----------|-----------|---------|
| **Critical** | App is unusable or blank | White screen, crash, no content rendered |
| **High** | Major feature is broken or hidden | Navigation doesn't work, primary view is empty, text is unreadable |
| **Medium** | Visual quality issue | AppTheme violation, inconsistent spacing, poor contrast |
| **Low** | Minor polish | Border radius inconsistency, shadow too strong, alignment off by pixels |

## Reporting Format

For each issue found:

```
[SEVERITY] Short description
  File: path/to/file.swift (if identifiable)
  What: What's wrong (describe what you see)
  Fix: What needs to change
```
