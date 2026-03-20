---
name: "remote-images"
description: "Remote image loading: AsyncImage for simple cases, Kingfisher for feeds/galleries with caching and prefetch."
---
# Remote Images

## Decision Rule

| Scenario | Use | Why |
|----------|-----|-----|
| 1-5 images, simple display | `AsyncImage` | Built-in, no dependency |
| Image feed, gallery, 20+ images | Kingfisher (`KFImage`) | Disk caching, prefetch, memory management |
| Grid with scroll prefetch | Kingfisher | `ImagePrefetcher` prevents blank cells |
| Animated loading effects | Kingfisher | Built-in fade, placeholder, progress |

## AsyncImage (Built-in)

```swift
AsyncImage(url: URL(string: "https://picsum.photos/id/10/400/300")) { phase in
    switch phase {
    case .empty:
        ProgressView()
    case .success(let image):
        image
            .resizable()
            .aspectRatio(contentMode: .fill)
    case .failure:
        Image(systemName: "photo")
            .foregroundStyle(Color(AppTheme.Colors.textSecondary))
    @unknown default:
        EmptyView()
    }
}
.frame(width: 400, height: 300)
.clipShape(RoundedRectangle(cornerRadius: AppTheme.Radius.md))
```

**Limitations of AsyncImage:**
- No disk caching (re-downloads every app launch)
- No prefetching
- No placeholder crossfade
- No retry on failure

These limitations don't matter for a few images but become obvious in feeds/galleries.

## Kingfisher — Image Feeds and Galleries

**Package:** Kingfisher (already in the curated package registry)

### Basic Usage

```swift
import Kingfisher

KFImage(URL(string: imageURLString))
    .placeholder {
        Color(AppTheme.Colors.surfaceSecondary)
    }
    .fade(duration: 0.25)
    .resizable()
    .aspectRatio(contentMode: .fill)
    .frame(width: 200, height: 200)
    .clipShape(RoundedRectangle(cornerRadius: AppTheme.Radius.md))
```

### Image Grid with Prefetch

```swift
struct ImageGridView: View {
    let imageURLs: [URL]
    private let columns = [
        GridItem(.flexible(), spacing: AppTheme.Spacing.sm),
        GridItem(.flexible(), spacing: AppTheme.Spacing.sm),
        GridItem(.flexible(), spacing: AppTheme.Spacing.sm)
    ]

    var body: some View {
        ScrollView {
            LazyVGrid(columns: columns, spacing: AppTheme.Spacing.sm) {
                ForEach(imageURLs, id: \.self) { url in
                    KFImage(url)
                        .placeholder {
                            Color(AppTheme.Colors.surfaceSecondary)
                        }
                        .fade(duration: 0.2)
                        .resizable()
                        .aspectRatio(1, contentMode: .fill)
                        .clipShape(RoundedRectangle(cornerRadius: AppTheme.Radius.sm))
                }
            }
            .padding(AppTheme.Spacing.md)
        }
        .onAppear {
            // Prefetch all images for smooth scrolling
            let prefetcher = ImagePrefetcher(urls: imageURLs)
            prefetcher.start()
        }
    }
}
```

### Feed with Lazy Loading

```swift
struct ImageFeedView: View {
    let items: [FeedItem]

    var body: some View {
        ScrollView {
            LazyVStack(spacing: AppTheme.Spacing.md) {
                ForEach(items) { item in
                    VStack(alignment: .leading, spacing: AppTheme.Spacing.sm) {
                        KFImage(item.imageURL)
                            .placeholder {
                                RoundedRectangle(cornerRadius: AppTheme.Radius.md)
                                    .fill(Color(AppTheme.Colors.surfaceSecondary))
                                    .aspectRatio(16/9, contentMode: .fit)
                            }
                            .fade(duration: 0.25)
                            .resizable()
                            .aspectRatio(16/9, contentMode: .fill)
                            .clipShape(RoundedRectangle(cornerRadius: AppTheme.Radius.md))

                        Text(item.title)
                            .font(AppTheme.Typography.headline)
                    }
                }
            }
            .padding(AppTheme.Spacing.md)
        }
    }
}
```

### Cache Configuration

Kingfisher caches images automatically. For custom limits:

```swift
// In app setup (e.g., init of App struct)
let cache = ImageCache.default
cache.memoryStorage.config.totalCostLimit = 100 * 1024 * 1024 // 100 MB memory
cache.diskStorage.config.sizeLimit = 500 * 1024 * 1024 // 500 MB disk
```

### Clear Cache

```swift
ImageCache.default.clearMemoryCache()
ImageCache.default.clearDiskCache()
```

## Avatar / Profile Images

For circular user avatars:

```swift
KFImage(user.avatarURL)
    .placeholder {
        Circle()
            .fill(Color(AppTheme.Colors.surfaceSecondary))
            .overlay {
                Text(user.initials)
                    .font(AppTheme.Typography.caption)
                    .foregroundStyle(Color(AppTheme.Colors.textSecondary))
            }
    }
    .resizable()
    .aspectRatio(contentMode: .fill)
    .frame(width: 44, height: 44)
    .clipShape(Circle())
```

## Image Containment

For correct image sizing within containers, use `.resizable()` + `.aspectRatio(contentMode:)` + a frame or container. See the `swiftui` skill for image containment/clip patterns — do not duplicate those patterns here.
