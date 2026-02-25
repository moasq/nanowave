---
name: "navigation"
description: "macOS navigation patterns: NavigationSplitView, WindowGroup, Settings scene, MenuBarExtra, multiple windows. Use when working on macOS navigation, window management, or scene architecture. Triggers: navigation, NavigationSplitView, WindowGroup, Settings, MenuBarExtra, openWindow."
---
# Navigation Patterns (macOS)

## NavigationSplitView (primary navigation)
```swift
NavigationSplitView {
    List(categories, selection: $selectedCategory) { category in
        Label(category.name, systemImage: category.icon)
    }
    .navigationSplitViewColumnWidth(min: 180, ideal: 220)
} detail: {
    if let category = selectedCategory {
        CategoryDetailView(category: category)
    } else {
        ContentUnavailableView("Select a Category", systemImage: "sidebar.left")
    }
}
.navigationSplitViewStyle(.balanced)
```

### Three-Column Layout
```swift
NavigationSplitView {
    SidebarView(selection: $selectedGroup)
        .navigationSplitViewColumnWidth(min: 180, ideal: 220)
} content: {
    ContentListView(group: selectedGroup, selection: $selectedItem)
        .navigationSplitViewColumnWidth(min: 250, ideal: 300)
} detail: {
    DetailView(item: selectedItem)
}
```

## WindowGroup (main window)
Users can open multiple instances:
```swift
@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
        .defaultSize(width: 900, height: 600)
    }
}
```

## Window (single utility window)
```swift
Window("Activity Monitor", id: "activity") {
    ActivityView()
}
.defaultSize(width: 400, height: 300)
```

## Settings Scene
Auto-wired to Cmd+, menu:
```swift
@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup { ContentView() }
        Settings { SettingsView() }
    }
}
```

## MenuBarExtra (menu bar apps)
```swift
@main
struct MyApp: App {
    var body: some Scene {
        MenuBarExtra("Status", systemImage: "circle.fill") {
            StatusMenuView()
        }
        .menuBarExtraStyle(.window)
    }
}
```

## Opening / Dismissing Windows
```swift
@Environment(\.openWindow) var openWindow
@Environment(\.dismissWindow) var dismissWindow

Button("Open Monitor") {
    openWindow(id: "activity")
}
```

## NavigationStack (within detail views)
```swift
NavigationStack {
    List(items) { item in
        NavigationLink(value: item) {
            ItemRow(item: item)
        }
    }
    .navigationTitle("Items")
    .navigationDestination(for: Item.self) { item in
        ItemDetailView(item: item)
    }
}
```

## TabView (sidebar tabs)
```swift
TabView {
    Tab("Library", systemImage: "books.vertical") {
        LibraryView()
    }
    Tab("Search", systemImage: "magnifyingglass") {
        SearchView()
    }
    Tab("Settings", systemImage: "gear") {
        SettingsView()
    }
}
.tabViewStyle(.sidebarAdaptable)
```

## CommandMenu (system menu bar)
CommandMenu closures run outside the view hierarchy — use `@FocusedValue` to wire them to view state.
```swift
@main
struct MyApp: App {
    @FocusedValue(\.activeItems) private var items

    var body: some Scene {
        WindowGroup { ContentView() }

        CommandMenu("Items") {
            Button("New Item") { items?.create() }
                .keyboardShortcut("n", modifiers: .command)
                .disabled(items == nil)
            Button("Delete Item") { items?.deleteSelected() }
                .keyboardShortcut(.delete, modifiers: .command)
                .disabled(items == nil)
        }

        CommandGroup(replacing: .newItem) {
            Button("New Document") { items?.create() }
                .keyboardShortcut("n", modifiers: .command)
                .disabled(items == nil)
        }
    }
}
// In the active view: .focusedValue(\.activeItems, viewModel) to publish state
// FocusedValues key: extension FocusedValues { @Entry var activeItems: ItemsViewModel? }
```

## Sheet Presentation
```swift
.sheet(isPresented: $showEditor) {
    EditorView()
        .frame(minWidth: 400, minHeight: 300)
}
```

## NOT Available on macOS
- No `fullScreenCover` — use `.sheet` or open a new `Window`
- No swipe-back navigation — users use toolbar back buttons or Cmd+[
- No `UINavigationController` — SwiftUI only
- No tab bar at bottom — use sidebar or top-level TabView

## Rules
1. Use `NavigationSplitView` as primary navigation (2 or 3 columns)
2. Use `WindowGroup` for main window, `Window(id:)` for utility windows
3. Use `Settings` scene for preferences (auto-wires Cmd+,)
4. Use `MenuBarExtra` for menu bar apps
5. Use `openWindow(id:)` / `dismissWindow(id:)` for window management
6. Use `.tabViewStyle(.sidebarAdaptable)` for sidebar tabs
7. Use `CommandMenu` / `CommandGroup` for system menu bar customization
8. Always provide `.presentationDetents` or frame constraints on `.sheet`
