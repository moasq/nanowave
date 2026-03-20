---
name: "layout"
description: "watchOS layout: glanceable design, watch-sized stacks, containerRelativeFrame, 1-2 second interaction principles. Use when working on shared watchOS patterns related to layout."
---
# SwiftUI Layout (watchOS)

Glanceable design: users interact with Apple Watch for 1-2 seconds. Every view must communicate its purpose instantly.

## Watch Screen Constraints

- Screen sizes: 40mm (162pt), 41mm (176pt), 44mm (184pt), 45mm (198pt), 49mm Ultra (205pt)
- Always design for the smallest screen, let larger screens breathe
- No landscape orientation — always portrait
- No split views, no multi-column layouts

## Layout Principles

### Vertical Stacking
```swift
// Good - simple vertical stack, fills naturally
VStack(spacing: 8) {
    Text("Heart Rate")
        .font(AppTheme.Fonts.caption2)
        .foregroundStyle(.secondary)
    Text("72")
        .font(AppTheme.Fonts.largeTitle)
    Text("BPM")
        .font(AppTheme.Fonts.caption)
        .foregroundStyle(.secondary)
}
```

### Full-Width Elements
```swift
// Good - buttons fill width on watch
Button("Start Workout") {
    startWorkout()
}
.buttonStyle(.borderedProminent)
// Buttons naturally fill width on watchOS — no .frame(maxWidth:) needed
```

### containerRelativeFrame (preferred over GeometryReader)
```swift
Image("chart")
    .resizable()
    .containerRelativeFrame(.horizontal) { width, _ in
        width * 0.9
    }
    .aspectRatio(contentMode: .fit)
```

## View Structure

### Prefer Modifiers Over Conditional Views
```swift
// Good - same view, different states
SomeView()
    .opacity(isVisible ? 1 : 0)

// Avoid - creates/destroys view identity
if isVisible {
    SomeView()
}
```

### Extract Subviews Into Separate Structs
```swift
// Good - separate struct, SwiftUI can skip body when inputs unchanged
struct MetricDisplay: View {
    let value: Int
    let unit: String

    var body: some View {
        VStack {
            Text("\(value)")
                .font(AppTheme.Fonts.title2)
            Text(unit)
                .font(AppTheme.Fonts.caption2)
                .foregroundStyle(.secondary)
        }
    }
}
```

## What NOT to Use on watchOS

- No `GeometryReader` — use `containerRelativeFrame` or let stacks fill naturally
- No `UIScreen.main.bounds` — doesn't exist on watchOS
- No size classes — watch has one size class
- No `NavigationSplitView` — watch is single-column only
- No adaptive grids with many columns — use simple `VStack` or single-column `List`
- No `.frame(maxWidth: 700)` readability constraints — the screen is already small
- No `AnyLayout` switching — there's only one layout direction on watch

## Layout Rules
1. Keep views shallow — 2-3 levels of nesting maximum
2. Use `List` for scrollable content, `ScrollView` for custom layouts
3. Large text (`.title`, `.largeTitle`) for primary information
4. Small text (`.caption`, `.caption2`) for labels and secondary info
5. Minimum tap target: 44pt (Apple requirement, critical on small screen)
6. Use `@ScaledMetric` for custom dimensions that respect Dynamic Type
