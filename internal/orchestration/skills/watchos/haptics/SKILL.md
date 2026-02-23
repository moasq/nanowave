---
name: "haptics"
description: "watchOS haptic feedback: WKInterfaceDevice preset haptic types for wrist-based feedback. Use when implementing watchOS-specific patterns related to haptics."
---
# Haptics (watchOS)

HAPTIC FEEDBACK (watchOS):
- Use `WKInterfaceDevice.current().play(_:)` for all haptic feedback
- No UIFeedbackGenerator or CoreHaptics on watchOS

PRESET HAPTIC TYPES:
| Type | Use Case |
|------|----------|
| `.click` | General tap confirmation, button press |
| `.directionUp` | Value increasing, scrolling up |
| `.directionDown` | Value decreasing, scrolling down |
| `.success` | Task completed, action confirmed |
| `.failure` | Error, invalid input |
| `.retry` | Retry prompt, try again |
| `.start` | Activity/timer started |
| `.stop` | Activity/timer stopped |
| `.notification` | Alert, incoming notification |

USAGE:
```swift
import WatchKit

// Simple tap feedback
WKInterfaceDevice.current().play(.click)

// Outcome feedback
func completeTask() {
    // ... perform action
    WKInterfaceDevice.current().play(.success)
}

// Error feedback
func handleError() {
    WKInterfaceDevice.current().play(.failure)
}

// Digital Crown value change
.digitalCrownRotation($value)
.onChange(of: value) {
    WKInterfaceDevice.current().play(.click)
}
```

RULES:
- Always use the semantic preset that matches the interaction context
- Pair `.click` with Digital Crown detents and button taps
- Pair `.success`/`.failure` with operation outcomes
- Pair `.start`/`.stop` with timer or workout transitions
- Keep haptic calls lightweight — avoid calling on every frame during Crown rotation
