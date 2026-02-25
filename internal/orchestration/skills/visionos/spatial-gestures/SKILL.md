---
name: "spatial-gestures"
description: "visionOS spatial gesture patterns: eye tracking, hand pinch, drag, rotate, magnify gestures. Use when implementing visionOS input handling or spatial interactions. Triggers: gesture, tap, drag, pinch, rotate, magnify, spatial."
---
# Spatial Gestures (visionOS)

visionOS uses eye tracking + hand gestures instead of touch.

## Tap Gesture (look + pinch)
```swift
Model3D(named: "Globe")
    .gesture(TapGesture().targetedToAnyEntity().onEnded { value in
        handleTap(value.entity)
    })
```

## Drag Gesture (pinch + move)
```swift
RealityView { content in
    // entity setup
}
.gesture(DragGesture().targetedToAnyEntity().onChanged { value in
    value.entity.position = value.convert(value.location3D, from: .local, to: .scene)
})
```

## Long Press
```swift
SpatialTapGesture()
    .onEnded { value in
        handleSpatialTap(at: value.location)
    }
```

## Indirect Gestures (SwiftUI views)
Standard SwiftUI gestures work on 2D windows via eye tracking:
```swift
Button("Action") { doSomething() }
    .hoverEffect()  // REQUIRED — visual feedback for eye tracking

Toggle("Setting", isOn: $setting)
    .hoverEffect()
```

## Rules
1. ALL interactive SwiftUI elements MUST have `.hoverEffect()` for eye tracking feedback
2. Use `TapGesture().targetedToAnyEntity()` for 3D entity interaction
3. Use `DragGesture().targetedToAnyEntity()` for spatial dragging
4. Standard SwiftUI gestures work on 2D window content via eye tracking
5. No haptic feedback available — use visual and audio feedback instead
6. Minimum interactive target size: 60x60 points
