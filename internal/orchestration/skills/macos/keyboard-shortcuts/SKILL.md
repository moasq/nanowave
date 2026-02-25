---
name: "keyboard-shortcuts"
description: "macOS keyboard shortcut patterns: keyboardShortcut modifier, CommandMenu, CommandGroup, menu bar customization. Use when implementing macOS keyboard shortcuts, menu bar commands, or hotkeys. Triggers: keyboard, shortcut, CommandMenu, CommandGroup, menu bar, hotkey."
---
# Keyboard Shortcuts (macOS)

Every primary action in a macOS app MUST have a keyboard shortcut.

## Button Shortcuts
```swift
Button("Save") { save() }
    .keyboardShortcut("s", modifiers: .command)

Button("New") { createNew() }
    .keyboardShortcut("n", modifiers: .command)

Button("Delete") { deleteSelected() }
    .keyboardShortcut(.delete, modifiers: .command)

Button("Find") { showSearch() }
    .keyboardShortcut("f", modifiers: .command)
```

## Menu Bar — CommandMenu (custom menu)
Add custom menus to the menu bar via `.commands { }` on a Scene:
```swift
WindowGroup {
    ContentView()
}
.commands {
    CommandMenu("Items") {
        Button("New Item") { createItem() }
            .keyboardShortcut("n", modifiers: .command)
        Button("Duplicate") { duplicateItem() }
            .keyboardShortcut("d", modifiers: .command)
        Divider()
        Button("Delete") { deleteItem() }
            .keyboardShortcut(.delete, modifiers: .command)
    }
}
```

## Menu Bar — CommandGroup (extend system menus)
```swift
.commands {
    CommandGroup(after: .newItem) {
        Button("New from Template") { newFromTemplate() }
            .keyboardShortcut("t", modifiers: [.command, .shift])
    }
}
```

## Common Shortcuts Reference
| Action | Shortcut |
|--------|----------|
| New | Cmd+N |
| Save | Cmd+S |
| Delete | Cmd+Delete |
| Find | Cmd+F |
| Select All | Cmd+A |
| Undo | Cmd+Z |
| Preferences | Cmd+, |

## Rules
1. Every primary action MUST have `.keyboardShortcut()`
2. Use `CommandMenu` for app-specific menus, `CommandGroup` to extend system menus
3. `CommandMenu` and `CommandGroup` go inside `.commands { }` modifier on a Scene — NEVER directly in `@SceneBuilder body`
4. Follow Apple's standard shortcut conventions (Cmd+S for save, Cmd+N for new, etc.)
5. Use modifier combinations for secondary actions (Cmd+Shift+N, Cmd+Option+D)
