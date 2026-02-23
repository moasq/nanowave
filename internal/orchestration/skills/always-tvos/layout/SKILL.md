---
name: "layout"
description: "tvOS layout patterns: focus-driven layout, large-screen content grids, 16:9 safe area, shelf-based browsing. Use when working on tvOS view layouts, content arrangement, or adapting UI for the big screen. Triggers: layout, grid, shelf, LazyVGrid, focus section."
---
# Layout Patterns (tvOS)

## Screen Dimensions
- tvOS renders at 1920x1080 (1080p) with a safe area inset
- Content should respect the safe area — avoid placing interactive elements at edges
- Use generous spacing — users view from 10+ feet away

## Primary Layout Patterns

SHELF / HORIZONTAL SCROLL (most common tvOS pattern):
```swift
ScrollView(.horizontal, showsIndicators: false) {
    LazyHStack(spacing: 40) {
        ForEach(items) { item in
            Button {
                select(item)
            } label: {
                ItemCard(item: item)
            }
            .buttonStyle(.card)
        }
    }
    .padding(.horizontal, 80)
}
```

CONTENT GRID:
```swift
LazyVGrid(columns: [
    GridItem(.adaptive(minimum: 250), spacing: 40)
], spacing: 40) {
    ForEach(items) { item in
        Button {
            select(item)
        } label: {
            ItemCard(item: item)
        }
        .buttonStyle(.card)
    }
}
.padding(.horizontal, 80)
```

FULL-WIDTH HERO / BANNER:
```swift
VStack(alignment: .leading, spacing: 20) {
    // Hero image
    Image(item.heroImage)
        .resizable()
        .aspectRatio(16/9, contentMode: .fill)
        .frame(height: 400)
        .clipped()
        .focusable()

    // Metadata row
    HStack(spacing: 24) {
        Text(item.title)
            .font(.title)
        Text(item.subtitle)
            .foregroundStyle(.secondary)
    }
    .padding(.horizontal, 80)
}
```

## Focus Sections
Use `.focusSection()` to group focusable elements:
```swift
VStack(spacing: 60) {
    // Section 1
    VStack(alignment: .leading) {
        Text("Trending")
            .font(.title3)
            .padding(.horizontal, 80)
        ShelfRow(items: trending)
    }
    .focusSection()

    // Section 2
    VStack(alignment: .leading) {
        Text("New Releases")
            .font(.title3)
            .padding(.horizontal, 80)
        ShelfRow(items: newReleases)
    }
    .focusSection()
}
```

## Spacing Guidelines
| Element | Spacing |
|---------|---------|
| Between sections | 60pt |
| Between cards | 40pt |
| Horizontal padding | 80pt |
| Between text lines | 12-20pt |

## NOT Available on tvOS
- No `NavigationSplitView` three-column layout — use tab + shelf pattern
- No adaptive layout for multiple device sizes — tvOS is always 1080p
- No `.safeAreaInset` bottom bar — tvOS has no bottom bar concept
- No `GeometryReader` for rotation — tvOS is always landscape

## Rules
1. Use horizontal shelves as the primary browsing pattern
2. All interactive elements MUST be focusable (`Button`, `.focusable()`)
3. Generous spacing — minimum 40pt between interactive elements for focus clarity
4. Horizontal padding of 80pt for content margins
5. Use `.buttonStyle(.card)` for media card interactions
6. Group related content with `.focusSection()` for predictable focus movement
