---
name: "timers"
description: "Timer patterns: TimelineView for smooth updates, Timer.publish for periodic ticks, countdown/stopwatch formatting. Use when implementing app features related to timers."
---
# Timers

TIMERS:
- Use TimelineView(.animation) for smooth countdown/stopwatch UI
- Timer.publish(every:on:in:).autoconnect() for periodic updates
- @State private var timeRemaining: TimeInterval for countdown state
- .onReceive(timer) { _ in } to update state each tick
- Format with Duration.formatted() or custom mm:ss formatter
- Invalidate timer in .onDisappear to prevent leaks
