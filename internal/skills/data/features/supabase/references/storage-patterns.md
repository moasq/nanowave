# Storage Patterns

This file covers the raw Supabase Storage API (upload, download, URLs, file management). For higher-level patterns — StorageService singleton, image compression, PhotosPicker → upload flow, ViewModel upload state — see `storage-service.md`.

## Contents
- Upload files
- Upload with return value
- Download files
- Download with image transformation
- Public URLs
- Signed URLs (time-limited)
- List and manage files

## Upload

```swift
let storage = SupabaseService.shared.client.storage

// Upload image data with content type
let response = try await storage.from("avatars")
    .upload(
        "user/avatar.png",
        data: imageData,
        options: FileOptions(
            cacheControl: "3600",
            contentType: "image/png",
            upsert: true
        )
    )
print("Uploaded to: \(response.fullPath)")

// Upload with user-scoped path (required by storage policies)
let userId = try await SupabaseService.shared.client.auth.session.user.id
let path = "\(userId)/\(UUID().uuidString).jpg"
try await storage.from("post-images")
    .upload(
        path,
        data: jpegData,
        options: FileOptions(contentType: "image/jpeg")
    )
```

Key rules:
- Always set `contentType` to the correct MIME type
- Use `upsert: true` when replacing existing files (e.g., avatar updates)
- Use `upsert: false` (default) for unique uploads (e.g., post images)
- Path must start with `{userId}/` to pass storage policies

## Download

```swift
// Download file data
let data = try await storage.from("documents")
    .download(path: "reports/quarterly-report.pdf")

// Download with image transformation (resize on the fly)
let thumbnailData = try await storage.from("images")
    .download(
        path: "photos/vacation.jpg",
        options: TransformOptions(
            width: 200,
            height: 200,
            resize: .cover
        )
    )

// Convert to UIImage
if let image = UIImage(data: data) {
    // Use image
}
```

## Public URL

For buckets with `public: true`. No authentication required to access.

```swift
let url = try storage.from("post-images")
    .getPublicURL(path: "user123/photo.jpg")

// With image transformation
let thumbnailURL = try storage.from("post-images")
    .getPublicURL(
        path: "user123/photo.jpg",
        options: TransformOptions(width: 400, height: 400)
    )
```

## Signed URL

For private buckets — generates a time-limited access URL.

```swift
// Single signed URL (valid for 1 hour)
let signedURL = try await storage.from("private-files")
    .createSignedURL(path: "document.pdf", expiresIn: 3600)

// Multiple signed URLs
let signedURLs = try await storage.from("private-files")
    .createSignedURLs(
        paths: ["doc1.pdf", "doc2.pdf", "doc3.pdf"],
        expiresIn: 3600
    )
```

## List and Manage Files

```swift
// List files in a folder
let files = try await storage.from("documents")
    .list(
        path: "reports",
        options: SearchOptions(limit: 100, offset: 0)
    )

for file in files {
    print("File: \(file.name), Size: \(file.metadata?.size ?? 0)")
}

// Check if file exists
let exists = try await storage.from("avatars")
    .exists(path: "user123/avatar.png")

// Move file
try await storage.from("documents")
    .move(from: "drafts/report.pdf", to: "published/report.pdf")

// Delete files
let deleted = try await storage.from("documents")
    .remove(paths: ["old-report.pdf", "draft.txt"])
```
