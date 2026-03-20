---
name: "visionos-patterns"
description: "visionOS platform patterns: scene types, RealityKit, spatial gestures, hand tracking, entity interaction. Use when building visionOS-specific features, handling spatial input, or implementing Vision Pro app patterns. Triggers: visionOS, Vision Pro, RealityKit, spatial, immersive, hand tracking, eye tracking."
---
# visionOS Platform Patterns

## Three Scene Types
1. **Windows** — 2D SwiftUI content in a glass panel (default)
2. **Volumes** — 3D content in a bounded box
3. **Immersive Spaces** — full spatial experience around the user

## RealityKit Entity Loading
```swift
import RealityKit

// In a RealityView
RealityView { content in
    if let entity = try? await Entity(named: "Scene", in: realityKitContentBundle) {
        // Add collision and input for interaction
        entity.components.set(CollisionComponent(shapes: [.generateBox(size: [0.1, 0.1, 0.1])]))
        entity.components.set(InputTargetComponent())
        content.add(entity)
    }
}
```

## Entity Interaction
Entities need `CollisionComponent` + `InputTargetComponent` for user interaction:
```swift
RealityView { content in
    let sphere = ModelEntity(mesh: .generateSphere(radius: 0.1))
    sphere.components.set(CollisionComponent(shapes: [.generateSphere(radius: 0.1)]))
    sphere.components.set(InputTargetComponent())
    content.add(sphere)
} update: { content in
    // Update entities
}
.gesture(
    TapGesture()
        .targetedToAnyEntity()
        .onEnded { value in
            // Handle tap on entity
        }
)
```

## Spatial Gestures
```swift
// Tap gesture on entities
.gesture(
    TapGesture()
        .targetedToAnyEntity()
        .onEnded { value in
            handleTap(on: value.entity)
        }
)

// Drag gesture on entities
.gesture(
    DragGesture()
        .targetedToAnyEntity()
        .onChanged { value in
            value.entity.position = value.convert(value.location3D, from: .local, to: .scene)
        }
)
```

## ARKit in Immersive Spaces
ARKit is only available in Full Space (not Shared Space):
```swift
ImmersiveSpace(id: "full-immersive") {
    RealityView { content in
        // ARKit features available here
    }
}
.immersionStyle(selection: .constant(.full), in: .full)
```

## Observable Pattern
Use `@Observable` over `ObservableObject`:
```swift
@Observable
class AppModel {
    var items: [Item] = []
    var selectedItem: Item?
    var isImmersive = false
}

// In App
@State private var model = AppModel()

WindowGroup {
    ContentView()
        .environment(model)
}
```

## Concurrency
MainActor by default in Swift 6.2. Use detached Tasks for background work:
```swift
// Already on MainActor by default
func loadData() {
    Task.detached {
        let data = await fetchData()
        await MainActor.run {
            self.items = data
        }
    }
}
```

## Hover and Focus
Eye tracking drives hover states automatically:
```swift
Button("Action") { }
    .hoverEffect() // Required for all interactive elements
    .hoverEffect(.highlight) // Highlight variant
    .hoverEffect(.lift) // Lift variant
```

## Widgets
visionOS 26 widgets are spatial — they snap to walls and tables:
```swift
// Standard WidgetKit implementation works
// visionOS renders them as spatial glass panels
```

## Deprecated API Alternatives
- Use `.foregroundStyle()` not `.foregroundColor()`
- Use `.clipShape(.rect(cornerRadius:))` not `.cornerRadius()`
- Use `@Observable` not `ObservableObject`

## NOT Available on visionOS
- No camera access (enterprise API only)
- No haptic feedback (CoreHaptics)
- No HealthKit
- No direct gaze data (privacy protected)
- No `UIScreen.main`
- No MapKit with full map view
- No NFC or Bluetooth LE scanning
- No App Clips
- No share extensions
- No Safari extensions

## Rules
1. Three scene types: Windows (default), Volumes (3D bounded), Immersive Spaces (full spatial)
2. Entities need `CollisionComponent` + `InputTargetComponent` for interaction
3. ARKit only works in Full Space immersive experiences
4. Use `@Observable` over `ObservableObject`
5. ALL interactive elements MUST have `.hoverEffect()`
6. Use spatial gestures (`.targetedToAnyEntity()`) for 3D interaction
7. MainActor by default — use `Task.detached` for background work
8. Use `.foregroundStyle()` not `.foregroundColor()`
9. Use `.clipShape(.rect(cornerRadius:))` not `.cornerRadius()`
