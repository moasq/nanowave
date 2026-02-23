---
name: "layout"
description: "Layout patterns: VStack/HStack/ZStack composition, view structure, subview extraction, GeometryReader alternatives, safe area handling. Use when arranging views, building screen layouts, or structuring view hierarchies. Triggers: VStack, HStack, ZStack, LazyVStack, Grid, Spacer, padding, frame, GeometryReader."
---
# SwiftUI Layout & View Structure Reference

Comprehensive guide to stack layouts, view composition, subview extraction, and layout best practices.

## Relative Layout Over Constants

```swift
// Good - relative to actual layout
GeometryReader { geometry in
    VStack {
        HeaderView()
            .frame(height: geometry.size.height * 0.2)
        ContentView()
    }
}

// Avoid - magic numbers that don't adapt
VStack {
    HeaderView()
        .frame(height: 150)  // Doesn't adapt to different screens
    ContentView()
}
```

---

## Context-Agnostic Views

Views should work in any context. Never assume presentation style or screen size.

```swift
// Good - adapts to given space
struct ProfileCard: View {
    let user: User

    var body: some View {
        VStack {
            Image(user.avatar)
                .resizable()
                .aspectRatio(contentMode: .fit)
            Text(user.name)
            Spacer()
        }
        .padding()
    }
}

// Avoid - assumes full screen
Image(user.avatar)
    .frame(width: UIScreen.main.bounds.width)  // Wrong!
```

---

## Own Your Container

Custom views should own static containers but not lazy/repeatable ones.

```swift
// Good - owns static container
struct HeaderView: View {
    var body: some View {
        HStack {
            Image(systemName: "star")
            Text("Title")
            Spacer()
        }
    }
}
```

---

## View Structure Principles

SwiftUI's diffing algorithm compares view hierarchies to determine what needs updating.

### Prefer Modifiers Over Conditional Views

```swift
// Good - same view, different states
SomeView()
    .opacity(isVisible ? 1 : 0)

// Avoid - creates/destroys view identity
if isVisible {
    SomeView()
}
```

Use conditionals when you truly have **different views**:

```swift
// Correct - fundamentally different views
if isLoggedIn {
    DashboardView()
} else {
    LoginView()
}
```

---

## Extract Subviews, Not Computed Properties

### The Problem with @ViewBuilder Functions

```swift
// BAD - re-executes complexSection() on every tap
struct ParentView: View {
    @State private var count = 0

    var body: some View {
        VStack {
            Button("Tap: \(count)") { count += 1 }
            complexSection()  // Re-executes every tap!
        }
    }

    @ViewBuilder
    func complexSection() -> some View {
        ForEach(0..<100) { i in
            HStack {
                Image(systemName: "star")
                Text("Item \(i)")
            }
        }
    }
}
```

### The Solution: Separate Structs

```swift
// GOOD - ComplexSection body SKIPPED when its inputs don't change
struct ParentView: View {
    @State private var count = 0

    var body: some View {
        VStack {
            Button("Tap: \(count)") { count += 1 }
            ComplexSection()  // Body skipped during re-evaluation
        }
    }
}

struct ComplexSection: View {
    var body: some View {
        ForEach(0..<100) { i in
            HStack {
                Image(systemName: "star")
                Text("Item \(i)")
            }
        }
    }
}
```

---

## Container View Pattern

```swift
// BAD - closure prevents SwiftUI from skipping updates
struct MyContainer<Content: View>: View {
    let content: () -> Content
    var body: some View {
        VStack { Text("Header"); content() }
    }
}

// GOOD - view can be compared
struct MyContainer<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        VStack { Text("Header"); content }
    }
}
```

---

## ZStack vs overlay/background

Use `ZStack` to **compose multiple peer views** that should be layered together.

Prefer `overlay` / `background` when **decorating a primary view**.

```swift
// GOOD - decoration in overlay
Button("Continue") { }
.overlay(alignment: .trailing) {
    Image(systemName: "lock.fill")
        .padding(.trailing, 8)
}

// GOOD - background shape takes parent size
HStack(spacing: 12) {
    Image(systemName: "tray")
    Text("Inbox")
}
.background {
    Capsule()
        .strokeBorder(.blue, lineWidth: 2)
}
```

---

## Layout Performance

### Avoid Layout Thrash

```swift
// Bad - deep nesting, excessive layout passes
VStack { HStack { VStack { HStack { Text("Deep") } } } }

// Good - flatter hierarchy
VStack { Text("Shallow"); Text("Structure") }
```

### Minimize GeometryReader (use iOS 17+ alternatives)

```swift
// Good - single geometry reader or containerRelativeFrame
containerRelativeFrame(.horizontal) { width, _ in
    width * 0.8
}
```

### Gate Frequent Geometry Updates

```swift
// Good - gate by threshold
.onPreferenceChange(ViewSizeKey.self) { size in
    let difference = abs(size.width - currentSize.width)
    if difference > 10 { currentSize = size }
}
```

---

## View Logic and Testability

```swift
// Good - logic in testable model (iOS 17+)
@Observable
@MainActor
final class LoginViewModel {
    var email = ""
    var password = ""
    var isValid: Bool {
        !email.isEmpty && password.count >= 8
    }

    func login() async throws { }
}

struct LoginView: View {
    @State private var viewModel = LoginViewModel()

    var body: some View {
        Form {
            TextField("Email", text: $viewModel.email)
            SecureField("Password", text: $viewModel.password)
            Button("Login") {
                Task { try? await viewModel.login() }
            }
            .disabled(!viewModel.isValid)
        }
    }
}
```

---

## Action Handlers

```swift
// Good - action references method
struct PublishView: View {
    @State private var viewModel = PublishViewModel()

    var body: some View {
        Button("Publish Project", action: viewModel.handlePublish)
    }
}
```

---

## iPad-Specific Patterns

### Size Classes

Use `@Environment(\.horizontalSizeClass)` for layout decisions — never device checks:
```swift
@Environment(\.horizontalSizeClass) private var horizontalSizeClass
```

| Context | horizontalSizeClass |
|---|---|
| iPad full-screen (any orientation) | `.regular` |
| iPad Split View (narrow) | `.compact` |
| iPad Split View (wide) | `.regular` |

**Critical:** iPad in Split View can report `.compact` — always use size classes, never `UIDevice.current`.

### Adaptive Layout Switching

```swift
@Environment(\.horizontalSizeClass) private var sizeClass

var body: some View {
    let layout = sizeClass == .compact
        ? AnyLayout(VStackLayout(spacing: 16))
        : AnyLayout(HStackLayout(spacing: 24))
    layout {
        ContentBlockA()
        ContentBlockB()
    }
}
```

### Adaptive Grids

Always use `GridItem(.adaptive(minimum:maximum:))` — automatically adjusts columns:
```swift
LazyVGrid(
    columns: [GridItem(.adaptive(minimum: 160, maximum: 320))],
    spacing: 16
) {
    ForEach(items) { item in CardView(item: item) }
}
.padding()
```

### Readability on Wide Screens

Constrain text content width on iPad to maintain readability:
```swift
ScrollView {
    content
        .frame(maxWidth: 700)
        .frame(maxWidth: .infinity)
}
```

### Form Layout

Forms should not stretch full-width on iPad:
```swift
@Environment(\.horizontalSizeClass) private var sizeClass

Form {
    Section("General") { /* ... */ }
}
.formStyle(.grouped)
.frame(maxWidth: sizeClass == .regular ? 600 : .infinity)
```

### ViewThatFits (component-level)

Use when a component should pick the best layout for available space:
```swift
ViewThatFits {
    HStack(spacing: 16) { icon; title; subtitle; actionButton }
    VStack(alignment: .leading, spacing: 8) {
        HStack { icon; title }; subtitle; actionButton
    }
}
```

### Relative Sizing

Prefer `containerRelativeFrame` over `GeometryReader`:
```swift
Image("photo")
    .containerRelativeFrame(.horizontal, count: 3, span: 1, spacing: 16)
```

### iPad Layout Rules
1. NEVER use `UIDevice.current`, `UIScreen.main.bounds`, or `#if targetEnvironment` for layout
2. NEVER hardcode frame widths or column counts
3. ALWAYS use `GridItem(.adaptive(minimum:))` for grids
4. ALWAYS constrain text to ~700pt max width on wide screens
5. ALWAYS use `.leading/.trailing` — never `.left/.right`
6. Use `@ScaledMetric` for custom dimensions that respect Dynamic Type
