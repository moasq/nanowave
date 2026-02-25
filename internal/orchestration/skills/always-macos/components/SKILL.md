---
name: "components"
description: "macOS UI components: button styles, context menus, tables, drag-and-drop, keyboard shortcuts. Use when working on macOS component patterns, desktop UI, or Mac app interactions. Triggers: Button, Table, Menu, contextMenu, keyboardShortcut, component."
---
# Component Patterns (macOS)

## macOS Visual Design — Desktop-Native Feel

macOS apps should feel like professional desktop tools, not enlarged phone apps.

**Sidebar**: Use translucent sidebar by default (NavigationSplitView handles this). The sidebar background is slightly transparent when the window is active, becoming opaque when inactive — this signals which window has focus.

**Color density**: Similar to iOS — accent colors, branded surfaces, and colored icons are appropriate. However, in macOS Tahoe (26), sidebar icons default to monochrome (not tinted) since tinted icons clash with the translucent sidebar.

**Content areas**: Use opaque AppTheme backgrounds for content areas. Sidebars are the main translucent element.

## Button Styles
```swift
// Primary action
Button("Save") { save() }
    .buttonStyle(.borderedProminent)
    .keyboardShortcut("s", modifiers: .command)

// Secondary action
Button("Cancel") { cancel() }
    .buttonStyle(.bordered)
    .keyboardShortcut(.escape)

// Tertiary / inline
Button("Details") { showDetails() }
    .buttonStyle(.borderless)

// Link-style
Button("Learn More") { openURL() }
    .buttonStyle(.link)
```

BUTTON HIERARCHY:
| Level | Style | Use Case |
|-------|-------|----------|
| Primary action | `.borderedProminent` | Save, Confirm, Send |
| Secondary | `.bordered` | Cancel, Settings |
| Tertiary | `.borderless` | Inline actions |
| Link | `.link` | Navigation, external links |

## Liquid Glass (macOS 26)
```swift
// Translucent glass background
VStack { content }
    .glassEffect()

// Glass button styles
Button("Action") { }
    .buttonStyle(.glass)

Button("Primary") { }
    .buttonStyle(.glassProminent)
```

## Context Menus (right-click)
```swift
Text(item.name)
    .contextMenu {
        Button("Copy", systemImage: "doc.on.doc") { copy(item) }
            .keyboardShortcut("c", modifiers: .command)
        Button("Delete", systemImage: "trash", role: .destructive) { delete(item) }
        Divider()
        Menu("Move to") {
            ForEach(folders) { folder in
                Button(folder.name) { move(item, to: folder) }
            }
        }
    }
```

## Keyboard Shortcuts
Every primary action and menu item MUST have a keyboard shortcut:
```swift
Button("New Item") { createItem() }
    .keyboardShortcut("n", modifiers: .command)

Button("Delete") { deleteItem() }
    .keyboardShortcut(.delete, modifiers: .command)

Button("Find") { showSearch() }
    .keyboardShortcut("f", modifiers: .command)
```

## Drag and Drop
```swift
// Draggable source
Text(item.name)
    .draggable(item)

// Drop destination
List { ... }
    .dropDestination(for: Item.self) { items, location in
        handleDrop(items, at: location)
        return true
    }
```

## Table (macOS-specific)
```swift
Table(items) {
    TableColumn("Name", value: \.name)
    TableColumn("Date") { item in
        Text(item.date, style: .date)
    }
    TableColumn("Size") { item in
        Text(item.formattedSize)
    }
}
.tableStyle(.inset(alternatesRowBackgrounds: true))
```

## Menu (dropdown in toolbar)
```swift
Menu("Options", systemImage: "ellipsis.circle") {
    Button("Import...", systemImage: "square.and.arrow.down") { importData() }
    Button("Export...", systemImage: "square.and.arrow.up") { exportData() }
    Divider()
    Toggle("Show Hidden", isOn: $showHidden)
}
```

## Toggle
```swift
Toggle("Enable Notifications", isOn: $notificationsEnabled)
    .toggleStyle(.switch)
```

## Form (Preferences)
```swift
Form {
    Section("General") {
        TextField("Name", text: $name)
        Picker("Theme", selection: $theme) {
            Text("System").tag(Theme.system)
            Text("Light").tag(Theme.light)
            Text("Dark").tag(Theme.dark)
        }
    }
    Section("Advanced") {
        Toggle("Debug Mode", isOn: $debugMode)
            .toggleStyle(.switch)
    }
}
.formStyle(.grouped)
```

## Empty States
```swift
ContentUnavailableView(
    "No Documents",
    systemImage: "doc",
    description: Text("Create a new document to get started")
)
```

## NOT Available on macOS
- No UIKit controls — macOS uses AppKit under the hood; SwiftUI apps should never import UIKit
- No card-style focus system like tvOS
- No spatial hover effects like visionOS
- Touch Bar is deprecated — do not implement

## Rules
1. Every primary action MUST have `.keyboardShortcut()`
2. Use `.contextMenu {}` for right-click interactions on data items
3. Use `Table` for tabular data display (macOS-specific view)
4. Use `Menu` for dropdown actions in toolbars
5. Use `.formStyle(.grouped)` for preference forms
6. Use `.draggable()` / `.dropDestination()` for drag-and-drop
7. ONE `.borderedProminent` per visible area
8. Use `.foregroundStyle()` not `.foregroundColor()`
9. Use `.clipShape(.rect(cornerRadius:))` not `.cornerRadius()`
