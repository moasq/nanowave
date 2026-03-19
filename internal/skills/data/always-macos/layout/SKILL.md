---
name: "layout"
description: "macOS layout patterns: window sizing, toolbar, sidebar columns, resizable windows, Dynamic Type. Use when working on macOS layout, window management, or toolbar customization. Triggers: layout, window, toolbar, sidebar, defaultSize, windowResizability."
---
# Layout Patterns (macOS)

## Window Default Sizing
```swift
WindowGroup {
    ContentView()
}
.defaultSize(width: 900, height: 600)
```

## Window Resizability
```swift
// Allow free resizing with a minimum content size
WindowGroup {
    ContentView()
        .frame(minWidth: 600, minHeight: 400)
}
.windowResizability(.contentMinSize)

// Fixed content size (non-resizable)
Window("Preferences", id: "prefs") {
    PreferencesView()
}
.windowResizability(.contentSize)
```

## Toolbar
```swift
NavigationSplitView {
    SidebarView()
} detail: {
    DetailView()
        .toolbar {
            ToolbarItem(placement: .principal) {
                Text("Document")
                    .font(AppTheme.Fonts.headline)
            }
            ToolbarItemGroup {
                Button("Share", systemImage: "square.and.arrow.up") { share() }
                Button("Settings", systemImage: "gear") { openSettings() }
            }
        }
}
```

## Customizable Toolbar
```swift
.toolbar(id: "editor") {
    ToolbarItem(id: "bold", placement: .automatic) {
        Button("Bold", systemImage: "bold") { toggleBold() }
    }
    ToolbarItem(id: "italic", placement: .automatic) {
        Button("Italic", systemImage: "italic") { toggleItalic() }
    }
}
.toolbarRole(.editor)
```

## Sidebar Column Width
```swift
NavigationSplitView {
    SidebarView()
        .navigationSplitViewColumnWidth(min: 180, ideal: 220, max: 300)
} content: {
    ContentList()
        .navigationSplitViewColumnWidth(min: 250, ideal: 300)
} detail: {
    DetailView()
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

## Safe Areas
macOS windows have title bar inset and traffic light buttons area:
```swift
VStack {
    content
}
.padding() // Respect default window padding
```

## File Handling (Open/Save Panels)
```swift
.fileImporter(
    isPresented: $showImporter,
    allowedContentTypes: [.json, .plainText]
) { result in
    handleImport(result)
}

.fileExporter(
    isPresented: $showExporter,
    document: myDocument,
    contentType: .json
) { result in
    handleExport(result)
}
```

## NOT Available on macOS
- No device rotation — desktop has resizable windows
- No `.edgesIgnoringSafeArea` patterns for notch/Dynamic Island
- No status bar customization
- No `UIScreen.main.bounds` — use GeometryReader or window sizing

## Rules
1. Use `.defaultSize(width:height:)` for initial window dimensions
2. Use `.windowResizability(.contentMinSize)` for resizable windows with minimums
3. Use `.toolbar {}` with placement for toolbar items
4. Use `.toolbarRole(.editor)` for user-customizable toolbars
5. Use `.navigationSplitViewColumnWidth()` to control sidebar widths
6. Design for resizable windows — never assume fixed dimensions
7. Use AppTheme spacing tokens for consistent layout
8. Use `.fileImporter()` / `.fileExporter()` for open/save panels
