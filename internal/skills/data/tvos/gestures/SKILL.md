---
name: "gestures"
description: "tvOS gesture patterns: Siri Remote input handling with onMoveCommand, onPlayPauseCommand, onExitCommand, swipe recognition. Use when implementing tvOS input handling, remote control interactions, or focus-based gestures. Triggers: gesture, Siri Remote, onMoveCommand, swipe, remote."
---
# Gestures (tvOS)

tvOS does NOT support touch gestures. All input comes from the Siri Remote.

## Siri Remote Commands

DIRECTIONAL INPUT:
```swift
.onMoveCommand { direction in
    switch direction {
    case .up: moveUp()
    case .down: moveDown()
    case .left: moveLeft()
    case .right: moveRight()
    @unknown default: break
    }
}
```

PLAY/PAUSE BUTTON:
```swift
.onPlayPauseCommand {
    togglePlayback()
}
```

MENU/BACK BUTTON:
```swift
.onExitCommand {
    dismiss()
}
```

SELECT (CLICK) BUTTON:
- Handled automatically by Button tap actions
- Use `.onTapGesture` only for non-button focusable views

LONG PRESS:
```swift
.onLongPressGesture(minimumDuration: 0.5) {
    showContextOptions()
}
```

## Swipe Recognition (Siri Remote touchpad)
```swift
.gesture(
    DragGesture(minimumDistance: 50)
        .onEnded { value in
            if abs(value.translation.width) > abs(value.translation.height) {
                if value.translation.width > 0 {
                    swipeRight()
                } else {
                    swipeLeft()
                }
            }
        }
)
```

## NOT Available on tvOS
- No pinch gestures
- No rotation gestures
- No multi-touch
- No 3D Touch / Force Touch
- No pencil input
- No drag and drop

## Rules
1. Primary interaction is focus movement + select — not swipe
2. Use `onMoveCommand` for directional navigation, not drag gestures
3. Always handle `onExitCommand` for back navigation
4. Siri Remote swipes are secondary — focus navigation is primary
5. All interactive elements must work with focus + select pattern
