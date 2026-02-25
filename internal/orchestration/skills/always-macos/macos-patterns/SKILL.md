---
name: "macos-patterns"
description: "macOS platform patterns: window management, menu bar, keyboard shortcuts, Settings, drag-and-drop, App Sandbox. Use when building macOS-specific features, handling desktop input, or implementing Mac app patterns. Triggers: macOS, Mac, desktop, menu bar, keyboard shortcut, Settings scene, window management."
---
# macOS Platform Patterns

## Window Management
Three scene types for Mac apps:
```swift
@main
struct MyApp: App {
    var body: some Scene {
        // Main window — users can open multiple instances
        WindowGroup {
            ContentView()
        }
        .defaultSize(width: 900, height: 600)

        // Single utility window
        Window("Inspector", id: "inspector") {
            InspectorView()
        }
        .defaultSize(width: 300, height: 400)

        // Document-based app
        DocumentGroup(newDocument: MyDocument()) { file in
            DocumentEditor(document: file.$document)
        }
    }
}
```

## Menu Bar Customization (CommandMenu + FocusedValue)
CommandMenu closures run at the App scene level — outside the view hierarchy. They CANNOT access view @State directly. Use @FocusedValue to bridge view state to menu actions.

```swift
// 1. Define a FocusedValue key using @Entry
extension FocusedValues {
    @Entry var activeDocument: DocumentViewModel?
}

// 2. ViewModel — @Observable only (NOT ObservableObject)
@Observable @MainActor
class DocumentViewModel {
    var content = ""
    func save() { /* persist */ }
    func togglePreview() { /* toggle */ }
}

// 3. Publish from active view using .focusedValue()
struct DocumentView: View {
    @State var viewModel = DocumentViewModel()
    var body: some View {
        EditorContent(viewModel: viewModel)
            .focusedValue(\.activeDocument, viewModel)  // REQUIRED — publish to menu bar
    }
}

// 4. Consume in App via @FocusedValue and wire ALL menu actions
@main
struct MyApp: App {
    @FocusedValue(\.activeDocument) private var document

    var body: some Scene {
        WindowGroup { DocumentView() }

        CommandMenu("Document") {
            Button("Save") { document?.save() }
                .keyboardShortcut("s", modifiers: .command)
                .disabled(document == nil)
            Button("Toggle Preview") { document?.togglePreview() }
                .keyboardShortcut("p", modifiers: [.command, .option])
                .disabled(document == nil)
        }

        CommandGroup(replacing: .newItem) {
            Button("New Document") { document?.createNew() }
                .keyboardShortcut("n", modifiers: .command)
                .disabled(document == nil)
        }
    }
}
```

Rules:
- ALWAYS define FocusedValues key using `@Entry` macro
- ALWAYS use `.focusedValue(\.key, value)` on the active view to publish state
- ALWAYS consume via `@FocusedValue(\.key) private var name` in the App struct
- ALWAYS `.disabled(object == nil)` on every menu item — menus are active even when no view is focused
- Using empty closures `{}` on CommandMenu buttons is unacceptable — every action must call through to the FocusedValue
- CommandMenu closures run outside the view hierarchy — they cannot call view methods directly

## Keyboard Shortcuts
Every menu item and primary action needs `.keyboardShortcut()`:
```swift
// Standard shortcuts
Button("Save") { save() }
    .keyboardShortcut("s", modifiers: .command)

Button("Undo") { undo() }
    .keyboardShortcut("z", modifiers: .command)

Button("Find") { showSearch() }
    .keyboardShortcut("f", modifiers: .command)

// Custom shortcuts
Button("Toggle Sidebar") { toggleSidebar() }
    .keyboardShortcut("s", modifiers: [.command, .control])
```

## Settings / Preferences
```swift
@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup { ContentView() }

        // Auto-creates "Settings..." in app menu (Cmd+,)
        Settings {
            SettingsView()
        }
    }
}

struct SettingsView: View {
    var body: some View {
        TabView {
            GeneralSettingsView()
                .tabItem { Label("General", systemImage: "gear") }
            AppearanceSettingsView()
                .tabItem { Label("Appearance", systemImage: "paintbrush") }
        }
        .frame(width: 450, height: 300)
    }
}
```

## Menu Bar Apps
```swift
@main
struct StatusApp: App {
    var body: some Scene {
        MenuBarExtra("Status", systemImage: "circle.fill") {
            StatusMenuView()
        }
        .menuBarExtraStyle(.window)
    }
}
```

## Drag and Drop
```swift
// Draggable item
ItemRow(item: item)
    .draggable(item)

// Drop destination
FolderView(folder: folder)
    .dropDestination(for: Item.self) { items, location in
        moveItems(items, to: folder)
        return true
    }
```

## App Sandbox
macOS apps need entitlements for system access:
- File access: `com.apple.security.files.user-selected.read-write`
- Network: `com.apple.security.network.client`
- Camera: `com.apple.security.device.camera`
- Microphone: `com.apple.security.device.audio-input`

## Platform Conditionals
Use for platform-specific code in shared modules:
```swift
#if os(macOS)
import AppKit
// macOS-specific code
#elseif os(iOS)
import UIKit
// iOS-specific code
#endif
```

## Liquid Glass (macOS 26)
```swift
// Translucent glass backgrounds
VStack { content }
    .glassEffect()

// Glass button styles
Button("Action") { }
    .buttonStyle(.glass)

Button("Primary") { }
    .buttonStyle(.glassProminent)
```

## File Handling
```swift
// Open panel
.fileImporter(
    isPresented: $showOpen,
    allowedContentTypes: [.json]
) { result in
    // handle
}

// Save panel
.fileExporter(
    isPresented: $showSave,
    document: doc,
    contentType: .json
) { result in
    // handle
}
```

## Deprecated API Alternatives
- Use `.foregroundStyle()` not `.foregroundColor()`
- Use `.clipShape(.rect(cornerRadius:))` not `.cornerRadius()`
- Use `@Observable` not `ObservableObject` — never combine both on the same class
- Touch Bar is deprecated — do not implement

## NOT Available on macOS
- No UIKit — macOS uses AppKit; SwiftUI apps should never import UIKit
- No `UIColor` / `UIImage` — use SwiftUI `Color` / `Image` instead
- No HealthKit
- No haptic feedback (CoreHaptics)
- No rear camera, LiDAR, or portrait mode (FaceTime camera only)
- No App Clips
- No Live Activities
- No Safari extensions (different extension model)

## Rules
1. Use `WindowGroup`, `Window`, or `DocumentGroup` for scene types
2. Use `CommandMenu` / `CommandGroup` for menu bar customization
3. Every primary action MUST have `.keyboardShortcut()`
4. Use `Settings { }` scene for preferences (auto-wires Cmd+,)
5. Use `MenuBarExtra` for menu bar apps with `.menuBarExtraStyle(.window)`
6. Use `.draggable()` / `.dropDestination()` for drag-and-drop
7. Use `#if os(macOS)` for platform-specific code in shared modules
8. Never import UIKit — use SwiftUI `Color`/`Image` not `NSColor`/`NSImage`
9. Use `.foregroundStyle()` not `.foregroundColor()`
10. Use `.clipShape(.rect(cornerRadius:))` not `.cornerRadius()`
