---
name: "watchos-patterns"
description: "watchOS platform patterns: Digital Crown, Always On Display, battery constraints, WKApplicationDelegate lifecycle, wrist detection. Use when working on shared watchOS patterns related to watchos patterns."
---
# watchOS Platform Patterns

## Digital Crown Input

The Digital Crown is the primary input for value adjustment on Apple Watch.

```swift
// Basic rotation binding
@State private var crownValue: Double = 0

ScrollView {
    content
}
.digitalCrownRotation($crownValue)

// Bounded rotation with haptics
@State private var volume: Double = 50

VolumeView(level: volume)
    .digitalCrownRotation(
        $volume,
        from: 0,
        through: 100,
        by: 1,
        sensitivity: .medium,
        isContinuous: false,
        isHapticFeedbackEnabled: true
    )
```

### Crown Sensitivity
- `.low` — large physical rotation per unit (precise adjustment)
- `.medium` — balanced (default for most use cases)
- `.high` — small physical rotation per unit (fast scrolling)

### Focus for Crown
Only the focused view receives Crown events:
```swift
@FocusState private var crownFocused: Bool

VStack {
    MetricView(value: crownValue)
}
.digitalCrownRotation($crownValue)
.focusable()
.focused($crownFocused)
.onAppear { crownFocused = true }
```

## Always On Display

watchOS apps should support Always On Display when the wrist is lowered.

```swift
@Environment(\.isLuminanceReduced) var isLuminanceReduced

var body: some View {
    VStack {
        if isLuminanceReduced {
            // Simplified, dim view — reduce updates and brightness
            Text(Date.now, style: .time)
                .font(AppTheme.Fonts.title)
        } else {
            // Full interactive view
            DetailedContentView()
        }
    }
}
```

### Always On Display Rules
- Check `\.isLuminanceReduced` to detect Always On state
- Reduce visual complexity: hide animations, secondary info, and interactive elements
- Use `TimelineView(.everyMinute)` for clock-like updates in reduced mode
- Avoid bright colors — use dimmer variants in reduced luminance
- Stop all timers and animations when `isLuminanceReduced == true`

```swift
TimelineView(.everyMinute) { context in
    if isLuminanceReduced {
        Text(context.date, style: .time)
    } else {
        LiveActivityView()
    }
}
```

## Battery & Performance Constraints

Apple Watch has very limited battery — every CPU/GPU cycle matters.

### Rules
- Avoid continuous animations (use `.animation` only for state transitions)
- Minimize network requests — batch and cache aggressively
- No background processing unless using `WKApplicationRefreshBackgroundTask`
- Prefer `TimelineView` over `Timer` for periodic updates
- Use `.task` for async work — it cancels automatically when the view disappears
- Keep view hierarchies shallow (2-3 levels)
- Images should be small and pre-sized for watch dimensions

### Background Refresh
```swift
func handle(_ backgroundTasks: Set<WKRefreshBackgroundTask>) {
    for task in backgroundTasks {
        switch task {
        case let refreshTask as WKApplicationRefreshBackgroundTask:
            // Update data
            scheduleNextRefresh()
            refreshTask.setTaskCompletedWithSnapshot(true)
        default:
            task.setTaskCompletedWithSnapshot(false)
        }
    }
}
```

## App Lifecycle

### WKApplicationDelegate (watchOS app lifecycle)
```swift
import WatchKit

class AppDelegate: NSObject, WKApplicationDelegate {
    func applicationDidFinishLaunching() {
        // App launched
    }

    func applicationDidBecomeActive() {
        // App is active and visible
    }

    func applicationWillResignActive() {
        // App is about to go inactive
    }

    func handle(_ backgroundTasks: Set<WKRefreshBackgroundTask>) {
        // Handle background tasks
    }
}

@main
struct MyWatchApp: App {
    @WKApplicationDelegateAdaptor(AppDelegate.self) var delegate

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
```

### Scene Phases
```swift
@Environment(\.scenePhase) var scenePhase

.onChange(of: scenePhase) { _, phase in
    switch phase {
    case .active:
        refreshData()
    case .inactive:
        saveState()
    case .background:
        scheduleBackgroundRefresh()
    @unknown default:
        break
    }
}
```

## Extended Runtime Sessions

For workouts or navigation that need to keep running:
```swift
import WatchKit

let session = WKExtendedRuntimeSession()
session.start()
// Session keeps app alive for the allowed duration
session.invalidate() // when done
```

## Wrist Detection Auth Pattern
```swift
// Watch is authenticated when on wrist and unlocked
// For sensitive features, confirm with LAContext
import LocalAuthentication

func requireAuth() async -> Bool {
    let context = LAContext()
    guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: nil) else {
        return false
    }
    do {
        return try await context.evaluatePolicy(
            .deviceOwnerAuthentication,
            localizedReason: "Access sensitive data"
        )
    } catch {
        return false
    }
}
```

## Platform Rules
1. Digital Crown is the primary non-touch input — always support it for value adjustment
2. Always support `isLuminanceReduced` for Always On Display
3. Battery is the #1 constraint — no continuous animations, no polling
4. Use `WKApplicationDelegate` for lifecycle hooks and background tasks
5. Keep interactions to 1-2 seconds — the watch is for glances, not sessions
6. No UIKit — watchOS is SwiftUI-only (no UIViewController, UIView, etc.)
