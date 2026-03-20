---
name: "tabbar-patterns"
description: "Tab bar visibility, styling, safe area interaction, and content-behind-tab-bar patterns for SwiftUI TabView."
---
# Tab Bar Management Patterns

## Tab Bar Visibility

Hide the tab bar on immersive screens (video feed, full-screen content, detail views):

```swift
// Per-view tab bar hiding (iOS 16+)
.toolbar(.hidden, for: .tabBar)
```

Apply this modifier on the view inside the `TabView`, not on the `TabView` itself.

## Tab Bar Styling

For screens where the tab bar IS visible, apply translucent material:

```swift
TabView(selection: $selectedTab) {
    // ... tabs
}
.toolbarBackground(.ultraThinMaterial, for: .tabBar)
.toolbarBackgroundVisibility(.visible, for: .tabBar)
```

## Content Behind Tab Bar — Safe Area Rules

When a scroll view uses `.ignoresSafeArea()` (e.g., full-screen video feeds), the scroll area extends behind the tab bar. But `GeometryReader` still reports the **safe area** height (excluding tab bar). This mismatch causes visible gaps.

### Fix: Use `.containerRelativeFrame` (iOS 17+, preferred)

```swift
ScrollView(.vertical) {
    LazyVStack(spacing: 0) {
        ForEach(items) { item in
            ItemView(item: item)
                .containerRelativeFrame([.horizontal, .vertical])
        }
    }
    .scrollTargetLayout()
}
.scrollTargetBehavior(.paging)
.ignoresSafeArea()
```

`.containerRelativeFrame` sizes relative to the nearest scroll view's visible frame, which includes the area behind the tab bar when `.ignoresSafeArea()` is applied. This eliminates the height mismatch that occurs with manual `GeometryReader` + `.frame()` sizing.

### Do NOT use GeometryReader for paging card sizing

```swift
// BAD — causes gap equal to tab bar height
GeometryReader { geometry in
    ScrollView {
        ForEach(items) { item in
            ItemView(item: item)
                .frame(height: geometry.size.height) // safe area height, NOT full screen
        }
    }
    .ignoresSafeArea() // scroll area = full screen
}

// GOOD — use containerRelativeFrame instead (see above)
```

## Overlay Safe Area Awareness

When overlay content (action buttons, labels) sits at the bottom of a full-screen view, it must account for safe area insets — especially when the tab bar is hidden:

```swift
GeometryReader { geo in
    VStack {
        Spacer()
        overlayContent
            .padding(.bottom, max(AppTheme.Spacing.xxl, geo.safeAreaInsets.bottom + AppTheme.Spacing.sm))
    }
}
```

## Header Safe Area Awareness

When using `.ignoresSafeArea()` on a parent, headers must manually account for the top safe area (Dynamic Island / status bar):

```swift
GeometryReader { geo in
    VStack {
        headerContent
            .padding(.top, geo.safeAreaInsets.top + AppTheme.Spacing.sm)
        Spacer()
    }
}
```

## Decision Table: When to Hide vs Show Tab Bar

| Screen Type | Tab Bar | Modifier |
|---|---|---|
| Full-screen video feed | Hidden | `.toolbar(.hidden, for: .tabBar)` |
| Immersive media player | Hidden | `.toolbar(.hidden, for: .tabBar)` |
| Detail/drill-down views | Hidden | `.toolbar(.hidden, for: .tabBar)` |
| Browse/discovery screens | Visible | (default, no modifier needed) |
| Profile/settings | Visible | (default) |
| Onboarding/auth flows | Hidden | `.toolbar(.hidden, for: .tabBar)` |
