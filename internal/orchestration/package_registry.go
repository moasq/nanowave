package orchestration

// CuratedPackage describes a pre-validated SPM package in the registry.
type CuratedPackage struct {
	Key         string   // lookup key, e.g. "kingfisher"
	Name        string   // display name, e.g. "Kingfisher"
	Category    string   // category key matching PackageCategory.Key
	Description string   // what it enables beyond native frameworks
	RepoURL     string   // full GitHub URL
	RepoName    string   // last path component of URL (XcodeGen packages: key)
	Products    []string // SPM product names from Package.swift (import + product: in XcodeGen)
	MinVersion  string   // minimum version to use with from:
}

// PackageCategory groups packages under a human-readable label.
type PackageCategory struct {
	Key         string // e.g. "images"
	Label       string // e.g. "Image Loading & Caching"
	Description string // when to use this category
}

// packageCategories defines the available categories in display order.
var packageCategories = []PackageCategory{
	// Media
	{Key: "images", Label: "Image Loading & Caching", Description: "Disk-cached remote image loading, prefetch, downsampling, animated GIFs"},
	{Key: "gif", Label: "Animated GIFs", Description: "High-performance animated GIF rendering and display"},
	{Key: "svg", Label: "SVG Rendering", Description: "Display SVG vector graphics as native SwiftUI views"},
	{Key: "image-editing", Label: "Image Editing", Description: "Photo cropping, filters, adjustments, and composable editing UI"},
	{Key: "waveform", Label: "Audio Waveform", Description: "Visualize audio files as waveform images in SwiftUI"},
	{Key: "audio", Label: "Audio Engine", Description: "Audio synthesis, processing, and analysis beyond AVFoundation"},

	// Animations & Effects
	{Key: "animations", Label: "Animations", Description: "After Effects playback, vector animations, Lottie JSON"},
	{Key: "effects", Label: "Visual Effects", Description: "Confetti, particles, celebration effects, delightful transitions"},
	{Key: "shimmer", Label: "Shimmer & Skeleton", Description: "Loading placeholder shimmer effects and skeleton views"},
	{Key: "loading-indicators", Label: "Loading Indicators", Description: "Custom animated loading indicators beyond ProgressView"},

	// Layouts
	{Key: "flow-layout", Label: "Flow / Wrap Layout", Description: "Wrapping layout where items flow to next line — tag clouds, filter chips, skill badges. Native HStack/VStack do NOT wrap."},
	{Key: "waterfall-grid", Label: "Waterfall / Masonry Grid", Description: "Pinterest-style staggered grid with variable-height items. Native LazyVGrid forces equal row heights."},

	// UI Components
	{Key: "toasts", Label: "Toasts & Popups", Description: "In-app toast notifications, popups, floating alerts — no native SwiftUI equivalent"},
	{Key: "onboarding", Label: "Onboarding & What's New", Description: "Welcome screens, feature tours, version changelogs"},
	{Key: "calendar", Label: "Calendar UI", Description: "Custom calendar views for date picking and scheduling"},
	{Key: "chat-ui", Label: "Chat UI", Description: "Pre-built chat message interfaces with media, replies, and customization"},

	// Text & Content
	{Key: "markdown", Label: "Markdown Rendering", Description: "Render Markdown text as native SwiftUI views"},
	{Key: "rich-text", Label: "Rich Text Editing", Description: "Attributed text editing with bold, italic, fonts, colors in SwiftUI"},

	// Code Display
	{Key: "syntax-highlighting", Label: "Syntax Highlighting", Description: "Highlight code in 185+ languages with color themes — no native API exists"},

	// Scanning & Codes
	{Key: "qr-codes", Label: "QR Code Generation", Description: "Generate stylized QR codes with logos, colors, and custom shapes. Native CIQRCodeGenerator handles plain QR codes."},

	// Data & Security
	{Key: "keychain", Label: "Keychain Storage", Description: "Simple secure storage in the iOS Keychain without Keychain API complexity"},

	// Backend
	{Key: "backend", Label: "Backend", Description: "Server-side backend SDKs"},
	{Key: "monetization", Label: "Monetization", Description: "In-app purchases, subscriptions, and paywall management"},
}

// curatedPackages is the registry of pre-validated, top-tier SPM packages.
// Only packages with >500 GitHub stars, active maintenance, and MIT/Apache 2.0 license are included.
var curatedPackages = []CuratedPackage{

	// ── Images ──────────────────────────────────────────────────────────
	{
		Key:         "kingfisher",
		Name:        "Kingfisher",
		Category:    "images",
		Description: "Disk-cached image loading with prefetch, progressive decoding, and built-in SwiftUI views (KFImage, KFAnimatedImage)",
		RepoURL:     "https://github.com/onevcat/Kingfisher",
		RepoName:    "Kingfisher",
		Products:    []string{"Kingfisher"},
		MinVersion:  "8.1.0",
	},
	{
		Key:         "nuke",
		Name:        "Nuke",
		Category:    "images",
		Description: "High-performance image loading with memory/disk caching, progressive JPEG, request coalescing, and SwiftUI LazyImage view (via NukeUI)",
		RepoURL:     "https://github.com/kean/Nuke",
		RepoName:    "Nuke",
		Products:    []string{"Nuke", "NukeUI"},
		MinVersion:  "12.8.0",
	},
	{
		Key:         "sdwebimage-swiftui",
		Name:        "SDWebImageSwiftUI",
		Category:    "images",
		Description: "SwiftUI image loading with memory/disk caching and animated GIF playback (WebImage, AnimatedImage), powered by SDWebImage",
		RepoURL:     "https://github.com/SDWebImage/SDWebImageSwiftUI",
		RepoName:    "SDWebImageSwiftUI",
		Products:    []string{"SDWebImageSwiftUI"},
		MinVersion:  "3.1.0",
	},

	// ── Animated GIFs ──────────────────────────────────────────────────
	{
		Key:         "gifu",
		Name:        "Gifu",
		Category:    "gif",
		Description: "High-performance animated GIF rendering with memory-efficient frame caching (UIKit, wrap with UIViewRepresentable for SwiftUI)",
		RepoURL:     "https://github.com/kaishin/Gifu",
		RepoName:    "Gifu",
		Products:    []string{"Gifu"},
		MinVersion:  "4.0.0",
	},

	// ── SVG Rendering ──────────────────────────────────────────────────
	{
		Key:         "svgview",
		Name:        "SVGView",
		Category:    "svg",
		Description: "SVG parser and renderer as native SwiftUI views with interactive elements and animation support",
		RepoURL:     "https://github.com/exyte/SVGView",
		RepoName:    "SVGView",
		Products:    []string{"SVGView"},
		MinVersion:  "1.0.6",
	},

	// ── Image Editing ──────────────────────────────────────────────────
	{
		Key:         "brightroom",
		Name:        "Brightroom",
		Category:    "image-editing",
		Description: "Composable image editor with crop, filters, and adjustments using CoreImage and Metal",
		RepoURL:     "https://github.com/FluidGroup/Brightroom",
		RepoName:    "Brightroom",
		Products:    []string{"BrightroomEngine", "BrightroomUI"},
		MinVersion:  "3.0.0",
	},
	{
		Key:         "cropviewcontroller",
		Name:        "CropViewController",
		Category:    "image-editing",
		Description: "Full-featured image cropping with rotation, aspect ratio presets, and gesture-based interaction (UIKit, wrap for SwiftUI)",
		RepoURL:     "https://github.com/TimOliver/TOCropViewController",
		RepoName:    "TOCropViewController",
		Products:    []string{"CropViewController"},
		MinVersion:  "3.1.0",
	},

	// ── Audio Waveform ─────────────────────────────────────────────────
	{
		Key:         "dswaveformimage",
		Name:        "DSWaveformImage",
		Category:    "waveform",
		Description: "Generate and display audio waveform images with native SwiftUI views (WaveformView, WaveformLiveCanvas) and custom styling",
		RepoURL:     "https://github.com/dmrschmidt/DSWaveformImage",
		RepoName:    "DSWaveformImage",
		Products:    []string{"DSWaveformImage", "DSWaveformImageViews"},
		MinVersion:  "14.0.0",
	},

	// ── Audio Engine ───────────────────────────────────────────────────
	{
		Key:         "audiokit",
		Name:        "AudioKit",
		Category:    "audio",
		Description: "Full audio synthesis, processing, and analysis platform beyond AVFoundation — oscillators, effects, sequencing",
		RepoURL:     "https://github.com/AudioKit/AudioKit",
		RepoName:    "AudioKit",
		Products:    []string{"AudioKit"},
		MinVersion:  "5.6.0",
	},

	// ── Animations ─────────────────────────────────────────────────────
	{
		Key:         "lottie",
		Name:        "Lottie",
		Category:    "animations",
		Description: "Render After Effects vector animations from JSON with native SwiftUI LottieView",
		RepoURL:     "https://github.com/airbnb/lottie-spm",
		RepoName:    "lottie-spm",
		Products:    []string{"Lottie"},
		MinVersion:  "4.5.0",
	},

	// ── Visual Effects ─────────────────────────────────────────────────
	{
		Key:         "confetti",
		Name:        "ConfettiSwiftUI",
		Category:    "effects",
		Description: "Configurable confetti animations with shapes, emojis, SF Symbols, and haptic feedback — pure SwiftUI",
		RepoURL:     "https://github.com/simibac/ConfettiSwiftUI",
		RepoName:    "ConfettiSwiftUI",
		Products:    []string{"ConfettiSwiftUI"},
		MinVersion:  "1.0.0",
	},
	{
		Key:         "pow",
		Name:        "Pow",
		Category:    "effects",
		Description: "Delightful SwiftUI transition and change effects — anvil, blur, clock, spray, shake, shine, spin, ping",
		RepoURL:     "https://github.com/EmergeTools/Pow",
		RepoName:    "Pow",
		Products:    []string{"Pow"},
		MinVersion:  "1.0.0",
	},
	{
		Key:         "vortex",
		Name:        "Vortex",
		Category:    "effects",
		Description: "High-performance SwiftUI particle effects with built-in presets for fire, rain, smoke, snow, and confetti",
		RepoURL:     "https://github.com/twostraws/Vortex",
		RepoName:    "Vortex",
		Products:    []string{"Vortex"},
		MinVersion:  "1.0.0",
	},

	// ── Shimmer & Skeleton ─────────────────────────────────────────────
	{
		Key:         "shimmer",
		Name:        "Shimmer",
		Category:    "shimmer",
		Description: "Lightweight .shimmering() modifier for animated shimmer loading effects. Native .redacted(reason: .placeholder) covers static skeletons; this adds the animated shimmer with a single modifier.",
		RepoURL:     "https://github.com/markiv/SwiftUI-Shimmer",
		RepoName:    "SwiftUI-Shimmer",
		Products:    []string{"Shimmer"},
		MinVersion:  "1.5.0",
	},

	// ── Loading Indicators ─────────────────────────────────────────────
	{
		Key:         "activity-indicator",
		Name:        "ActivityIndicatorView",
		Category:    "loading-indicators",
		Description: "30+ preset animated loading indicators (arcs, dots, equalizer, gradient) in pure SwiftUI — beyond native ProgressView",
		RepoURL:     "https://github.com/exyte/ActivityIndicatorView",
		RepoName:    "ActivityIndicatorView",
		Products:    []string{"ActivityIndicatorView"},
		MinVersion:  "1.1.0",
	},

	// ── Toasts & Popups ────────────────────────────────────────────────
	{
		Key:         "popupview",
		Name:        "PopupView",
		Category:    "toasts",
		Description: "Toasts, popups, and floating alerts for SwiftUI with top/bottom/center positioning and customizable animations",
		RepoURL:     "https://github.com/exyte/PopupView",
		RepoName:    "PopupView",
		Products:    []string{"PopupView"},
		MinVersion:  "3.0.0",
	},
	{
		Key:         "alerttoast",
		Name:        "AlertToast",
		Category:    "toasts",
		Description: "Apple-style toast alerts for SwiftUI — success, error, loading, and info HUD displays",
		RepoURL:     "https://github.com/elai950/AlertToast",
		RepoName:    "AlertToast",
		Products:    []string{"AlertToast"},
		MinVersion:  "1.3.9",
	},

	// ── Onboarding & What's New ────────────────────────────────────────
	{
		Key:         "whatsnewkit",
		Name:        "WhatsNewKit",
		Category:    "onboarding",
		Description: "Apple-style 'What's New' version changelog screens for SwiftUI — feature list, icons, and buttons",
		RepoURL:     "https://github.com/SvenTiigi/WhatsNewKit",
		RepoName:    "WhatsNewKit",
		Products:    []string{"WhatsNewKit"},
		MinVersion:  "2.0.0",
	},
	{
		Key:         "concentric-onboarding",
		Name:        "ConcentricOnboarding",
		Category:    "onboarding",
		Description: "Walkthrough/onboarding flow with concentric circle tap-action animations and page navigation in SwiftUI",
		RepoURL:     "https://github.com/exyte/ConcentricOnboarding",
		RepoName:    "ConcentricOnboarding",
		Products:    []string{"ConcentricOnboarding"},
		MinVersion:  "1.1.0",
	},

	// ── Calendar UI ────────────────────────────────────────────────────
	{
		Key:         "horizoncalendar",
		Name:        "HorizonCalendar",
		Category:    "calendar",
		Description: "Declarative, performant calendar UI by Airbnb — date picking, range selection, and custom day views with SwiftUI support",
		RepoURL:     "https://github.com/airbnb/HorizonCalendar",
		RepoName:    "HorizonCalendar",
		Products:    []string{"HorizonCalendar"},
		MinVersion:  "2.0.0",
	},

	// ── Chat UI ────────────────────────────────────────────────────────
	{
		Key:         "exyte-chat",
		Name:        "ExyteChat",
		Category:    "chat-ui",
		Description: "SwiftUI chat UI framework with customizable message cells, media picker, audio recording, replies, and link previews",
		RepoURL:     "https://github.com/exyte/Chat",
		RepoName:    "Chat",
		Products:    []string{"ExyteChat"},
		MinVersion:  "2.0.0",
	},

	// ── Markdown ───────────────────────────────────────────────────────
	{
		Key:         "markdown-ui",
		Name:        "MarkdownUI",
		Category:    "markdown",
		Description: "Display and customize GitHub Flavored Markdown as native SwiftUI views — headings, lists, code blocks, tables, images",
		RepoURL:     "https://github.com/gonzalezreal/swift-markdown-ui",
		RepoName:    "swift-markdown-ui",
		Products:    []string{"MarkdownUI"},
		MinVersion:  "2.4.0",
	},

	// ── Rich Text ──────────────────────────────────────────────────────
	{
		Key:         "richtextkit",
		Name:        "RichTextKit",
		Category:    "rich-text",
		Description: "Rich text editing in SwiftUI with bold, italic, underline, fonts, colors, alignment, and image attachments",
		RepoURL:     "https://github.com/danielsaidi/RichTextKit",
		RepoName:    "RichTextKit",
		Products:    []string{"RichTextKit"},
		MinVersion:  "1.1.0",
	},

	// ── QR Codes ───────────────────────────────────────────────────────
	{
		Key:         "efqrcode",
		Name:        "EFQRCode",
		Category:    "qr-codes",
		Description: "Stylized QR code generation with watermarks, icons, custom shapes, and animated GIF output. Use native CIQRCodeGenerator for plain QR codes.",
		RepoURL:     "https://github.com/EFPrefix/EFQRCode",
		RepoName:    "EFQRCode",
		Products:    []string{"EFQRCode"},
		MinVersion:  "7.0.0",
	},

	// ── Flow / Wrap Layout ────────────────────────────────────────────
	{
		Key:         "swiftui-flow",
		Name:        "SwiftUI-Flow",
		Category:    "flow-layout",
		Description: "HFlow and VFlow layout containers where items wrap to the next line — essential for tag clouds, filter chips, and skill badges. Native HStack/VStack do not wrap.",
		RepoURL:     "https://github.com/tevelee/SwiftUI-Flow",
		RepoName:    "SwiftUI-Flow",
		Products:    []string{"Flow"},
		MinVersion:  "3.1.0",
	},

	// ── Waterfall / Masonry Grid ──────────────────────────────────────
	{
		Key:         "waterfallgrid",
		Name:        "WaterfallGrid",
		Category:    "waterfall-grid",
		Description: "Pinterest-style staggered grid layout with variable-height items flowing into columns. Native LazyVGrid forces equal row heights.",
		RepoURL:     "https://github.com/paololeonardi/WaterfallGrid",
		RepoName:    "WaterfallGrid",
		Products:    []string{"WaterfallGrid"},
		MinVersion:  "1.1.0",
	},

	// ── Syntax Highlighting ───────────────────────────────────────────
	{
		Key:         "highlightr",
		Name:        "Highlightr",
		Category:    "syntax-highlighting",
		Description: "Syntax highlighting for 185+ programming languages with 89 color themes using highlight.js — no native code highlighting API exists",
		RepoURL:     "https://github.com/raspu/Highlightr",
		RepoName:    "Highlightr",
		Products:    []string{"Highlightr"},
		MinVersion:  "2.2.0",
	},

	// ── Keychain Storage ───────────────────────────────────────────────
	{
		Key:         "keychainswift",
		Name:        "KeychainSwift",
		Category:    "keychain",
		Description: "Simple helper functions for saving text and data securely in the iOS Keychain",
		RepoURL:     "https://github.com/evgenyneu/keychain-swift",
		RepoName:    "keychain-swift",
		Products:    []string{"KeychainSwift"},
		MinVersion:  "24.0.0",
	},
	{
		Key:         "valet",
		Name:        "Valet",
		Category:    "keychain",
		Description: "Securely store data in the Keychain without knowing the Keychain API — by Square, with biometric and shared access group support",
		RepoURL:     "https://github.com/square/Valet",
		RepoName:    "Valet",
		Products:    []string{"Valet"},
		MinVersion:  "4.3.0",
	},

	// ── Backend ────────────────────────────────────────────────────────
	{
		Key:         "supabase-swift",
		Name:        "Supabase",
		Category:    "backend",
		Description: "Swift client for Supabase: auth, PostgreSQL, real-time, storage",
		RepoURL:     "https://github.com/supabase/supabase-swift",
		RepoName:    "supabase-swift",
		Products:    []string{"Supabase"},
		MinVersion:  "2.0.0",
	},

	// ── Monetization ──────────────────────────────────────────────────
	{
		Key:         "purchases-ios",
		Name:        "RevenueCat",
		Category:    "monetization",
		Description: "In-app purchases and subscriptions via RevenueCat SDK",
		RepoURL:     "https://github.com/RevenueCat/purchases-ios",
		RepoName:    "purchases-ios",
		Products:    []string{"RevenueCat"},
		MinVersion:  "5.0.0",
	},
}

// packageIndex is a key→package lookup built at init time.
var packageIndex map[string]*CuratedPackage

// categoryIndex is a category→packages lookup built at init time.
var categoryIndex map[string][]*CuratedPackage

func init() {
	packageIndex = make(map[string]*CuratedPackage, len(curatedPackages))
	categoryIndex = make(map[string][]*CuratedPackage)
	for i := range curatedPackages {
		pkg := &curatedPackages[i]
		packageIndex[pkg.Key] = pkg
		categoryIndex[pkg.Category] = append(categoryIndex[pkg.Category], pkg)
	}
}

// LookupPackage returns the curated package for a given key, or nil if not found.
func LookupPackage(key string) *CuratedPackage {
	return packageIndex[key]
}

// LookupPackageByName tries to find a curated package by its display name (case-insensitive).
// This is used when the planner outputs a name like "Kingfisher" instead of the key "kingfisher".
func LookupPackageByName(name string) *CuratedPackage {
	for i := range curatedPackages {
		if equalFoldASCII(curatedPackages[i].Name, name) || equalFoldASCII(curatedPackages[i].Key, name) {
			return &curatedPackages[i]
		}
	}
	return nil
}

// PackagesByCategory returns all curated packages in a category.
func PackagesByCategory(category string) []*CuratedPackage {
	return categoryIndex[category]
}

// AllCategories returns the category definitions in display order.
func AllCategories() []PackageCategory {
	return packageCategories
}

// AllPackages returns a copy of the full curated package list.
func AllPackages() []CuratedPackage {
	result := make([]CuratedPackage, len(curatedPackages))
	copy(result, curatedPackages)
	return result
}

// equalFoldASCII is a simple ASCII case-insensitive comparison.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
