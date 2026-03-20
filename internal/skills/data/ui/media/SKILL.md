---
name: "media"
description: "Comprehensive media patterns: video playback, audio/music players, HLS streaming, remote images, AVAudioSession, memory management. Use when implementing any media-related features."
tags: "avplayer, avkit, video, audio, music, streaming, hls, asyncimage, kingfisher, media"
---
# Media

## Networking Clarification

Media playback URLs are **permitted**. The forbidden-patterns networking ban targets REST API clients (URLSession data tasks, custom HTTP clients), **not** media playback. The following are allowed:

- `AVPlayer` with remote URLs (MP4, HLS `.m3u8` streams)
- `AsyncImage` loading remote image URLs
- `KFImage` (Kingfisher) loading and caching remote images
- `URLSession.shared.download(from:)` **only** for downloading media files to local storage

Do **not** use URLSession for REST API calls, JSON fetching, or custom networking — that ban still applies.

## Decision Tree

Pick the right pattern based on what you're building:

| Building | Reference | Key API |
|----------|-----------|---------|
| Video player | video-playback | `VideoPlayer`, `AVPlayer` |
| HLS/live streaming | video-playback | `AVPlayer` + `.m3u8` URL |
| Video feed (TikTok-style) | video-playback + media-memory-management | `FeedPlayerCoordinator` |
| Music/audio player | audio-playback + audio-session | `AVAudioPlayer` (local) or `AVPlayer` (streaming) |
| Podcast player | audio-playback + audio-session | `AVPlayer` + Now Playing + Remote Commands |
| Background audio | audio-session | `.playback` category + `UIBackgroundModes` |
| Image gallery/feed | remote-images | `KFImage` (Kingfisher) |
| Simple remote image | remote-images | `AsyncImage` |
| Sound effects | audio-playback | `AVAudioPlayer` with `.ambient` category |
| Sample content | sample-media-urls | Public URLs, local audio generation |

## Platform Capabilities

| Feature | iOS | macOS | tvOS | watchOS | visionOS |
|---------|-----|-------|------|---------|----------|
| Video playback | VideoPlayer | AVPlayerView (NSViewRepresentable) | VideoPlayer | No | VideoPlayer |
| Audio playback | AVAudioPlayer / AVPlayer | AVAudioPlayer / AVPlayer | AVAudioPlayer / AVPlayer | WKAudioFilePlayer | AVAudioPlayer / AVPlayer |
| AVAudioSession | Yes | No (not needed) | Yes | Limited | Yes |
| Background audio | Yes (UIBackgroundModes) | N/A | No | Limited | Yes |
| Now Playing info | Yes | Yes | Limited | No | Yes |
| Remote commands | Yes | Yes | Limited | No | Yes |
| AsyncImage | Yes | Yes | Yes | Yes | Yes |
| Kingfisher | Yes | Yes | Yes | No | Yes |

## Recommended Packages

All already in the curated package registry — no new dependencies to add:

- **Kingfisher** — remote image loading with disk/memory caching, prefetch, placeholder, fade. Use for image feeds and galleries (20+ images).
- **DSWaveformImage** — audio waveform visualization. Use for voice memo or audio editing UIs.
- **AudioKit** — advanced audio synthesis and processing. Only use when the user specifically needs audio effects, synthesis, or real-time audio processing.

## Required Info.plist Keys

For background audio playback, add to the target's `info` section in `project.yml`:

```yaml
UIBackgroundModes: ["audio"]
```

No other Info.plist keys are needed for basic media playback.

## Cross-References

- **PhotosPicker** — see the `camera` skill (not duplicated here)
- **Camera access** — see the `camera` skill
- **MapKit / CoreLocation** — see the `maps` skill (not duplicated here)
- **Image containment / clip patterns** — see the `swiftui` skill
- **Haptic feedback on media controls** — see the `haptics` skill

## References

This skill includes the following reference documents (auto-loaded):

- **video-playback** — VideoPlayer, HLS streaming, custom controls, feed-style video, looping
- **audio-playback** — AVAudioPlayer (local), AVPlayer (streaming), Now Playing, remote commands, playlists
- **audio-session** — AVAudioSession categories, interruption handling, route changes, background audio
- **remote-images** — AsyncImage vs Kingfisher decision, image grids, feeds, prefetch, cache config
- **sample-media-urls** — Public video/HLS/image URLs and local audio generation with `say`
- **media-memory-management** — Player lifecycle, observer cleanup, KVO, feed coordinator, debug checklist
