---
name: "widgets"
description: "watchOS complications: WidgetKit complication families, accessory sizes, timeline providers for watch face. Use when implementing watchOS-specific patterns related to widgets."
---
# Widgets / Complications (watchOS)

WATCH COMPLICATIONS (WidgetKit):
- watchOS uses WidgetKit for complications (same framework as iOS widgets)
- Complications appear on watch faces — must be ultra-compact and glanceable

COMPLICATION FAMILIES:

| Family | Shape | Use Case |
|--------|-------|----------|
| `.accessoryCircular` | Small circle | Single metric, icon + value |
| `.accessoryRectangular` | Rectangle | Multi-line text, small chart |
| `.accessoryInline` | Single line text | Short status text on watch face |
| `.accessoryCorner` | Corner gauge | Gauge with label |

TIMELINE PROVIDER:
```swift
struct ComplicationProvider: TimelineProvider {
    func placeholder(in context: Context) -> ComplicationEntry {
        ComplicationEntry(date: .now, value: 0, label: "—")
    }

    func getSnapshot(in context: Context, completion: @escaping (ComplicationEntry) -> Void) {
        completion(ComplicationEntry(date: .now, value: 42, label: "Steps"))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<ComplicationEntry>) -> Void) {
        let entry = ComplicationEntry(date: .now, value: 42, label: "Steps")
        let timeline = Timeline(entries: [entry], policy: .after(.now.addingTimeInterval(900)))
        completion(timeline)
    }
}

struct ComplicationEntry: TimelineEntry {
    let date: Date
    let value: Int
    let label: String
}
```

COMPLICATION VIEWS:
```swift
struct ComplicationView: View {
    var entry: ComplicationProvider.Entry
    @Environment(\.widgetFamily) var family

    var body: some View {
        switch family {
        case .accessoryCircular:
            Gauge(value: Double(entry.value), in: 0...100) {
                Text(entry.label)
            }
            .gaugeStyle(.accessoryCircularCapacity)

        case .accessoryRectangular:
            VStack(alignment: .leading) {
                Text(entry.label)
                    .font(.headline)
                    .widgetAccentable()
                Text("\(entry.value)")
                    .font(.title2)
            }

        case .accessoryInline:
            Text("\(entry.label): \(entry.value)")

        case .accessoryCorner:
            Text("\(entry.value)")
                .widgetLabel {
                    Text(entry.label)
                }

        default:
            Text(entry.label)
        }
    }
}
```

WIDGET DEFINITION:
```swift
struct MyComplication: Widget {
    let kind: String = "MyComplication"

    var body: some WidgetConfiguration {
        StaticConfiguration(kind: kind, provider: ComplicationProvider()) { entry in
            ComplicationView(entry: entry)
                .containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("My Complication")
        .description("Shows current status on watch face")
        .supportedFamilies([
            .accessoryCircular,
            .accessoryRectangular,
            .accessoryInline,
            .accessoryCorner
        ])
    }
}
```

WIDGET BUNDLE:
```swift
@main
struct MyWidgetBundle: WidgetBundle {
    var body: some Widget {
        MyComplication()
    }
}
```

CRITICAL RULES:
- `.containerBackground(.fill.tertiary, for: .widget)` is REQUIRED
- @main entry point is MANDATORY on the WidgetBundle
- Use `.widgetAccentable()` on elements that should tint with the watch face color
- Complications must be self-contained — no @StateObject, @ObservedObject, or network calls
- Use `.accessoryCircular` for the most common single-value complication
- Keep text ultra-short — complications have very limited space
- Shared data types go in `Shared/` directory
- Do NOT use iOS families (.systemSmall, .systemMedium, etc.) — they don't exist on watchOS
- AppIntent static properties must use `static let` for Swift 6 concurrency
