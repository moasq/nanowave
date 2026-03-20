---
name: "audio-playback"
description: "Audio playback patterns: AVAudioPlayer for local files, AVPlayer for streaming, Now Playing info, remote commands."
---
# Audio Playback

## AVAudioPlayer — Local/Bundled Audio

Use `AVAudioPlayer` for audio files bundled in the app or saved locally:

```swift
import AVFoundation

@Observable
@MainActor
final class AudioPlayerService {
    private(set) var isPlaying = false
    private(set) var currentTime: TimeInterval = 0
    private(set) var duration: TimeInterval = 0
    private(set) var currentTrack: Track?

    private var audioPlayer: AVAudioPlayer?
    private var updateTimer: Timer?

    init() {
        configureAudioSession()
    }

    func play(track: Track) {
        guard let url = Bundle.main.url(forResource: track.filename, withExtension: track.fileExtension) else { return }

        do {
            audioPlayer = try AVAudioPlayer(contentsOf: url)
            audioPlayer?.prepareToPlay()
            audioPlayer?.play()
            duration = audioPlayer?.duration ?? 0
            currentTrack = track
            isPlaying = true
            startProgressUpdates()
        } catch {
            print("Audio playback failed: \(error)")
        }
    }

    func togglePlayPause() {
        guard let player = audioPlayer else { return }
        if player.isPlaying {
            player.pause()
            isPlaying = false
            stopProgressUpdates()
        } else {
            player.play()
            isPlaying = true
            startProgressUpdates()
        }
    }

    func seek(to time: TimeInterval) {
        audioPlayer?.currentTime = time
        currentTime = time
    }

    func stop() {
        audioPlayer?.stop()
        audioPlayer = nil
        isPlaying = false
        currentTime = 0
        stopProgressUpdates()
    }

    private func startProgressUpdates() {
        stopProgressUpdates()
        updateTimer = Timer.scheduledTimer(withTimeInterval: 0.25, repeats: true) { [weak self] _ in
            Task { @MainActor in
                self?.currentTime = self?.audioPlayer?.currentTime ?? 0
            }
        }
    }

    private func stopProgressUpdates() {
        updateTimer?.invalidate()
        updateTimer = nil
    }

    private func configureAudioSession() {
        do {
            try AVAudioSession.sharedInstance().setCategory(.playback, mode: .default)
            try AVAudioSession.sharedInstance().setActive(true)
        } catch {
            print("Audio session setup failed: \(error)")
        }
    }

    func teardown() {
        stop()
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }
}
```

## AVPlayer — Streaming Audio

Use `AVPlayer` for remote audio URLs. It's the same API as video — AVPlayer handles audio-only content:

```swift
@Observable
@MainActor
final class StreamingAudioService {
    private(set) var player: AVPlayer?
    private(set) var isPlaying = false
    private(set) var isBuffering = false
    private(set) var currentTime: Double = 0
    private(set) var duration: Double = 0

    private var timeObserver: Any?
    private var statusObservation: NSKeyValueObservation?
    private var bufferObservation: NSKeyValueObservation?

    init() {
        configureAudioSession()
    }

    func load(url: URL) {
        teardown()

        let item = AVPlayerItem(url: url)
        player = AVPlayer(playerItem: item)

        statusObservation = item.observe(\.status, options: [.new]) { [weak self] item, _ in
            Task { @MainActor in
                if item.status == .readyToPlay {
                    self?.duration = item.duration.seconds.isFinite ? item.duration.seconds : 0
                    self?.player?.play()
                    self?.isPlaying = true
                }
            }
        }

        bufferObservation = item.observe(\.isPlaybackBufferEmpty, options: [.new]) { [weak self] item, _ in
            Task { @MainActor in
                self?.isBuffering = item.isPlaybackBufferEmpty
            }
        }

        timeObserver = player?.addPeriodicTimeObserver(
            forInterval: CMTime(seconds: 0.5, preferredTimescale: 600),
            queue: .main
        ) { [weak self] time in
            Task { @MainActor in
                self?.currentTime = time.seconds
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
        isPlaying = false
        currentTime = 0
        duration = 0
    }

    private func configureAudioSession() {
        do {
            try AVAudioSession.sharedInstance().setCategory(.playback, mode: .default)
            try AVAudioSession.sharedInstance().setActive(true)
        } catch {
            print("Audio session setup failed: \(error)")
        }
    }
}
```

## Now Playing Info Center

Display track info on the lock screen and Control Center:

```swift
import MediaPlayer

func updateNowPlayingInfo(title: String, artist: String, duration: TimeInterval, currentTime: TimeInterval, artwork: UIImage? = nil) {
    var info: [String: Any] = [
        MPMediaItemPropertyTitle: title,
        MPMediaItemPropertyArtist: artist,
        MPMediaItemPropertyPlaybackDuration: duration,
        MPNowPlayingInfoPropertyElapsedPlaybackTime: currentTime,
        MPNowPlayingInfoPropertyPlaybackRate: 1.0
    ]

    if let artwork {
        info[MPMediaItemPropertyArtwork] = MPMediaItemArtwork(boundsSize: artwork.size) { _ in artwork }
    }

    MPNowPlayingInfoCenter.default().nowPlayingInfo = info
}
```

Call `updateNowPlayingInfo` whenever the track changes and periodically as playback progresses.

## Remote Command Center

Handle play/pause/next/previous from headphones, lock screen, and Control Center:

```swift
import MediaPlayer

func setupRemoteCommands(
    onPlay: @escaping () -> Void,
    onPause: @escaping () -> Void,
    onNextTrack: (() -> Void)? = nil,
    onPreviousTrack: (() -> Void)? = nil
) {
    let center = MPRemoteCommandCenter.shared()

    center.playCommand.addTarget { _ in
        onPlay()
        return .success
    }

    center.pauseCommand.addTarget { _ in
        onPause()
        return .success
    }

    if let onNext = onNextTrack {
        center.nextTrackCommand.isEnabled = true
        center.nextTrackCommand.addTarget { _ in
            onNext()
            return .success
        }
    }

    if let onPrev = onPreviousTrack {
        center.previousTrackCommand.isEnabled = true
        center.previousTrackCommand.addTarget { _ in
            onPrev()
            return .success
        }
    }
}
```

**Important:** Call `setupRemoteCommands` once (e.g., in the audio service init), not on every track change.

## Audio Queue (Playlist)

For playing multiple tracks in sequence:

```swift
@Observable
@MainActor
final class PlaylistService {
    private(set) var tracks: [Track] = []
    private(set) var currentIndex: Int = 0
    private let audioService: AudioPlayerService

    init(audioService: AudioPlayerService) {
        self.audioService = audioService
    }

    func loadPlaylist(_ tracks: [Track]) {
        self.tracks = tracks
        currentIndex = 0
        if let first = tracks.first {
            audioService.play(track: first)
        }
    }

    func next() {
        guard currentIndex + 1 < tracks.count else { return }
        currentIndex += 1
        audioService.play(track: tracks[currentIndex])
    }

    func previous() {
        // If more than 3 seconds in, restart current track
        if audioService.currentTime > 3 {
            audioService.seek(to: 0)
            return
        }
        guard currentIndex > 0 else { return }
        currentIndex -= 1
        audioService.play(track: tracks[currentIndex])
    }
}
```

## Local Audio Generation

Generate sample audio files for bundled music players using macOS `say`:

```bash
say -o track1.aiff "Welcome to the music player"
afconvert track1.aiff track1.m4a -d aac -f m4af -b 128000
rm track1.aiff
```

See sample-media-urls reference for more generation options.

## Platform Notes

| Platform | AVAudioPlayer | AVPlayer (streaming) | Now Playing | Remote Commands |
|----------|--------------|---------------------|-------------|-----------------|
| iOS | Full | Full | Full | Full |
| macOS | Full | Full | Full | Full |
| tvOS | Full | Full | Full | Limited |
| watchOS | Use `WKAudioFilePlayer` | Limited | No | No |
| visionOS | Full | Full | Full | Full |
