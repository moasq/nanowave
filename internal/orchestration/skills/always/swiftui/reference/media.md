---
description: "Image containment and clipping, video thumbnails, AsyncImage, PhotosPicker, camera, MapKit, and media patterns"
---
# SwiftUI Media Reference

## Contents
- [Image Containment and Clipping](#image-containment-and-clipping)
- [Video Thumbnails](#video-thumbnails)
- [AsyncImage Containment](#asyncimage-containment)
- [PhotosPicker](#photospicker)
- [MapKit Integration](#mapkit-integration)
- [Location Services](#location-services)
- [Camera Access](#camera-access)

## Image Containment and Clipping

Images placed inside ANY sized container — cards, list rows, grid cells, headers, banners, circles, rounded rectangles — MUST be explicitly clipped. Without clipping, the image renders at its intrinsic size and overflows the container boundaries.

This applies everywhere: `.frame()`, grid cells, card thumbnails, profile avatars, hero banners, row leading icons — any time an image is placed inside a container with a defined size.

MODIFIER ORDER (CRITICAL — order matters):
```swift
// CORRECT — image fills the container and clips to its bounds
image
    .resizable()
    .scaledToFill()
    .frame(width: 120, height: 120)
    .clipped()

// WRONG — clipped before frame has no effect
image
    .resizable()
    .clipped()           // too early — no frame to clip to yet
    .scaledToFill()
    .frame(width: 120, height: 120)

// WRONG — missing clipped, image overflows container
image
    .resizable()
    .scaledToFill()
    .frame(width: 120, height: 120)
    // no .clipped() — image renders beyond the 120x120 box
```

THE OVERLAY PATTERN — the most robust way to contain an image. The image never affects the container's layout size:
```swift
Color.clear
    .aspectRatio(1, contentMode: .fit)   // or any ratio: 16/9, 3/4, etc.
    .overlay {
        image
            .resizable()
            .scaledToFill()
    }
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
    .contentShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
```

WHY Color.clear + overlay:
- `Color.clear` with `.aspectRatio()` creates a container whose size is determined by the parent layout, not by the image
- `.overlay { }` places the image on top without affecting the container's intrinsic size — so the image cannot push the container larger
- `.clipShape()` clips the rendered pixels to the container shape (rectangle, rounded rectangle, circle, etc.)
- `.contentShape()` restricts the tap target to the visible area (clipping is visual-only — without contentShape, taps register on the invisible overflow)

USE CASES — same pattern, different shapes and ratios:

```swift
// Square card thumbnail
Color.clear
    .aspectRatio(1, contentMode: .fit)
    .overlay { image.resizable().scaledToFill() }
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))

// Landscape banner / hero image (16:9)
Color.clear
    .aspectRatio(16/9, contentMode: .fit)
    .overlay { image.resizable().scaledToFill() }
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))

// Portrait card (3:4)
Color.clear
    .aspectRatio(3/4, contentMode: .fit)
    .overlay { image.resizable().scaledToFill() }
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))

// Circular avatar
Color.clear
    .aspectRatio(1, contentMode: .fit)
    .overlay { image.resizable().scaledToFill() }
    .clipShape(Circle())
    .contentShape(Circle())

// Fixed-size row thumbnail (no aspect ratio needed)
image
    .resizable()
    .scaledToFill()
    .frame(width: 60, height: 60)
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
    .contentShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
```

WHEN TO USE WHICH APPROACH:
- **Overlay pattern** (Color.clear + aspectRatio + overlay): When the container should size itself from the parent layout (grids, flexible cards, full-width banners). The image never influences layout.
- **Frame + clipped**: When you want an exact pixel size (row icons, fixed thumbnails). Simpler, but the image's intrinsic size can still affect layout of siblings if not careful.
- **Rule of thumb**: If the container is inside a LazyVGrid, LazyHGrid, ScrollView, List, or any flexible layout — use the overlay pattern. If the container has a hardcoded fixed frame — frame + clipped is fine.

COMMON MISTAKES:
| Mistake | Symptom | Fix |
|---|---|---|
| Missing `.clipped()` / `.clipShape()` | Image overflows container bounds | Add `.clipShape()` or `.clipped()` after sizing |
| `.scaledToFit()` instead of `.scaledToFill()` | Letterboxing, empty gaps around image | Use `.scaledToFill()` + clip |
| `.frame()` without `.clipped()` | Frame set but image still renders outside it | Always pair `.frame()` with `.clipped()` or `.clipShape()` |
| Image directly in flexible layout (no overlay) | Image pushes container larger than intended | Use `Color.clear` + `.aspectRatio()` + `.overlay` |
| `.clipped()` before `.frame()` | Clipping has no boundary to clip to | Move `.clipped()` after `.frame()` |
| Missing `.contentShape()` | Taps register outside visible bounds | Add `.contentShape()` after `.clipShape()` |
| `.resizable()` without `.scaledToFill()` | Image stretches non-uniformly to fill frame | Add `.scaledToFill()` after `.resizable()` |
| `.aspectRatio()` on the image instead of container | Image constrains itself before filling | Apply `.aspectRatio()` to `Color.clear`, not the image |

## Video Thumbnails

Video thumbnails use the same overlay + clip pattern. The container shape and ratio work identically.

GENERATING A THUMBNAIL:
```swift
import AVFoundation

func generateThumbnail(for url: URL) async -> UIImage? {
    let asset = AVURLAsset(url: url)
    let generator = AVAssetImageGenerator(asset: asset)
    generator.appliesPreferredTrackTransform = true
    generator.maximumSize = CGSize(width: 600, height: 600)

    guard let cgImage = try? await generator.image(at: .zero).image else {
        return nil
    }
    return UIImage(cgImage: cgImage)
}
```

VIDEO THUMBNAIL IN A CONTAINER:
```swift
struct VideoThumbnailView: View {
    let videoURL: URL
    @State private var thumbnail: UIImage?

    var body: some View {
        Color.clear
            .aspectRatio(16/9, contentMode: .fit)
            .overlay {
                Group {
                    if let thumbnail {
                        Image(uiImage: thumbnail)
                            .resizable()
                            .scaledToFill()
                    } else {
                        Rectangle()
                            .fill(AppTheme.Colors.surface)
                            .overlay { ProgressView() }
                    }
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
            .contentShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
            .overlay(alignment: .bottomTrailing) {
                Image(systemName: "play.circle.fill")
                    .font(AppTheme.Fonts.title2)
                    .foregroundStyle(.white)
                    .shadow(radius: 2)
                    .padding(AppTheme.Spacing.xs)
            }
            .task {
                thumbnail = await generateThumbnail(for: videoURL)
            }
    }
}
```

IMPORTANT: Always set `appliesPreferredTrackTransform = true` on the generator — without it, portrait videos appear rotated 90 degrees.

## Remote and Loaded Images

The overlay + clip pattern applies regardless of how the image is loaded — AsyncImage, any SPM image library (Kingfisher, Nuke, SDWebImage, etc.), or images loaded from data/disk. The containment rules are the same: the image view goes inside the `.overlay { }`, and `.clipShape()` goes on the outside.

```swift
// Example with AsyncImage — same pattern works with any image-loading view
Color.clear
    .aspectRatio(1, contentMode: .fit)
    .overlay {
        AsyncImage(url: url) { phase in
            switch phase {
            case .empty:
                Rectangle()
                    .fill(AppTheme.Colors.surface)
                    .overlay { ProgressView() }
            case .success(let image):
                image
                    .resizable()
                    .scaledToFill()
            case .failure:
                Rectangle()
                    .fill(AppTheme.Colors.surface)
                    .overlay {
                        Image(systemName: "photo")
                            .foregroundStyle(AppTheme.Colors.textTertiary)
                    }
            @unknown default:
                EmptyView()
            }
        }
    }
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
    .contentShape(RoundedRectangle(cornerRadius: AppTheme.Style.cornerRadius))
```

The same structure works when using any SPM image library — replace the `AsyncImage(...)` block with the library's image view, keeping the `Color.clear` + `.overlay` + `.clipShape()` wrapper identical.

## PhotosPicker

```swift
import PhotosUI

struct PhotoPickerView: View {
    @State private var selectedItem: PhotosPickerItem?
    @State private var selectedImage: Image?

    var body: some View {
        VStack {
            if let selectedImage {
                selectedImage
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .frame(maxHeight: 300)
            }

            PhotosPicker("Select Photo", selection: $selectedItem, matching: .images)
        }
        .onChange(of: selectedItem) { _, newItem in
            Task {
                if let data = try? await newItem?.loadTransferable(type: Data.self),
                   let uiImage = UIImage(data: data) {
                    selectedImage = Image(uiImage: uiImage)
                }
            }
        }
    }
}
```

### Multiple Photo Selection

```swift
@State private var selectedItems: [PhotosPickerItem] = []

PhotosPicker("Select Photos", selection: $selectedItems, maxSelectionCount: 5, matching: .images)
```

## MapKit Integration

```swift
import MapKit

struct MapView: View {
    @State private var position: MapCameraPosition = .automatic
    let annotations: [Location]

    var body: some View {
        Map(position: $position) {
            ForEach(annotations) { location in
                Marker(location.name, coordinate: location.coordinate)
            }
        }
        .mapControls {
            MapUserLocationButton()
            MapCompass()
            MapScaleView()
        }
    }
}
```

### Map with Custom Annotations

```swift
Map(position: $position) {
    ForEach(places) { place in
        Annotation(place.name, coordinate: place.coordinate) {
            Image(systemName: "mappin.circle.fill")
                .foregroundStyle(AppTheme.Colors.error)
                .font(AppTheme.Fonts.title)
        }
    }
}
```

## Location Services

```swift
import CoreLocation

@Observable
@MainActor
final class LocationManager: NSObject, CLLocationManagerDelegate {
    private let manager = CLLocationManager()
    var location: CLLocation?
    var authorizationStatus: CLAuthorizationStatus = .notDetermined

    override init() {
        super.init()
        manager.delegate = self
    }

    func requestPermission() {
        manager.requestWhenInUseAuthorization()
    }

    nonisolated func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        Task { @MainActor in
            location = locations.last
        }
    }

    nonisolated func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        Task { @MainActor in
            authorizationStatus = manager.authorizationStatus
        }
    }
}
```

## Camera Access

```swift
struct CameraView: UIViewControllerRepresentable {
    @Binding var image: UIImage?
    @Environment(\.dismiss) private var dismiss

    func makeUIViewController(context: Context) -> UIImagePickerController {
        let picker = UIImagePickerController()
        picker.sourceType = .camera
        picker.delegate = context.coordinator
        return picker
    }

    func updateUIViewController(_ uiViewController: UIImagePickerController, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator(self)
    }

    class Coordinator: NSObject, UIImagePickerControllerDelegate, UINavigationControllerDelegate {
        let parent: CameraView
        init(_ parent: CameraView) { self.parent = parent }

        func imagePickerController(_ picker: UIImagePickerController, didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]) {
            parent.image = info[.originalImage] as? UIImage
            parent.dismiss()
        }
    }
}
```
