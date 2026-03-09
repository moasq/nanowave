---
name: "performance"
description: "Performance: LazyVStack, task modifiers, image caching, profiling. Use when implementing UI patterns related to performance."
tags: "swiftui, ui-patterns"
---
# SwiftUI Performance Reference

Comprehensive guide to performance optimization, lazy loading, image handling, and concurrency patterns.

## Avoid Redundant State Updates

```swift
// BAD - triggers update even if value unchanged
.onReceive(publisher) { value in
    self.currentValue = value
}

// GOOD - only update when different
.onReceive(publisher) { value in
    if self.currentValue != value {
        self.currentValue = value
    }
}
```


## Pass Only What Views Need

```swift
// Good - pass specific values
struct SettingsView: View {
    @State private var config = AppConfig()

    var body: some View {
        VStack {
            ThemeSelector(theme: config.theme)
            FontSizeSlider(fontSize: config.fontSize)
        }
    }
}
```


## POD Views for Fast Diffing

POD (Plain Old Data) views use `memcmp` for fastest diffing — only simple value types, no property wrappers.

```swift
// POD view - fastest diffing
struct FastView: View {
    let title: String
    let count: Int
    var body: some View { Text("\(title): \(count)") }
}
```

**Advanced**: Wrap expensive non-POD views in POD parent views.


## Task Cancellation

```swift
struct DataView: View {
    @State private var data: [Item] = []

    var body: some View {
        List(data) { item in Text(item.name) }
        .task {
            data = await fetchData()  // Auto-cancelled on disappear
        }
    }
}
```


## Eliminate Unnecessary Dependencies

```swift
// Good - narrow dependency
struct ItemRow: View {
    let item: Item
    let themeColor: Color  // Only depends on what it needs
    var body: some View {
        Text(item.name).foregroundStyle(themeColor)
    }
}
```


## AsyncImage Best Practices

```swift
AsyncImage(url: imageURL) { phase in
    switch phase {
    case .empty:
        ProgressView()
    case .success(let image):
        image
            .resizable()
            .aspectRatio(contentMode: .fit)
    case .failure:
        Image(systemName: "photo")
            .foregroundStyle(.secondary)
    @unknown default:
        EmptyView()
    }
}
.frame(width: 200, height: 200)
```


## SF Symbols

```swift
Image(systemName: "star.fill")
    .foregroundStyle(.yellow)

Image(systemName: "heart.fill")
    .symbolRenderingMode(.multicolor)

// Animated symbols (iOS 17+)
Image(systemName: "antenna.radiowaves.left.and.right")
    .symbolEffect(.variableColor)
```
