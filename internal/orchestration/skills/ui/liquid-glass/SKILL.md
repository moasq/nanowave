---
name: "liquid-glass"
description: "iOS 26 Liquid Glass material: glassEffect modifier, placement rules, modifier ordering, interactivity constraints. Use when applying glass effects, using translucent materials, or styling toolbars/tab bars for iOS 26+. Triggers: glassEffect, .glass, material, translucent, liquid glass, iOS 26, toolbar style."
---
# Liquid Glass

iOS 26 introduces Liquid Glass as the primary surface material. Apply it to key UI surfaces.

WHERE TO APPLY .glassEffect():
- List row backgrounds and card surfaces
- Floating action buttons and toolbar items
- Empty state containers (ContentUnavailableView wrappers)
- Tab bars and bottom toolbars (custom ones)
- Segmented controls and chip groups
- Modal sheet headers

WHERE NOT TO APPLY:
- Text editing surfaces (TextEditor, TextField containers) — keep opaque for readability
- Full-screen backgrounds — use solid AppTheme.Colors.background
- Every single element — use glass on 2-4 key surfaces per screen maximum
- Deeply nested views — glass on parent is enough

MODIFIER ORDER (CRITICAL):
```swift
// CORRECT — glass AFTER layout modifiers
Text("Label")
    .font(.headline)
    .padding()
    .glassEffect(.regular, in: .rect(cornerRadius: 12))

// WRONG — glass before padding
Text("Label")
    .glassEffect()
    .padding()
```

INTERACTIVITY:
- .glassEffect(.regular.interactive()) ONLY on tappable elements (Button, NavigationLink, tappable rows)
- .glassEffect(.regular) on static/display-only surfaces
- Never use .interactive() on labels, headers, or decorative elements

GROUPING:
- Wrap adjacent glass elements in GlassEffectContainer
- Match GlassEffectContainer(spacing:) to the actual layout spacing
```swift
GlassEffectContainer(spacing: 12) {
    HStack(spacing: 12) {
        ActionButton(icon: "pencil")
            .glassEffect(.regular.interactive(), in: .circle)
        ActionButton(icon: "trash")
            .glassEffect(.regular.interactive(), in: .circle)
    }
}
```

PROMINENCE:
- .regular — default for most surfaces
- .prominent — high-emphasis elements (selected states, primary actions)
- Use .prominent sparingly — one per screen section maximum

TINTING:
- Tint with accent color for branded surfaces: .glassEffect(.regular.tint(AppTheme.Colors.accent))
- Keep tint subtle — use the color directly, not at full opacity

LIST ROWS WITH GLASS:
```swift
List(items) { item in
    ItemRow(item: item)
        .listRowBackground(
            Rectangle()
                .glassEffect(.regular.interactive(), in: .rect(cornerRadius: 10))
                .padding(.horizontal, 4)
        )
}
.scrollContentBackground(.hidden)
```

BUTTON STYLES:
- Use .buttonStyle(.glass) for standard glass buttons
- Use .buttonStyle(.glassProminent) for primary/emphasized actions
```swift
Button("Save") { save() }
    .buttonStyle(.glassProminent)
```
