---
name: "gestures"
description: "watchOS gesture patterns: Digital Crown, long press, swipe-to-dismiss, tap, watch-appropriate interactions. Use when implementing watchOS-specific patterns related to gestures."
---
# Gestures (watchOS)

STANDARD GESTURE TABLE (watchOS):

| Gesture | SwiftUI API | Use Case |
|---------|-------------|----------|
| Tap | `Button()` | Primary actions |
| Long press | `.onLongPressGesture` | Secondary actions |
| Digital Crown | `.digitalCrownRotation()` | Value adjustment, scrolling |
| Swipe left/right | System back gesture | Navigation back |
| Scroll | `ScrollView` / `List` | Vertical content scrolling |

BUTTON VS ONTAPGESTURE:
- ALWAYS use `Button()` for tappable elements — same rule as iOS
- Button provides accessibility, highlight states, and VoiceOver support on watchOS

DIGITAL CROWN:
```swift
@State private var crownValue: Double = 0

ScrollView {
    content
}
.digitalCrownRotation(
    $crownValue,
    from: 0,
    through: 100,
    by: 1,
    sensitivity: .medium,
    isContinuous: false,
    isHapticFeedbackEnabled: true
)
```

DIGITAL CROWN FOR VALUE SELECTION:
```swift
@State private var selectedIndex: Double = 0

VStack {
    Text("Temperature")
    Text("\(Int(selectedIndex + 60))\u{00B0}F")
        .font(.title2)
}
.digitalCrownRotation(
    $selectedIndex,
    from: 0,
    through: 40,
    by: 1,
    sensitivity: .low,
    isHapticFeedbackEnabled: true
)
```

LONG PRESS:
```swift
Button("Action") { primaryAction() }
    .onLongPressGesture {
        secondaryAction()
    }
```

NOT AVAILABLE ON watchOS:
- No pinch/MagnifyGesture (no multitouch on watch screen)
- No RotateGesture
- No .contextMenu with preview (use simple .contextMenu or long press)
- No 3D Touch / Force Touch (deprecated)
- No drag-and-drop
- No .swipeActions on List rows (use Button with .onDelete for delete)

LIST DELETE:
```swift
List {
    ForEach(items) { item in
        ItemRow(item: item)
    }
    .onDelete { indexSet in
        items.remove(atOffsets: indexSet)
    }
}
```

CONFIRMATION FOR DESTRUCTIVE ACTIONS:
```swift
.confirmationDialog("Delete?", isPresented: $showConfirm) {
    Button("Delete", role: .destructive) { delete() }
    Button("Cancel", role: .cancel) { }
}
```

RULES:
- Digital Crown is the primary input method for value adjustment — always prefer it over sliders
- Enable haptic feedback on Crown rotation for tactile detents
- Keep tap targets large (minimum 44pt, prefer filling available width)
- Avoid gesture-heavy interactions — watch interactions should be brief (1-2 seconds)
