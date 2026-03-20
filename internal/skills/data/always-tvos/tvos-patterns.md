---
name: "tvos-patterns"
description: "tvOS platform patterns: Siri Remote input handling, Top Shelf extensions, media playback, focus engine, parallax effects. Use when building tvOS-specific features, handling remote input, or implementing TV app patterns. Triggers: tvOS, Siri Remote, Top Shelf, AVPlayer, onMoveCommand, onPlayPauseCommand, focus engine."
---
# tvOS Platform Patterns

## Siri Remote Input
tvOS uses the Siri Remote — all interaction is through focus and button presses:

```swift
// Handle directional movement
.onMoveCommand { direction in
    switch direction {
    case .up: moveUp()
    case .down: moveDown()
    case .left: moveLeft()
    case .right: moveRight()
    @unknown default: break
    }
}

// Handle Play/Pause button
.onPlayPauseCommand {
    togglePlayback()
}

// Handle Menu/Back button
.onExitCommand {
    dismiss()
}

// Long press on select button
.onLongPressGesture {
    showOptions()
}
```

## Media Playback (AVKit)
```swift
import AVKit

struct PlayerView: View {
    let url: URL
    @State private var player: AVPlayer?

    var body: some View {
        VideoPlayer(player: player)
            .onAppear {
                player = AVPlayer(url: url)
                player?.play()
            }
            .onDisappear {
                player?.pause()
            }
            .ignoresSafeArea()
    }
}
```

## Top Shelf Extension
The only supported extension type on tvOS — shows content on the home screen:

```swift
// In Targets/TopShelf/ContentProvider.swift
import TVServices

struct TopShelfProvider: TVTopShelfProvider {
    var topShelfStyle: TVTopShelfContentStyle { .sectioned }

    var topShelfItems: [TVTopShelfSectionedItem] {
        let section = TVTopShelfItemCollection(items: [
            makeItem(id: "1", title: "Featured Movie", imageURL: url1),
            makeItem(id: "2", title: "New Release", imageURL: url2),
        ])
        section.title = "Featured"
        return [section]
    }

    func makeItem(id: String, title: String, imageURL: URL) -> TVTopShelfSectionedItem {
        let item = TVTopShelfSectionedItem(identifier: id)
        item.title = title
        item.setImageURL(imageURL, for: .screenScale1x)
        item.displayAction = TVTopShelfAction(url: URL(string: "myapp://item/\(id)")!)
        return item
    }
}
```

## Parallax Effect (Focus Art)
tvOS automatically applies parallax effects to images in focused buttons:

```swift
// Images inside .buttonStyle(.card) get parallax automatically
Button { } label: {
    Image("poster")
        .resizable()
        .aspectRatio(2/3, contentMode: .fill)
        .frame(width: 200, height: 300)
}
.buttonStyle(.card)
```

## Search
```swift
struct SearchView: View {
    @State private var query = ""
    @State private var results: [Item] = []

    var body: some View {
        NavigationStack {
            VStack {
                // tvOS shows system keyboard for text input
                TextField("Search", text: $query)
                    .onChange(of: query) { _, newQuery in
                        search(newQuery)
                    }

                if results.isEmpty {
                    ContentUnavailableView.search(text: query)
                } else {
                    ScrollView {
                        LazyVGrid(columns: [
                            GridItem(.adaptive(minimum: 200), spacing: 40)
                        ], spacing: 40) {
                            ForEach(results) { item in
                                NavigationLink(value: item) {
                                    ItemCard(item: item)
                                }
                                .buttonStyle(.card)
                            }
                        }
                        .padding(.horizontal, 80)
                    }
                }
            }
            .navigationTitle("Search")
        }
    }
}
```

## Settings Pattern
```swift
NavigationStack {
    List {
        Section("Account") {
            NavigationLink("Profile") { ProfileView() }
            NavigationLink("Subscriptions") { SubscriptionView() }
        }
        Section("Playback") {
            Toggle("Auto-Play Next", isOn: $autoPlay)
            Picker("Quality", selection: $quality) {
                Text("Auto").tag(Quality.auto)
                Text("High").tag(Quality.high)
                Text("Low").tag(Quality.low)
            }
        }
        Section("About") {
            LabeledContent("Version", value: "1.0")
        }
    }
    .navigationTitle("Settings")
}
```

## NOT Available on tvOS
- No camera, microphone, or photo library access
- No HealthKit, Core Motion, or biometrics (Face ID/Touch ID)
- No Maps or MapKit
- No haptic feedback
- No WebKit or Safari
- No user-facing push notifications (only silent push for content updates)
- No NFC, Bluetooth LE scanning, or ARKit
- No share sheet or social sharing
- No App Clips
- No home screen widgets

## Rules
1. ALL user interaction is through the Siri Remote — never assume touch input
2. Handle `onMoveCommand`, `onPlayPauseCommand`, `onExitCommand` for remote control
3. Focus management is critical — test every screen's focus flow
4. Use `AVKit` for video playback — tvOS has deep system integration
5. Top Shelf is the ONLY extension type — use TVServices framework
6. Design for 10-foot viewing distance — large text, high contrast, simple layouts
7. Prefer selection-based input (pickers, toggles) over text fields
8. Keep session state — TV apps are often backgrounded during use
