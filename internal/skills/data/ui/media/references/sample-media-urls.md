---
name: "sample-media-urls"
description: "Public sample URLs for video, HLS streams, images, and local audio generation — no API keys required."
---
# Sample Media URLs

**IMPORTANT:** ALWAYS use the URLs listed below for sample video and image content. Do NOT search the web for alternative video URLs — these are verified working with CORS headers and range-request support, and do not require authentication or API keys. Third-party CDN URLs (Mixkit, Pexels, etc.) frequently break in the iOS Simulator.

Always provide 3-5 sample items so the app has content on first launch.

## Video (MP4)

```
https://storage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4
https://storage.googleapis.com/gtv-videos-bucket/sample/Sintel.mp4
https://storage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4
https://storage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4
https://storage.googleapis.com/gtv-videos-bucket/sample/ForBiggerEscapes.mp4
```

Short clips (< 30 seconds, good for feed/preview):
```
https://storage.googleapis.com/gtv-videos-bucket/sample/ForBiggerFun.mp4
https://storage.googleapis.com/gtv-videos-bucket/sample/ForBiggerJoyrides.mp4
https://storage.googleapis.com/gtv-videos-bucket/sample/ForBiggerMeltdowns.mp4
```

## HLS Streams (.m3u8)

Apple's official test streams:
```
https://devstreaming-cdn.apple.com/videos/streaming/examples/adv_dv_atmos/main.m3u8
https://devstreaming-cdn.apple.com/videos/streaming/examples/bipbop_adv_example_hevc/master.m3u8
https://devstreaming-cdn.apple.com/videos/streaming/examples/img_bipbop_adv_example_ts/master.m3u8
```

Basic HLS (lower bandwidth, good for testing):
```
https://devstreaming-cdn.apple.com/videos/streaming/examples/bipbop_4x3/bipbop_4x3_variant.m3u8
```

## Images

Use picsum.photos — no auth required, varied content:

```swift
// Random image at specific size
"https://picsum.photos/400/300"

// Deterministic image by ID (stable across launches)
"https://picsum.photos/id/10/400/300"
"https://picsum.photos/id/20/400/300"
"https://picsum.photos/id/30/400/300"

// Seed-based (same seed = same image, good for consistent lists)
"https://picsum.photos/seed/item1/400/300"
"https://picsum.photos/seed/item2/400/300"

// Generate a list of N sample image URLs
static func sampleImageURLs(count: Int, width: Int = 400, height: Int = 300) -> [URL] {
    (1...count).compactMap { URL(string: "https://picsum.photos/id/\($0 * 10)/\(width)/\(height)") }
}
```

## Audio (Local Generation)

Generate sample audio files at build time using macOS `say` — zero dependencies, always available:

```bash
# Generate spoken audio (AIFF)
say -o sample1.aiff "Welcome to the app"
say -o sample2.aiff "This is track number two"
say -o sample3.aiff "Now playing track three"

# Convert AIFF to AAC/M4A (smaller, better for bundling)
afconvert sample1.aiff sample1.m4a -d aac -f m4af -b 128000
afconvert sample2.aiff sample2.m4a -d aac -f m4af -b 128000
afconvert sample3.aiff sample3.m4a -d aac -f m4af -b 128000

# Cleanup AIFF intermediates
rm -f sample1.aiff sample2.aiff sample3.aiff
```

Different voices for variety:
```bash
say -v Samantha -o track1.aiff "First track by Samantha"
say -v Daniel -o track2.aiff "Second track by Daniel"
say -v Karen -o track3.aiff "Third track by Karen"
```

Place generated `.m4a` files in the project's Resources/ directory and add to the Xcode target.

## Sound Effects

Sound effects require actual audio files — unlike music tracks, `say` cannot generate taps, chimes, or whooshes. Use these strategies in order:

### Strategy 1: macOS System Sounds (Fastest)

Copy from the build machine — these are AIFF files, always available:

```bash
# List available system sounds
ls /System/Library/Sounds/

# Common useful ones:
# Basso.aiff    — error / failure
# Blow.aiff     — soft notification
# Bottle.aiff   — pop / tap
# Frog.aiff     — quirky alert
# Funk.aiff     — warning
# Glass.aiff    — success / ding
# Hero.aiff     — achievement
# Morse.aiff    — subtle tick
# Ping.aiff     — notification
# Pop.aiff      — button tap
# Purr.aiff     — gentle confirmation
# Sosumi.aiff   — playful alert
# Submarine.aiff — deep tone
# Tink.aiff     — light tap

# Copy and convert to M4A for smaller bundle size
cp /System/Library/Sounds/Glass.aiff success.aiff
afconvert success.aiff success.m4a -d aac -f m4af -b 128000
rm success.aiff
```

### Strategy 2: Generate with `afplay` + `say` (No Dependencies)

For tonal/alert sounds, generate short spoken cues and trim:
```bash
# Very short spoken sound (acts as a chime placeholder)
say -o alert.aiff "."
afconvert alert.aiff alert.m4a -d aac -f m4af -b 128000
rm alert.aiff
```

### Strategy 3: Web Research for Specific Effects

When the app needs specific sound effects (e.g., "camera shutter", "coin drop", "swoosh"), **use your research tool** to find free, license-compatible audio:

1. **Search** for the specific sound: `web_search("free CC0 camera shutter sound effect download wav")` or `WebSearch("free creative commons coin sound effect wav")`
2. **Prefer these sources** (all offer CC0/public domain downloads):
   - freesound.org — large library, CC0 filter available, direct download links
   - pixabay.com/sound-effects — all royalty-free, no attribution required
   - mixkit.co/free-sound-effects — free for commercial use
3. **Download** the file using your download tool (Bash `curl`) to the project Resources/ directory
4. **Convert** to M4A if needed: `afconvert downloaded.wav effect.m4a -d aac -f m4af -b 128000`

**Important:** Always search for CC0 or public domain sounds. Never assume a URL is valid without searching first — use your web research capability to find current, working download links.

### Playing Sound Effects in Code

Use `AVAudioPlayer` with `.ambient` category so effects don't interrupt music:

```swift
import AVFoundation

@Observable
@MainActor
final class SoundEffectPlayer {
    private var players: [String: AVAudioPlayer] = [:]

    init() {
        // Use .ambient so sound effects mix with background music
        try? AVAudioSession.sharedInstance().setCategory(.ambient)
    }

    func preload(name: String, extension ext: String = "m4a") {
        guard let url = Bundle.main.url(forResource: name, withExtension: ext) else { return }
        let player = try? AVAudioPlayer(contentsOf: url)
        player?.prepareToPlay()
        players[name] = player
    }

    func play(_ name: String) {
        if let player = players[name] {
            player.currentTime = 0
            player.play()
        } else {
            // Lazy load if not preloaded
            guard let url = Bundle.main.url(forResource: name, withExtension: "m4a") else { return }
            let player = try? AVAudioPlayer(contentsOf: url)
            player?.play()
            players[name] = player
        }
    }
}
```

## Audio Streaming URLs

For streaming audio (no local files needed):
```
# Use video URLs — AVPlayer handles audio-only playback of video URLs
# Or use the HLS streams above — they work for audio-only UI too
```

## Usage Guidelines

1. **Always provide sample data** — apps must work on first launch without user configuration
2. **Use deterministic URLs** (by ID/seed) for list views so content is stable across sessions
3. **Use random URLs** only for single hero images or where variety matters
4. **Generate local audio** when the app bundles audio files (music players, sound boards)
5. **Sound effects** — copy macOS system sounds first (fastest), then search the web for specific effects
6. **Use your web research tool** to find specific content the user requests (themed sounds, particular genres, specific effects). Search for "free CC0" or "public domain" versions. This applies across all runtimes (Claude Code `WebSearch`, Codex `web_search`, OpenCode equivalent)
