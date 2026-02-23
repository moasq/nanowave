---
name: "components"
description: "tvOS UI components: card buttons, focus states, media tiles, text entry, progress indicators. Use when working on tvOS component patterns, focus styling, or card-based UI. Triggers: Button, card, focus, CardButtonStyle, component, tile."
---
# Component Patterns (tvOS)

## Button Styles

CARD BUTTON (primary tvOS pattern):
```swift
Button {
    select(item)
} label: {
    VStack(alignment: .leading, spacing: 12) {
        Image(item.thumbnail)
            .resizable()
            .aspectRatio(16/9, contentMode: .fill)
            .frame(width: 300, height: 170)
            .clipped()
            .cornerRadius(12)

        Text(item.title)
            .font(.callout)
            .lineLimit(2)
    }
}
.buttonStyle(.card)
```

PLAIN BUTTON:
```swift
Button("Play", systemImage: "play.fill") {
    play()
}
.buttonStyle(.borderedProminent)
```

BUTTON HIERARCHY:
| Level | Style | Use Case |
|-------|-------|----------|
| Primary card | `.buttonStyle(.card)` | Media tiles, browsable items |
| Primary action | `.borderedProminent` | Play, Subscribe, Buy |
| Secondary | `.bordered` | More Info, Add to List |
| Destructive | `.borderedProminent` + `.tint(.red)` | Delete, Remove |

## Focus States
tvOS highlights focused elements with a lift/shadow effect:

```swift
struct FocusableCard: View {
    @Environment(\.isFocused) var isFocused

    var body: some View {
        VStack {
            Image(item.image)
                .resizable()
                .aspectRatio(16/9, contentMode: .fill)
            Text(item.title)
                .font(.callout)
        }
        .scaleEffect(isFocused ? 1.05 : 1.0)
        .animation(.easeInOut(duration: 0.2), value: isFocused)
    }
}
```

For custom focus effects:
```swift
.focusable()
.onFocusChange { focused in
    withAnimation(.easeInOut(duration: 0.15)) {
        isFocused = focused
    }
}
```

## Text Entry
tvOS uses a system keyboard — text input is limited:
```swift
// Search field (system keyboard appears)
@State private var searchText = ""

TextField("Search", text: $searchText)
    .textFieldStyle(.plain)
```

Prefer selection over free-text input:
```swift
// Better: use a Picker for known choices
Picker("Category", selection: $category) {
    ForEach(categories) { cat in
        Text(cat.name).tag(cat)
    }
}
```

## Progress / Loading
```swift
// Indeterminate
ProgressView("Loading...")

// Determinate
ProgressView(value: downloadProgress, total: 1.0)

// Overlay loading
ZStack {
    ContentView()
    if isLoading {
        ProgressView()
            .scaleEffect(1.5)
    }
}
```

## Lists
```swift
List {
    Section("Settings") {
        NavigationLink("Account") {
            AccountView()
        }
        Toggle("Notifications", isOn: $notifications)
        Picker("Quality", selection: $quality) {
            Text("Auto").tag(Quality.auto)
            Text("High").tag(Quality.high)
            Text("Low").tag(Quality.low)
        }
    }
}
```

## Empty States
```swift
ContentUnavailableView(
    "No Results",
    systemImage: "magnifyingglass",
    description: Text("Try a different search term")
)
```

## NOT Available on tvOS
- No `.textFieldStyle(.roundedBorder)` — tvOS text fields are plain
- No swipe actions on list rows
- No `.contextMenu` — use focused button actions
- No `.popover`
- No drag-and-drop or reordering
- No date picker (no touch input for calendars)

## Rules
1. Use `.buttonStyle(.card)` for browsable media content
2. All interactive elements MUST be focusable and show clear focus indication
3. Prefer Picker/Toggle over TextField — keyboard input is cumbersome on TV
4. Cards should use 16:9 aspect ratio for media thumbnails
5. ONE `.borderedProminent` per visible screen area
6. Use `@Environment(\.isFocused)` for custom focus effects
7. Minimum element size: 150x100pt for comfortable remote navigation
