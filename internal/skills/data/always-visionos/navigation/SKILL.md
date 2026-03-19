---
name: "navigation"
description: "visionOS navigation patterns: WindowGroup, volumes, immersive spaces, NavigationStack, TabView. Use when working on visionOS navigation, scene management, or window transitions. Triggers: navigation, WindowGroup, ImmersiveSpace, TabView, NavigationStack, openWindow."
---
# Navigation Patterns (visionOS)

## Scene Types

### Windows (default — 2D content)
```swift
@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
        .defaultSize(width: 800, height: 600)
    }
}
```

### Volumes (3D content in a bounded box)
```swift
WindowGroup(id: "model-viewer") {
    Model3DView()
}
.windowStyle(.volumetric)
.defaultSize(width: 0.5, height: 0.5, depth: 0.5, in: .meters)
```

### Immersive Spaces (full spatial experience)
```swift
ImmersiveSpace(id: "immersive") {
    ImmersiveView()
}
.immersionStyle(selection: .constant(.mixed), in: .mixed, .full)
```

## Opening Scenes
```swift
@Environment(\.openWindow) var openWindow
@Environment(\.openImmersiveSpace) var openImmersiveSpace
@Environment(\.dismissImmersiveSpace) var dismissImmersiveSpace

Button("Open Model") {
    openWindow(id: "model-viewer")
}

Button("Enter Immersive") {
    Task {
        await openImmersiveSpace(id: "immersive")
    }
}
```

## NavigationStack
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

## TabView (sidebar style on visionOS)
```swift
TabView {
    Tab("Home", systemImage: "house") {
        HomeView()
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

## Sheet Presentation
```swift
.sheet(isPresented: $showSettings) {
    SettingsView()
        .presentationDetents([.medium, .large])
}
```

## NOT Available on visionOS
- No `NavigationSplitView` sidebar — use TabView with `.tabViewStyle(.sidebarAdaptable)` instead
- No `UINavigationController` — SwiftUI only
- No split view — use multiple windows

## Rules
1. Use `WindowGroup` for 2D windows (default scene type)
2. Use `.windowStyle(.volumetric)` for 3D bounded content
3. Use `ImmersiveSpace` for full spatial experiences
4. Use `openWindow(id:)` and `openImmersiveSpace(id:)` for scene transitions
5. Use `NavigationStack` with `navigationDestination(for:)` for in-window navigation
6. Use `TabView` with `.tabViewStyle(.sidebarAdaptable)` for top-level navigation
7. Always provide `.presentationDetents` on `.sheet`
