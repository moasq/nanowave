---
name: "siri-intents"
description: "Siri and App Intents: AppIntents framework, App Shortcuts, voice commands, in-process intents, Shortcuts app integration. Use when adding Siri voice commands, creating App Shortcuts, or integrating with the Shortcuts app. Triggers: AppIntent, AppShortcutsProvider, Siri, voice command, shortcut."
---
# Siri Intents

SIRI VOICE COMMANDS (App Intents):
FRAMEWORK: import AppIntents (iOS 16+, replaces legacy SiriKit Intents)

BASIC INTENT:
struct OpenNoteIntent: AppIntent {
    static var title: LocalizedStringResource = "Open Note"
    static var description = IntentDescription("Opens a specific note in the app")

    @Parameter(title: "Note Name")
    var noteName: String

    func perform() async throws -> some IntentResult & ProvidesDialog {
        // Navigate to note or perform action
        return .result(dialog: "Opening \(noteName)")
    }
}

APP SHORTCUTS (makes intent discoverable via Siri without user setup):
struct MyAppShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: OpenNoteIntent(),
            phrases: ["Open \(\.$noteName) in \(.applicationName)", "Show note \(\.$noteName)"],
            shortTitle: "Open Note",
            systemImageName: "note.text"
        )
    }
}

ENTITLEMENT: Add com.apple.developer.siri entitlement (CONFIG_CHANGES entitlements key).
NO SEPARATE TARGET: App Intents run in-process — no extension target needed.
DONATION: AppShortcutsProvider automatically donates shortcuts to Siri.
