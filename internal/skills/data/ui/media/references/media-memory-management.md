---
name: "media-memory-management"
description: "Memory management patterns for AVPlayer, observers, and media resources — prevents leaks and crashes."
---
# Media Memory Management

Media players are the most common source of memory leaks in SwiftUI apps. Every player, observer, and notification registration **must** have a matching cleanup.

## Player Lifecycle Rule

Create in `onAppear`, destroy in `onDisappear`:

```swift
@Observable
@MainActor
final class VideoPlayerManager {
    private(set) var player: AVPlayer?
    private var timeObserver: Any?

    func setup(url: URL) {
        let item = AVPlayerItem(url: url)
        player = AVPlayer(playerItem: item)
        player?.play()
    }

    func teardown() {
        player?.pause()
        if let observer = timeObserver {
            player?.removeTimeObserver(observer)
            timeObserver = nil
        }
        player?.replaceCurrentItem(with: nil)
        player = nil
    }
}
```

In the view:
```swift
.onAppear { manager.setup(url: videoURL) }
.onDisappear { manager.teardown() }
```

**Critical:** Always call `replaceCurrentItem(with: nil)` before nilling the player. This releases the `AVPlayerItem`'s buffers immediately.

## Time Observer Cleanup

`addPeriodicTimeObserver` returns an opaque token — you **must** remove it:

```swift
// Setup
timeObserver = player?.addPeriodicTimeObserver(
    forInterval: CMTime(seconds: 0.5, preferredTimescale: 600),
    queue: .main
) { [weak self] time in
    Task { @MainActor in
        self?.currentTime = time.seconds
    }
}

// Teardown — MUST happen before player is released
if let observer = timeObserver {
    player?.removeTimeObserver(observer)
    timeObserver = nil
}
```

## KVO Observation

Use `NSKeyValueObservation` for player status/rate changes — it auto-invalidates on dealloc, but explicit cleanup is safer:

```swift
private var statusObservation: NSKeyValueObservation?
private var rateObservation: NSKeyValueObservation?

func observePlayer() {
    statusObservation = player?.currentItem?.observe(\.status, options: [.new]) { item, _ in
        // Handle status change
    }
    rateObservation = player?.observe(\.rate, options: [.new]) { player, _ in
        // Handle play/pause
    }
}

func invalidateObservations() {
    statusObservation?.invalidate()
    statusObservation = nil
    rateObservation?.invalidate()
    rateObservation = nil
}
```

## NotificationCenter Cleanup

Always use the task-based pattern for notification observers:

```swift
// In an @Observable class
private var notificationTask: Task<Void, Never>?

func startObserving() {
    notificationTask = Task { [weak self] in
        for await _ in NotificationCenter.default.notifications(
            named: AVPlayerItem.didPlayToEndTimeNotification
        ) {
            guard let self else { return }
            await MainActor.run {
                self.handlePlaybackEnd()
            }
        }
    }
}

func stopObserving() {
    notificationTask?.cancel()
    notificationTask = nil
}
```

## Multiple Players in Feed Views

**Rule: Only one player active at a time.** This prevents audio overlap and excessive memory use.

```swift
@Observable
@MainActor
final class FeedPlayerCoordinator {
    private(set) var activePlayerID: String?
    private(set) var activePlayer: AVPlayer?
    private var timeObserver: Any?

    func activate(id: String, url: URL) {
        // Tear down previous player
        deactivate()

        activePlayerID = id
        activePlayer = AVPlayer(url: url)
        activePlayer?.play()
    }

    func deactivate() {
        activePlayer?.pause()
        if let observer = timeObserver {
            activePlayer?.removeTimeObserver(observer)
            timeObserver = nil
        }
        activePlayer?.replaceCurrentItem(with: nil)
        activePlayer = nil
        activePlayerID = nil
    }
}
```

## Complete Teardown Pattern

For any `@Observable` class managing media:

```swift
func teardown() {
    // 1. Pause playback
    player?.pause()

    // 2. Remove time observers
    if let observer = timeObserver {
        player?.removeTimeObserver(observer)
        timeObserver = nil
    }

    // 3. Invalidate KVO observations
    statusObservation?.invalidate()
    statusObservation = nil

    // 4. Cancel notification tasks
    notificationTask?.cancel()
    notificationTask = nil

    // 5. Release player item buffers
    player?.replaceCurrentItem(with: nil)

    // 6. Release player
    player = nil
}
```

## Debug Checklist

Before considering a media feature complete, verify:

1. Navigate away from the screen — does audio stop?
2. Return to the screen — does playback resume correctly?
3. Open 10+ items in a list — does memory stay bounded?
4. Background the app — does the player respect audio session category?
5. Remove headphones — does playback pause (if route change handling is implemented)?
6. Check Instruments for leaked `AVPlayer` / `AVPlayerItem` instances after navigating away
