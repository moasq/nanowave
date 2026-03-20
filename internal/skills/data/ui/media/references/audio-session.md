---
name: "audio-session"
description: "AVAudioSession configuration: categories, interruptions, route changes, and background audio setup."
---
# Audio Session Configuration

AVAudioSession tells iOS how your app interacts with the system audio. Incorrect configuration is the #1 reason audio "doesn't work" in generated apps.

## Category Selection

| Category | Use Case | Mixes with others | Silenced by mute switch |
|----------|----------|-------------------|------------------------|
| `.ambient` | Background music in games, ambient sounds | Yes | Yes |
| `.playback` | Music player, podcast, video with audio | No (default) | No |
| `.playAndRecord` | Voice chat, audio recorder | No | No |

**Default rule:** Use `.playback` unless the user specifically wants ambient sound that mixes with other apps.

## Setup Pattern

```swift
import AVFoundation

func configureAudioSession(category: AVAudioSession.Category = .playback) {
    do {
        let session = AVAudioSession.sharedInstance()
        try session.setCategory(category, mode: .default)
        try session.setActive(true)
    } catch {
        print("Audio session setup failed: \(error)")
    }
}
```

Call this early — in the `@Observable` audio service's `init()` or when first starting playback.

## Interruption Handling

Handle phone calls, Siri, alarms gracefully:

```swift
private var interruptionTask: Task<Void, Never>?

func observeInterruptions() {
    interruptionTask = Task { [weak self] in
        for await notification in NotificationCenter.default.notifications(
            named: AVAudioSession.interruptionNotification
        ) {
            guard let self else { return }
            guard let info = notification.userInfo,
                  let typeValue = info[AVAudioSessionInterruptionTypeKey] as? UInt,
                  let type = AVAudioSession.InterruptionType(rawValue: typeValue) else { return }

            await MainActor.run {
                switch type {
                case .began:
                    // System paused our audio — update UI to reflect paused state
                    self.isPlaying = false
                case .ended:
                    if let optionsValue = info[AVAudioSessionInterruptionOptionKey] as? UInt {
                        let options = AVAudioSession.InterruptionOptions(rawValue: optionsValue)
                        if options.contains(.shouldResume) {
                            self.player?.play()
                            self.isPlaying = true
                        }
                    }
                @unknown default:
                    break
                }
            }
        }
    }
}
```

## Route Change Handling

Detect headphones disconnect — standard behavior is to pause:

```swift
private var routeChangeTask: Task<Void, Never>?

func observeRouteChanges() {
    routeChangeTask = Task { [weak self] in
        for await notification in NotificationCenter.default.notifications(
            named: AVAudioSession.routeChangeNotification
        ) {
            guard let self else { return }
            guard let info = notification.userInfo,
                  let reasonValue = info[AVAudioSessionRouteChangeReasonKey] as? UInt,
                  let reason = AVAudioSession.RouteChangeReason(rawValue: reasonValue) else { return }

            await MainActor.run {
                if reason == .oldDeviceUnavailable {
                    // Headphones were unplugged — pause playback
                    self.player?.pause()
                    self.isPlaying = false
                }
            }
        }
    }
}
```

## Background Audio

For audio to continue when the app is backgrounded:

### 1. Info.plist Configuration

Add to project.yml under the target's `info` section:
```yaml
UIBackgroundModes: ["audio"]
```

### 2. Audio Session Category

Must use `.playback` (not `.ambient`):
```swift
try session.setCategory(.playback, mode: .default)
```

### 3. Keep Playing

Background audio works automatically once the above two conditions are met. No additional code needed — AVPlayer/AVAudioPlayer continues playing.

## Deactivation

When your app finishes playing, deactivate the session so other apps can resume:

```swift
func deactivateAudioSession() {
    do {
        try AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    } catch {
        print("Audio session deactivation failed: \(error)")
    }
}
```

## Platform Notes

| Platform | AVAudioSession | Background Audio | Notes |
|----------|---------------|------------------|-------|
| iOS | Full support | Yes (with UIBackgroundModes) | Primary platform |
| macOS | Not available | N/A | macOS doesn't use AVAudioSession — audio just works |
| tvOS | Full support | No | tvOS apps cannot play in background |
| watchOS | Limited | Limited | Use `WKAudioFilePlayer` instead |
| visionOS | Full support | Yes | Same as iOS |

## Cleanup

Always cancel notification tasks in teardown:

```swift
func teardown() {
    interruptionTask?.cancel()
    interruptionTask = nil
    routeChangeTask?.cancel()
    routeChangeTask = nil
    deactivateAudioSession()
}
```
