---
name: "components"
description: "tvOS UI components: card buttons, focus states, media tiles, text entry, progress indicators. Use when working on tvOS component patterns, focus styling, or card-based UI. Triggers: Button, card, focus, CardButtonStyle, component, tile."
---
# Component Patterns (tvOS)

## CRITICAL — Dark-First, Muted Colors, Content Is King

tvOS defaults to dark appearance. Design for a 10-foot viewing distance on large screens.

**Color rules for tvOS:**
- **Avoid heavily saturated colors** — saturation looks overwhelming on large TV screens
- Use **muted, desaturated** palette colors that complement content without competing
- Color should enhance content, never draw attention away from it
- AppTheme palette for tvOS should use lower saturation versions of brand colors
- Focus is indicated through **scale, elevation, and shadow** — not color changes
- Use system background colors (`.background` / `.secondarySystemBackground`) when possible

**"Show, don't tell":**
- Minimize text, maximize imagery and animation
- Cards should be image-heavy with minimal text labels
- Content thumbnails and artwork are the primary visual element

## Button Styles

CARD BUTTON (primary tvOS pattern):
```swift
Button {
    select(item)
} label: {
    VStack(alignment: .leading, spacing: AppTheme.Spacing.sm) {
        Image(item.thumbnail)
            .resizable()
            .aspectRatio(16/9, contentMode: .fill)
            .frame(width: 300, height: 170)
            .clipped()
            .clipShape(.rect(cornerRadius: AppTheme.Style.cornerRadius))

        Text(item.title)
            .font(AppTheme.Fonts.callout)
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
tvOS highlights focused elements with a lift/shadow effect. Focus is the primary interaction model:

```swift
struct FocusableCard: View {
    @Environment(\.isFocused) var isFocused

    var body: some View {
        VStack {
            Image(item.image)
                .resizable()
                .aspectRatio(16/9, contentMode: .fill)
            Text(item.title)
                .font(AppTheme.Fonts.callout)
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

## Spacing — Generous for 10-Foot Distance
tvOS requires generous spacing since users sit far from the screen:

| Element | Recommended |
|---------|-------------|
| Between sections | 60pt |
| Between cards | 40pt |
| Horizontal content padding | 80pt |
| Minimum focusable element | 150x100pt |

## Text Entry
tvOS uses a system keyboard — text input is limited:
```swift
@State private var searchText = ""

TextField("Search", text: $searchText)
    .textFieldStyle(.plain)
```

Prefer selection over free-text input:
```swift
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
2. **Use muted, desaturated colors** — saturation is overwhelming on large screens
3. Focus is shown through **scale and shadow, not color** — do not color-code focus state
4. Content imagery is the primary visual element — minimize text, maximize images
5. All interactive elements MUST be focusable and show clear focus indication
6. Prefer Picker/Toggle over TextField — keyboard input is cumbersome on TV
7. Cards should use 16:9 aspect ratio for media thumbnails
8. ONE `.borderedProminent` per visible screen area
9. Use `@Environment(\.isFocused)` for custom focus effects
10. Minimum element size: 150x100pt for comfortable remote navigation
