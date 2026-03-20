---
name: "video-playback"
description: "Video playback patterns: SwiftUI VideoPlayer, HLS streaming, custom controls, and feed-style video."
---
# Video Playback

## Basic VideoPlayer (SwiftUI)

The simplest approach — use for single video screens:

```swift
import AVKit
import SwiftUI

struct VideoView: View {
    @State private var player: AVPlayer?
    let url: URL

    var body: some View {
        VideoPlayer(player: player)
            .onAppear {
                player = AVPlayer(url: url)
                player?.play()
            }
            .onDisappear {
                player?.pause()
                player?.replaceCurrentItem(with: nil)
                player = nil
            }
    }
}
```

**Always** clean up in `onDisappear`. See media-memory-management reference for the full teardown pattern.

## HLS Streaming

AVPlayer handles `.m3u8` URLs natively — no extra setup:

```swift
// HLS works exactly like MP4 — just pass the URL
let hlsURL = URL(string: "https://devstreaming-cdn.apple.com/videos/streaming/examples/bipbop_4x3/bipbop_4x3_variant.m3u8")!
let player = AVPlayer(url: hlsURL)
```

### Observing Stream Status

```swift
@Observable
@MainActor
final class StreamPlayerManager {
    private(set) var player: AVPlayer?
    private(set) var isBuffering = false
    private(set) var error: String?
    private var statusObservation: NSKeyValueObservation?
    private var bufferObservation: NSKeyValueObservation?
    private var timeObserver: Any?

    func load(url: URL) {
        let item = AVPlayerItem(url: url)
        player = AVPlayer(playerItem: item)

        statusObservation = item.observe(\.status, options: [.new]) { [weak self] item, _ in
            Task { @MainActor in
                switch item.status {
                case .readyToPlay:
                    self?.player?.play()
                case .failed:
                    self?.error = item.error?.localizedDescription ?? "Playback failed"
                default:
                    break
                }
            }
        }

        bufferObservation = item.observe(\.isPlaybackBufferEmpty, options: [.new]) { [weak self] item, _ in
            Task { @MainActor in
                self?.isBuffering = item.isPlaybackBufferEmpty
            }
        }
    }

    func teardown() {
        player?.pause()
        if let observer = timeObserver {
            player?.removeTimeObserver(observer)
            timeObserver = nil
        }
        statusObservation?.invalidate()
        statusObservation = nil
        bufferObservation?.invalidate()
        bufferObservation = nil
        player?.replaceCurrentItem(with: nil)
        player = nil
    }
}
```

## Local Video from Bundle

```swift
if let url = Bundle.main.url(forResource: "intro", withExtension: "mp4") {
    player = AVPlayer(url: url)
}
```

## Custom Transport Controls

When you need custom UI over the video (not the default VideoPlayer controls):

```swift
struct CustomVideoPlayer: View {
    @State private var manager = VideoControlsManager()
    let url: URL

    var body: some View {
        ZStack {
            VideoPlayer(player: manager.player)
                .disabled(true) // Disable default controls

            // Custom overlay
            VStack {
                Spacer()
                HStack(spacing: AppTheme.Spacing.lg) {
                    Button(action: { manager.skipBackward(seconds: 10) }) {
                        Image(systemName: "gobackward.10")
                            .font(.title)
                    }

                    Button(action: { manager.togglePlayPause() }) {
                        Image(systemName: manager.isPlaying ? "pause.circle.fill" : "play.circle.fill")
                            .font(.system(size: 50))
                    }

                    Button(action: { manager.skipForward(seconds: 10) }) {
                        Image(systemName: "goforward.10")
                            .font(.title)
                    }
                }
                .foregroundStyle(.white)

                // Progress bar
                Slider(
                    value: Binding(
                        get: { manager.progress },
                        set: { manager.seek(to: $0) }
                    ),
                    in: 0...1
                )
                .padding(.horizontal, AppTheme.Spacing.md)

                Text(manager.timeString)
                    .font(AppTheme.Typography.caption)
                    .foregroundStyle(.white)
            }
            .padding(AppTheme.Spacing.md)
        }
        .onAppear { manager.setup(url: url) }
        .onDisappear { manager.teardown() }
    }
}
```

### Controls Manager

```swift
@Observable
@MainActor
final class VideoControlsManager {
    private(set) var player: AVPlayer?
    private(set) var isPlaying = false
    private(set) var progress: Double = 0
    private(set) var currentTime: Double = 0
    private(set) var duration: Double = 0
    private var timeObserver: Any?
    private var statusObservation: NSKeyValueObservation?

    var timeString: String {
        let current = formatTime(currentTime)
        let total = formatTime(duration)
        return "\(current) / \(total)"
    }

    func setup(url: URL) {
        let item = AVPlayerItem(url: url)
        player = AVPlayer(playerItem: item)

        statusObservation = item.observe(\.status, options: [.new]) { [weak self] item, _ in
            Task { @MainActor in
                if item.status == .readyToPlay {
                    self?.duration = item.duration.seconds
                    self?.player?.play()
                    self?.isPlaying = true
                }
            }
        }

        timeObserver = player?.addPeriodicTimeObserver(
            forInterval: CMTime(seconds: 0.5, preferredTimescale: 600),
            queue: .main
        ) { [weak self] time in
            Task { @MainActor in
                guard let self, self.duration > 0 else { return }
                self.currentTime = time.seconds
                self.progress = time.seconds / self.duration
            }
        }
    }

    func togglePlayPause() {
        if isPlaying {
            player?.pause()
        } else {
            player?.play()
        }
        isPlaying.toggle()
    }

    func seek(to fraction: Double) {
        let time = CMTime(seconds: fraction * duration, preferredTimescale: 600)
        player?.seek(to: time)
    }

    func skipForward(seconds: Double) {
        guard let current = player?.currentTime() else { return }
        let target = CMTimeAdd(current, CMTime(seconds: seconds, preferredTimescale: 600))
        player?.seek(to: target)
    }

    func skipBackward(seconds: Double) {
        guard let current = player?.currentTime() else { return }
        let target = CMTimeSubtract(current, CMTime(seconds: seconds, preferredTimescale: 600))
        player?.seek(to: target)
    }

    func teardown() {
        player?.pause()
        if let observer = timeObserver {
            player?.removeTimeObserver(observer)
            timeObserver = nil
        }
        statusObservation?.invalidate()
        statusObservation = nil
        player?.replaceCurrentItem(with: nil)
        player = nil
    }

    private func formatTime(_ seconds: Double) -> String {
        guard seconds.isFinite, seconds >= 0 else { return "0:00" }
        let mins = Int(seconds) / 60
        let secs = Int(seconds) % 60
        return String(format: "%d:%02d", mins, secs)
    }
}
```

## Video in Scrollable Feed (TikTok-style)

**Rule: Only one player active at a time.** Use a coordinator (see media-memory-management reference).

### Tab Bar & Safe Area Rules for Video Feeds

When building a full-screen video feed inside a `TabView`:
1. **Hide the tab bar:** Apply `.toolbar(.hidden, for: .tabBar)` on the feed view — immersive video feeds should not show the tab bar.
2. **Use `.containerRelativeFrame`** (iOS 17+) instead of manual `GeometryReader` + `.frame()` sizing. `GeometryReader` reports the safe area height (excludes tab bar), but `.ignoresSafeArea()` makes the scroll area full screen — this mismatch causes visible gaps between paging cards.
3. **Overlay safe area:** Action buttons and video info overlays must account for safe area insets using `GeometryProxy.safeAreaInsets` or `.safeAreaPadding()`.
4. **Header safe area:** When `.ignoresSafeArea()` is on the scroll view, the feed header must manually add `safeAreaInsets.top` padding to clear the Dynamic Island / status bar.

```swift
struct VideoFeedView: View {
    @State private var coordinator = FeedPlayerCoordinator()
    let videos: [VideoItem]

    var body: some View {
        ScrollView {
            LazyVStack(spacing: 0) {
                ForEach(videos) { video in
                    VideoFeedCell(
                        video: video,
                        coordinator: coordinator
                    )
                    .containerRelativeFrame([.horizontal, .vertical])
                }
            }
            .scrollTargetLayout()
        }
        .scrollTargetBehavior(.paging)
        .ignoresSafeArea()
        .toolbar(.hidden, for: .tabBar)
        .onDisappear { coordinator.deactivate() }
    }
}

struct VideoFeedCell: View {
    let video: VideoItem
    let coordinator: FeedPlayerCoordinator

    var body: some View {
        ZStack {
            if coordinator.activePlayerID == video.id {
                VideoPlayer(player: coordinator.activePlayer)
            } else {
                // Thumbnail placeholder
                AsyncImage(url: video.thumbnailURL) { image in
                    image.resizable().aspectRatio(contentMode: .fill)
                } placeholder: {
                    Color(AppTheme.Colors.surfaceSecondary)
                }
            }
        }
        .onAppear {
            coordinator.activate(id: video.id, url: video.url)
        }
    }
}
```

## Looping Video

For background videos or short clips that should loop:

```swift
@Observable
@MainActor
final class LoopingPlayerManager {
    private(set) var player: AVPlayer?
    private var loopTask: Task<Void, Never>?

    func setup(url: URL) {
        player = AVPlayer(url: url)
        player?.play()

        loopTask = Task { [weak self] in
            for await _ in NotificationCenter.default.notifications(
                named: AVPlayerItem.didPlayToEndTimeNotification
            ) {
                guard let self else { return }
                await MainActor.run {
                    self.player?.seek(to: .zero)
                    self.player?.play()
                }
            }
        }
    }

    func teardown() {
        loopTask?.cancel()
        loopTask = nil
        player?.pause()
        player?.replaceCurrentItem(with: nil)
        player = nil
    }
}
```

## Platform Notes

| Platform | VideoPlayer | AVPlayer | Notes |
|----------|------------|----------|-------|
| iOS | Full support | Full support | Primary platform |
| macOS | Use `AVPlayerView` (AppKit) | Full support | No SwiftUI `VideoPlayer` — use NSViewRepresentable |
| tvOS | Full support | Full support | Integrate with Siri Remote for transport controls |
| watchOS | No video | No video | watchOS cannot play video |
| visionOS | Full support | Full support | Consider spatial placement of video surfaces |
