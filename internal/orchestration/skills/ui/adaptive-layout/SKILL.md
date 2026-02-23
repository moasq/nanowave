---
name: "adaptive-layout"
description: "Adaptive layout for iPad and universal apps: NavigationSplitView, size classes, horizontalSizeClass, presentations, HIG compliance. Use when building iPad-optimized layouts, supporting multiple screen sizes, or adapting UI for iPhone+iPad. Triggers: iPad, universal, NavigationSplitView, horizontalSizeClass, size class, adaptive."
---
# Adaptive Layout — iPhone & iPad

## Core Principle
Let SwiftUI adapt automatically. Never check `UIDevice.current.userInterfaceIdiom` or `UIScreen.main.bounds` for layout decisions. Use size classes and adaptive containers instead.

## Navigation

### NavigationSplitView (primary pattern for list-detail)
Use `NavigationSplitView` for ANY list-detail flow. It collapses to `NavigationStack` on iPhone and shows sidebar+detail on iPad — zero conditional code.

```swift
@State private var selectedItem: Item?

NavigationSplitView {
    List(items, selection: $selectedItem) { item in
        NavigationLink(value: item) { ItemRow(item: item) }
    }
    .navigationTitle("Items")
} detail: {
    if let selectedItem {
        ItemDetailView(item: selectedItem)
    } else {
        ContentUnavailableView("Select an Item", systemImage: "doc")
    }
}
```

Three-column pattern (sidebar categories → list → detail):
```swift
NavigationSplitView {
    List(categories, selection: $selectedCategory) { ... }
} content: {
    List(filteredItems, selection: $selectedItem) { ... }
} detail: {
    if let selectedItem { DetailView(item: selectedItem) }
    else { ContentUnavailableView(...) }
}
```

Use `NavigationStack` ONLY for purely linear flows (onboarding, checkout).

## Size Classes

```swift
@Environment(\.horizontalSizeClass) private var horizontalSizeClass
```

| Context | horizontalSizeClass |
|---|---|
| iPhone portrait | `.compact` |
| iPhone landscape | `.compact` |
| iPad full-screen | `.regular` |
| iPad Split View (narrow) | `.compact` |
| iPad Split View (wide) | `.regular` |

**Critical:** iPad in Split View multitasking can report `.compact`. This is why `UIDevice.current` is wrong — always use size classes.

### Switching layout axis:
```swift
@Environment(\.horizontalSizeClass) private var sizeClass

var body: some View {
    let layout = sizeClass == .compact
        ? AnyLayout(VStackLayout(spacing: 16))
        : AnyLayout(HStackLayout(spacing: 24))
    layout {
        ContentBlockA()
        ContentBlockB()
    }
}
```

## ViewThatFits (component-level adaptation)

Use when the decision is purely about available space — no environment reading needed:
```swift
ViewThatFits {
    HStack(spacing: 16) { icon; title; subtitle; actionButton }
    VStack(alignment: .leading, spacing: 8) {
        HStack { icon; title }
        subtitle
        actionButton
    }
}
```

**When to use which:**
- `ViewThatFits` → component-level (a card, a header, a toolbar item)
- `horizontalSizeClass` → screen-level (different page structures)

## Adaptive Grids

Always use `GridItem(.adaptive(minimum:maximum:))` — never hardcode column counts:
```swift
LazyVGrid(
    columns: [GridItem(.adaptive(minimum: 160, maximum: 320))],
    spacing: 16
) {
    ForEach(items) { item in CardView(item: item) }
}
.padding()
```
Automatically: 2 columns on iPhone, 3-4 on iPad portrait, 4-6 on iPad landscape.

## Presentations — Sheet & Popover

### Popovers auto-adapt
```swift
.popover(isPresented: $showOptions) {
    OptionsView()
}
```
- iPad (regular): floating popover anchored to source
- iPhone (compact): automatically becomes a sheet

### Sheet behavior
```swift
.sheet(isPresented: $showSheet) {
    SheetContent()
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.visible)
}
```
- iPhone portrait: respects detents
- iPad wide: centered floating modal (detents ignored)

### Confirmation dialogs
Always use `.confirmationDialog` — it becomes an action sheet on iPhone, popover on iPad:
```swift
.confirmationDialog("Options", isPresented: $showDialog) {
    Button("Option A") { }
    Button("Cancel", role: .cancel) { }
}
```

## Spacing & Readability

- Use `.padding()` with no arguments — SwiftUI applies 16pt on iPhone, 20pt on iPad automatically.
- For long-form text on wide screens, constrain readability:
```swift
ScrollView {
    content
        .frame(maxWidth: 700)
        .frame(maxWidth: .infinity)
}
```

- Use `@ScaledMetric` for custom dimensions that respect Dynamic Type:
```swift
@ScaledMetric(relativeTo: .body) private var iconSize: CGFloat = 24
```

## Relative Sizing

Prefer `containerRelativeFrame` over `GeometryReader`:
```swift
Image("photo")
    .containerRelativeFrame(.horizontal, count: 3, span: 1, spacing: 16)
```

Reserve `GeometryReader` only for complex calculations (parallax, custom alignment).

## STRICT RULES

1. **NEVER** use `UIDevice.current.userInterfaceIdiom` for layout — breaks iPad multitasking
2. **NEVER** use `UIScreen.main.bounds` for sizing — doesn't respond to multitasking
3. **NEVER** use `#if targetEnvironment` for layout decisions
4. **NEVER** hardcode frame widths (e.g., `.frame(width: 375)`)
5. **NEVER** hardcode grid column counts — use `.adaptive(minimum:)`
6. **ALWAYS** use Dynamic Type text styles (`.font(.body)`, `.font(.title)`)
7. **ALWAYS** provide a detail placeholder in `NavigationSplitView` for iPad empty state
8. **ALWAYS** use `.popover` for contextual actions — SwiftUI auto-adapts
9. **ALWAYS** use `.leading/.trailing` — never `.left/.right`
