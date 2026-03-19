---
name: "navigation"
description: "watchOS navigation: NavigationStack, vertical page TabView, sheets, alerts \u2014 no NavigationSplitView. Use when working on shared watchOS patterns related to navigation."
---
# Navigation Patterns (watchOS)

## Pattern Selection Guide

| Pattern | When to Use |
|---------|-------------|
| `NavigationStack` | Hierarchical drill-down (list → detail) |
| `TabView` vertical page | 2-4 peer sections (swipe vertically) |
| `.sheet(item:)` | Quick input, secondary action |
| `.alert` / `.confirmationDialog` | Confirmation, destructive actions |
| `.fullScreenCover` | Immersive single-purpose experience |

## NavigationStack (Primary Pattern)
```swift
NavigationStack {
    List(items) { item in
        NavigationLink(value: item) {
            ItemRow(item: item)
        }
    }
    .navigationDestination(for: Item.self) { item in
        ItemDetailView(item: item)
    }
    .navigationTitle("Items")
}
```

## TabView — Vertical Page Style
watchOS uses vertical paging (swipe up/down), not horizontal tabs:

```swift
TabView {
    SummaryView()
    DetailView()
    SettingsView()
}
.tabViewStyle(.verticalPage)
```

For page indicators:
```swift
TabView {
    Page1()
    Page2()
    Page3()
}
.tabViewStyle(.verticalPage(transitionStyle: .blur))
```

## Sheets
```swift
// Item-driven (preferred for existing items)
@State private var editingItem: Item?

.sheet(item: $editingItem) { item in
    EditItemView(item: item)
}

// Boolean-driven (for creation)
@State private var showAdd = false

.sheet(isPresented: $showAdd) {
    AddItemView()
}
```

Sheets on watchOS are full-screen — they slide up from the bottom.

## Alerts
```swift
.alert("Delete?", isPresented: $showAlert) {
    Button("Delete", role: .destructive) { deleteItem() }
    Button("Cancel", role: .cancel) { }
} message: {
    Text("This cannot be undone.")
}
```

## Confirmation Dialog
```swift
.confirmationDialog("Options", isPresented: $showDialog) {
    Button("Edit") { edit() }
    Button("Delete", role: .destructive) { delete() }
    Button("Cancel", role: .cancel) { }
}
```

## Type-Safe Routing
```swift
enum Route: Hashable {
    case detail(Item)
    case settings
}

NavigationStack {
    List {
        NavigationLink("Settings", value: Route.settings)
    }
    .navigationDestination(for: Route.self) { route in
        switch route {
        case .detail(let item):
            ItemDetailView(item: item)
        case .settings:
            SettingsView()
        }
    }
}
```

## NOT Available on watchOS
- No `NavigationSplitView` — watch is single-column only
- No horizontal `TabView` tabs — use `.tabViewStyle(.verticalPage)`
- No `Tab("Label", systemImage:)` API — use plain views inside TabView
- No `.popover` — use `.sheet` or `.alert` instead
- No `.presentationDetents` — sheets are always full-screen on watch
- No sidebar navigation

## Navigation Rules
1. ALWAYS use `NavigationStack` for hierarchical flows — never `NavigationSplitView`
2. Use `.tabViewStyle(.verticalPage)` for peer sections — never horizontal tabs
3. Keep navigation depth shallow (2-3 levels max) — users glance briefly
4. Prefer `List` as the root of `NavigationStack` for consistent watch styling
5. Use `.navigationTitle` for context — watch shows it as a small header
