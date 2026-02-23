# Build Loop

## Contents
- Build process
- Error fixing strategy
- Animation safety

## Build Process

1. Write ALL planned files first, then run the build command.
2. Read compiler errors carefully — identify the error type before fixing.
3. Fix the root cause first, not symptoms.
4. Rebuild after each fix batch to get the real next error list.
5. Finish ONLY when the build succeeds with zero errors.

## Cascading Error Resolution (fix in this order)

1. STRUCTURAL (missing class/struct wrapper) → rewrite entire file.
2. PROTOCOL_CONFORMANCE → may reveal missing arguments after fixing.
3. MISSING_ARGUMENTS → usually final layer.
4. SCOPE_ERROR and TYPE_MISMATCH → often resolved by earlier fixes.

If >=3 "Cannot find ... in scope" errors in the first 10 lines → file is corrupted. Rewrite it entirely.

## Investigation Strategy

1. READ error messages carefully — identify the error type.
2. INVESTIGATE before fixing — read related files, understand the codebase.
3. FIX based on evidence — never guess.

## Animation Safety — AsyncRenderer Crash Prevention

- NEVER use .symbolEffect(.bounce, value:) where the value changes at the same time as preferredColorScheme.
- NEVER apply .transaction { $0.disablesAnimations = true } at the root level while child views have explicit .animation() modifiers or .symbolEffect() triggers.
- When switching appearance (dark/light mode): do NOT trigger .symbolEffect, .animation(.spring), or other explicit animations on the same state change that drives preferredColorScheme.
- Avoid stacking multiple .animation() modifiers on the same view — consolidate into one, or use withAnimation {} at the call site.

## Common API Pitfalls

- String(localized:) ignores .environment(\.locale) — uses system locale. For in-view text, use direct string literals: Text("Settings").
- .environment(\.locale) does NOT set layoutDirection — must ALSO set .environment(\.layoutDirection, .rightToLeft) for RTL languages.
