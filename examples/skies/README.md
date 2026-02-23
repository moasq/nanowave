# Skies — Multi-Platform Weather App

A weather app for iPhone, iPad, Apple Watch, and Apple TV — built entirely by Nanowave from a single prompt.

## Prompt

```
Build a beautiful weather app for iPhone, iPad, Apple Watch, and Apple TV.
Rich gradient backgrounds that change with weather conditions (sunny golden,
rainy blue-gray, stormy dark purple, snowy white). Large animated weather icons,
hourly forecast carousel, 7-day forecast cards with high/low temperatures, and
a current conditions hero section with feels-like, humidity, wind speed, and UV
index. Watch shows current temp with condition icon and a compact 3-hour
forecast. TV displays a gorgeous full-screen ambient weather dashboard with
slow-moving cloud animations.
```

## Result

Nanowave analyzed the prompt, detected four platforms (iOS, watchOS, tvOS with universal device family for iPad), and generated a single Xcode project with 27 Swift files across 4 targets.

```
  Analyzed: Skies
  Plan ready (29 files, 5 models) — iOS, watchOS, tvOS
  Build complete — 27 files
  Skies is ready!
```

## Project structure

```
Skies/
├── Skies/                          # iOS (iPhone + iPad)
│   ├── App/
│   │   ├── SkiesApp.swift          # @main entry point
│   │   ├── RootView.swift
│   │   └── MainView.swift          # Adaptive iPhone/iPad layout
│   └── Features/
│       ├── Common/
│       │   └── GlassCardModifier.swift
│       ├── Dashboard/
│       │   ├── ConditionBackgroundView.swift
│       │   ├── CurrentConditionsHeroView.swift
│       │   └── DashboardViewModel.swift
│       └── Forecast/
│           ├── DailyForecastListView.swift
│           └── HourlyCarouselView.swift
│
├── SkiesWatch/                     # watchOS
│   ├── App/
│   │   ├── SkiesWatchApp.swift     # @main entry point
│   │   ├── WatchRootView.swift
│   │   └── WatchMainView.swift
│   └── Features/
│       └── Glance/
│           └── WatchMiniCarouselView.swift
│
├── SkiesTV/                        # tvOS
│   ├── App/
│   │   ├── SkiesTVApp.swift        # @main entry point
│   │   ├── TVRootView.swift
│   │   └── TVMainView.swift
│   └── Features/
│       └── Ambient/
│           ├── TVCloudParallaxView.swift
│           ├── TVConditionsTabView.swift
│           ├── TVDailyTabView.swift
│           └── TVHourlyTabView.swift
│
├── Shared/                         # Cross-platform code
│   ├── Models/
│   │   ├── WeatherCondition.swift
│   │   ├── CurrentWeather.swift
│   │   ├── DailyForecast.swift
│   │   ├── HourlyForecast.swift
│   │   └── WeatherSnapshot.swift
│   ├── Services/
│   │   └── WeatherStore.swift
│   └── Theme/
│       └── AppTheme.swift
│
└── project.yml                     # Multi-target XcodeGen spec
```

## Platforms

| Target | Platform | Source dir | What it does |
|---|---|---|---|
| Skies | iOS (iPhone + iPad) | `Skies/` | Full dashboard with hero section, hourly carousel, 7-day forecast. iPad gets a split-view layout. |
| SkiesWatch | watchOS | `SkiesWatch/` | Compact glance: current temp, condition icon, 3-hour mini carousel. |
| SkiesTV | tvOS | `SkiesTV/` | Full-screen ambient dashboard with tab navigation, slow-moving cloud parallax animation. |
| (shared) | all | `Shared/` | Weather models, data store, theme/color system. |

## Highlights

- **Condition-driven gradients** — Background colors shift automatically: golden for sunny, blue-gray for rain, dark purple for storms, white for snow
- **Animated SF Symbols** — Weather icons use `symbolEffect(.variableColor.iterative)` for subtle animation
- **Glass morphism cards** — `.ultraThinMaterial` with rounded corners and subtle stroke overlay
- **Adaptive iPad layout** — `NavigationSplitView` with forecast sidebar and conditions detail
- **watchOS mini carousel** — Compact 3-hour forecast using system-size-appropriate fonts
- **tvOS cloud parallax** — Canvas-based animated cloud layers using `TimelineView(.animation)`
- **Zero external dependencies** — All weather data is generated on-device (simulated)
- **Swift 6 strict concurrency** — `@MainActor` isolation, `Sendable` conformance throughout
