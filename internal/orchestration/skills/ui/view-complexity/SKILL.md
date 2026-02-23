---
name: "view-complexity"
description: "View complexity management: 30-line body rule, computed property extraction, subview decomposition, type-check timeout prevention. Use when a view body is getting long, refactoring large views, or hitting Swift type-checker timeouts. Triggers: long body, refactor view, extract subview, type-check timeout, view too complex."
---
# View Complexity

VIEW BODY COMPLEXITY (CRITICAL):

- If a View `body` grows beyond about 30 lines, extract sections into computed properties.
- If nesting is deeper than 3 levels, flatten the structure by extracting sub-sections.
- Prefer a body that reads like a table of contents:
  - `headerSection`
  - `contentSection`
  - `footerSection`

REQUIRED PATTERN:

```swift
var body: some View {
    VStack(spacing: AppTheme.Spacing.medium) {
        headerSection
        contentSection
        actionsSection
    }
}

private var headerSection: some View { ... }
private var contentSection: some View { ... }
private var actionsSection: some View { ... }
```

TYPE-CHECK TIMEOUT FIX:

- Error pattern: "The compiler is unable to type-check this expression in reasonable time"
- Fix by splitting large expressions:
  - Move long `HStack/VStack/ZStack` branches into computed properties
  - Move complex `Chart` blocks into computed properties
  - Move long modifier chains into intermediate variables/properties
- Keep behavior exactly the same; only refactor structure.

WHEN EDITING EXISTING FILES:

- If your new change pushes body size over the threshold, include the refactor in the same edit.
- Do not leave a giant body as technical debt.
