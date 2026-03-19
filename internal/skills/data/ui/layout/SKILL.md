---
name: "layout"
description: "Layout patterns: VStack, HStack, Grid, LazyVGrid, GeometryReader, alignment. Use when implementing UI patterns related to layout."
tags: "swiftui, ui-patterns"
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
