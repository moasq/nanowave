---
name: "components"
description: "watchOS UI components: watch-sized buttons, grouped lists, toggles, pickers, progress, empty states. Use when working on shared watchOS patterns related to components."
---
# Component Patterns (watchOS)

BUTTON HIERARCHY (watch-sized):

| Level | Style | Use Case |
|-------|-------|----------|
| Primary | `.borderedProminent` | Main action (Start, Save) |
| Secondary | `.bordered` | Alternative (Cancel, Edit) |
| Destructive | `.borderedProminent` + `.tint(.red)` | Delete, Remove |

- Buttons naturally fill width on watchOS — no `.frame(maxWidth: .infinity)` needed
- Use `.controlSize(.mini)` or `.controlSize(.small)` for compact button groups
- ONE `.borderedProminent` per screen
- ALWAYS use `Button()` — never `.onTapGesture` for actions

```swift
// Primary action
Button("Start") { start() }
    .buttonStyle(.borderedProminent)

// Compact button pair
HStack {
    Button("Skip") { skip() }
        .buttonStyle(.bordered)
        .controlSize(.small)
    Button("Next") { next() }
        .buttonStyle(.borderedProminent)
        .controlSize(.small)
}
```

LIST PATTERNS:
```swift
// Grouped list (primary pattern for watchOS)
List {
    Section("Today") {
        ForEach(todayItems) { item in
            NavigationLink(value: item) {
                HStack {
                    Image(systemName: item.icon)
                    Text(item.title)
                    Spacer()
                    Text(item.value)
                        .foregroundStyle(.secondary)
                }
            }
        }
    }
    Section("Settings") {
        Toggle("Notifications", isOn: $notifications)
    }
}
```

TOGGLE:
```swift
// Toggle in a list section
Section("Preferences") {
    Toggle("Sound", isOn: $soundEnabled)
    Toggle("Haptics", isOn: $hapticsEnabled)
}
```

PICKER:
```swift
// Wheel picker (good for watch)
Picker("Speed", selection: $speed) {
    Text("Slow").tag(Speed.slow)
    Text("Medium").tag(Speed.medium)
    Text("Fast").tag(Speed.fast)
}

// Navigation link picker for many options
Picker("Category", selection: $category) {
    ForEach(categories) { cat in
        Text(cat.name).tag(cat)
    }
}
```

PROGRESS:
```swift
// Circular progress (fits watch aesthetic)
ProgressView(value: progress, total: 1.0)
    .progressViewStyle(.circular)

// Linear progress
ProgressView(value: 0.6)

// Indeterminate
ProgressView("Loading...")
```

GAUGE (watch-native):
```swift
Gauge(value: heartRate, in: 40...200) {
    Text("BPM")
} currentValueLabel: {
    Text("\(Int(heartRate))")
} minimumValueLabel: {
    Text("40")
} maximumValueLabel: {
    Text("200")
}
.gaugeStyle(.accessoryCircular)
```

EMPTY STATES:
```swift
ContentUnavailableView(
    "No Workouts",
    systemImage: "figure.run",
    description: Text("Start a workout to see it here")
)
```

DATE/TIME DISPLAY:
```swift
// Use system date formatting
Text(Date.now, style: .time)
    .font(.title2)

Text(Date.now, style: .relative)
    .font(.caption)
```

NOT AVAILABLE ON watchOS:
- No card patterns with shadows (watch doesn't use card-based UI)
- No `.textFieldStyle(.roundedBorder)` — watch text input is system-driven
- No `.redacted(reason: .placeholder)` skeleton loading (too complex for watch)
- No full-width `.controlSize(.large)` buttons — watch buttons fill width by default
- No `.popover` presentations

RULES:
1. Keep components minimal — watch screen is ~40mm
2. Use `List` with sections as the primary content container
3. Gauge is the watch-native way to show metrics — prefer it over custom progress views
4. Text input on watch is limited — prefer selection (Picker, Toggle) over TextField
5. Use SF Symbols at appropriate sizes — `.font(.title3)` for icons in lists
