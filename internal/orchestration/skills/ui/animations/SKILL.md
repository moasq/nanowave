---
name: "animations"
description: "Animation enforcement: containment, modifier order, timing, performance, and transition safety. Use when implementing UI patterns related to animations."
---
# Animations

Enforce safe, performant animations that never escape their parent bounds.

CONTAINMENT (CRITICAL):
Animated content inside a container (card, row, sheet, etc.) MUST NOT overflow its parent.
Apply containment modifiers on the PARENT that clips:
```swift
// CORRECT — compositingGroup flattens, clipped constrains
CardContainer {
    AnimatedContent()
        .transition(.scale.combined(with: .opacity))
}
.compositingGroup()
.clipped()

// WRONG — animated child overflows parent during transition
CardContainer {
    AnimatedContent()
        .transition(.move(edge: .bottom))
}
// No containment — content renders outside CardContainer
```

WHY .compositingGroup().clipped():
- .compositingGroup() flattens child layers into one compositing pass (no Metal overhead like .drawingGroup())
- .clipped() then clips that single composited layer to the parent frame
- Together they guarantee zero visual overflow during any animation phase
- Do NOT use .drawingGroup() for this — it rasterizes via Metal, wastes memory, and redraws the entire group on any change
- Do NOT rely on .scaleEffect(1) hack — it is fragile and undocumented behavior

WHEN TO APPLY CONTAINMENT:
- Any view with .transition() inside a sized container (cards, rows, sheets, popovers)
- Spring/bouncy animations on child views that may overshoot parent bounds
- Phase animations or keyframe animations that scale or offset children
- ScrollView items with animated insertion/removal

WHEN CONTAINMENT IS NOT NEEDED:
- Full-screen views with no parent clipping boundary
- Opacity-only animations (no spatial overflow possible)
- Navigation transitions handled by the system

MODIFIER ORDER:
```swift
// CORRECT — animation AFTER layout, containment on parent
VStack {
    content
        .offset(y: animating ? -20 : 0)
        .opacity(animating ? 0 : 1)
        .animation(.spring(duration: 0.4), value: animating)
}
.compositingGroup()
.clipped()

// WRONG — animation before layout modifiers
content
    .animation(.spring, value: state)
    .padding()
    .frame(maxWidth: .infinity)
```

PREFER GPU TRANSFORMS:
- Use .scaleEffect, .offset, .rotationEffect, .opacity — GPU-accelerated, no layout pass
- Avoid animating .frame, .padding, .font — triggers full layout recalculation
```swift
// GOOD — GPU transform, no layout hit
Text("Hello")
    .scaleEffect(isPressed ? 0.95 : 1.0)
    .animation(.spring(duration: 0.2), value: isPressed)

// BAD — layout-driven animation
Text("Hello")
    .padding(isPressed ? 10 : 16)
    .animation(.spring, value: isPressed)
```

TIMING CURVES:
- .spring(duration: 0.3) — default for most UI (buttons, toggles, cards)
- .spring(duration: 0.4, bounce: 0.3) — playful emphasis (success states, celebrations)
- .easeInOut(duration: 0.25) — subtle transitions (opacity, color changes)
- .bouncy — ONLY for intentional delight moments, never on frequent actions
- Keep durations under 0.5s for responsive feel

TRANSITIONS:
- Place withAnimation or .animation OUTSIDE the conditional — not inside the branch
```swift
// CORRECT
withAnimation(.spring(duration: 0.3)) {
    showDetail.toggle()
}
// In body:
if showDetail {
    DetailView()
        .transition(.opacity.combined(with: .move(edge: .bottom)))
}

// WRONG — animation inside the conditional
if showDetail {
    DetailView()
        .animation(.spring, value: showDetail) // too late
}
```

ANIMATION SCOPE:
- Bind .animation to a specific value — NEVER use .animation(.spring) without value parameter
- Use withAnimation for user-triggered state changes
- Use .animation(_:value:) for derived/computed state changes
```swift
// CORRECT — scoped to specific value
.animation(.easeInOut(duration: 0.2), value: isSelected)

// WRONG — unscoped, animates everything
.animation(.easeInOut)
```

LIST AND SCROLL ANIMATIONS:
- Use .animation on the List/ForEach container, not individual rows
- Containment is especially important for row insertion/removal animations
```swift
List {
    ForEach(items) { item in
        ItemRow(item: item)
    }
}
.animation(.spring(duration: 0.3), value: items.count)
```

PHASE AND KEYFRAME (iOS 17+):
- PhaseAnimator: use for looping multi-step sequences (loading indicators, attention pulses)
- KeyframeAnimator: use for precise multi-property choreography
- Both MUST have containment if inside a bounded parent
```swift
// Phase animation with containment
PhaseAnimator([false, true]) { phase in
    Icon()
        .scaleEffect(phase ? 1.1 : 1.0)
        .opacity(phase ? 1.0 : 0.7)
}
.compositingGroup()
.clipped()
```
