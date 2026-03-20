---
name: "layout"
description: "visionOS layout patterns: window sizing, volume sizing, spatial depth, Dynamic Type. Use when working on visionOS layout, window management, or spatial arrangement. Triggers: layout, window, volume, sizing, defaultSize, depth."
---
# Layout Patterns (visionOS)

## Window Default Sizing
Windows have no fixed frame — use `.defaultSize()`:
```swift
WindowGroup {
    ContentView()
}
.defaultSize(width: 800, height: 600)
```

## Volume Sizing
Volumes use 3D dimensions:
```swift
WindowGroup {
    VolumetricContentView()
}
.windowStyle(.volumetric)
.defaultSize(width: 0.5, height: 0.5, depth: 0.5, in: .meters)
```

## Spatial Layout with Depth
Use depth alignment for layered content:
```swift
ZStack {
    BackgroundLayer()
    ContentLayer()
        .offset(z: 20)
    ForegroundLayer()
        .offset(z: 40)
}
```

## Adaptive Sizing
```swift
GeometryReader { geometry in
    let columns = geometry.size.width > 800
        ? [GridItem(.adaptive(minimum: 200))]
        : [GridItem(.adaptive(minimum: 150))]

    LazyVGrid(columns: columns) {
        ForEach(items) { item in
            ItemCard(item: item)
        }
    }
}
```

## Dynamic Type + Accessibility
```swift
@Environment(\.dynamicTypeSize) var dynamicTypeSize

var body: some View {
    VStack(spacing: AppTheme.Spacing.medium) {
        Text(title)
            .font(AppTheme.Fonts.headline)
        Text(subtitle)
            .font(AppTheme.Fonts.body)
    }
    .padding(AppTheme.Spacing.large)
}
```

## Safe Area
visionOS windows have built-in safe areas:
```swift
VStack {
    content
}
.padding() // Respect default window padding
```

## NOT Available on visionOS
- No device rotation — windows are freely positioned in space
- No split view controller — use multiple windows instead
- No `UIScreen.main.bounds` — windows are resizable
- No status bar or navigation bar background customization

## Rules
1. Use `.defaultSize()` for initial window dimensions
2. Use `.defaultSize(width:height:depth:in:)` for volumetric windows
3. Use `offset(z:)` for depth-based layering
4. Use GeometryReader for adaptive layouts
5. Design for resizable windows — never assume fixed dimensions
6. Use AppTheme spacing tokens for consistent spatial layout
