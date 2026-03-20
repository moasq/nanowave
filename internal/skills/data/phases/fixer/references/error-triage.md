# Error Triage

## Contents
- Error priority order
- Cascading resolution
- Property wrapper compatibility
- Common patterns

## Error Priority Order

1. Structural errors (missing class/struct wrapper, malformed file) → rewrite entire file.
2. Syntax errors and missing imports.
3. Protocol conformance and signature mismatches.
4. Scope errors ("Cannot find X in scope") and type mismatches.
5. Rebuild after each pass to get the real next error list.

## Cascading Error Resolution

Fix errors in dependency order — upstream fixes often resolve downstream errors:

1. STRUCTURAL → Rewrite corrupted files (>=3 scope errors in first 10 lines).
2. PROTOCOL_CONFORMANCE → Add missing Identifiable, Hashable, Codable, Sendable.
3. MISSING_ARGUMENTS → Fix init signatures after conformance is resolved.
4. SCOPE_ERROR / TYPE_MISMATCH → Often auto-resolved by earlier fixes.

## Property Wrapper Compatibility

| Observable Type            | Property Wrapper in View |
|----------------------------|--------------------------|
| @Observable (Swift 5.9+)   | @State                   |
| ObservableObject protocol  | @StateObject             |

Error "Generic struct 'StateObject' requires..." → Change @StateObject to @State.

## Common Patterns

- Many scope errors in one file often means the file is malformed or missing a wrapper.
- "Ambiguous reference" errors → check for type name conflicts with Apple frameworks.
- "Missing argument" after fixing protocol conformance → init signature changed.
- Extension linker error "undefined symbol: _main" → missing @main entry point file.
