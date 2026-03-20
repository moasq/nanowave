# Storage Service

Higher-level storage patterns for generated apps. For raw Supabase Storage API calls (upload, download, URLs, list), see `storage-patterns.md`.

## Contents
- StorageService singleton
- Image compression before upload
- PhotosPicker → upload flow
- ViewModel upload state management
- Composition with repositories

## StorageService

Place at `Services/Storage/StorageService.swift`. Wraps raw Supabase storage calls with compression and unique filename generation.

```swift
import Foundation
import Supabase
import UIKit

@Observable
final class StorageService {
    static let shared = StorageService()
    private let storage: SupabaseStorageClient

    private init() {
        storage = SupabaseService.shared.client.storage
    }

    /// Upload image data with automatic compression and unique filename.
    func uploadImage(
        _ data: Data,
        bucket: String,
        folder: String,
        upsert: Bool = false
    ) async throws -> URL {
        let compressed = compressImage(data)
        let userId = try await SupabaseService.shared.client.auth.session.user.id
        let filename = "\(userId)/\(folder)/\(UUID().uuidString).jpg"

        try await storage.from(bucket)
            .upload(
                filename,
                data: compressed,
                options: FileOptions(
                    cacheControl: "3600",
                    contentType: "image/jpeg",
                    upsert: upsert
                )
            )

        return try storage.from(bucket)
            .getPublicURL(path: filename)
    }

    /// Delete a file from storage.
    func deleteFile(path: String, bucket: String) async throws {
        try await storage.from(bucket).remove(paths: [path])
    }
}
```

## Image Compression

Built into StorageService as a private helper. Resizes to max 2048px, then iterates JPEG quality from 0.8 down to 0.1 until under 5 MB.

```swift
// Inside StorageService
private func compressImage(_ data: Data) -> Data {
    guard let image = UIImage(data: data) else { return data }

    let resized = resizeIfNeeded(image, maxDimension: 2048)

    var quality: CGFloat = 0.8
    var compressed = resized.jpegData(compressionQuality: quality)

    while let current = compressed,
          current.count > 5_000_000,
          quality > 0.1 {
        quality -= 0.1
        compressed = resized.jpegData(compressionQuality: quality)
    }

    return compressed ?? data
}

private func resizeIfNeeded(_ image: UIImage, maxDimension: CGFloat) -> UIImage {
    let size = image.size
    let maxSide = max(size.width, size.height)
    guard maxSide > maxDimension else { return image }

    let scale = maxDimension / maxSide
    let newSize = CGSize(width: size.width * scale, height: size.height * scale)
    let renderer = UIGraphicsImageRenderer(size: newSize)
    return renderer.image { _ in
        image.draw(in: CGRect(origin: .zero, size: newSize))
    }
}
```

## PhotosPicker → Upload Flow

Use `PhotosPicker` bound to a `PhotosPickerItem?` on the ViewModel. Extract data with `loadTransferable`, then call `StorageService.uploadImage()`.

```swift
import PhotosUI
import SwiftUI

struct ProfilePhotoSection: View {
    @Bindable var viewModel: ProfileViewModel

    var body: some View {
        PhotosPicker(
            selection: $viewModel.selectedPhotoItem,
            matching: .images
        ) {
            avatarContent
        }
        .onChange(of: viewModel.selectedPhotoItem) {
            Task { await viewModel.uploadPhoto() }
        }
    }

    private var avatarContent: some View {
        // Show local preview while uploading, remote image otherwise
        Group {
            if let preview = viewModel.localPreviewImage {
                Image(uiImage: preview)
                    .resizable()
                    .scaledToFill()
            } else {
                // AsyncImage or placeholder
            }
        }
        .frame(width: 80, height: 80)
        .clipShape(Circle())
        .overlay { if viewModel.isUploading { ProgressView() } }
    }
}
```

## ViewModel Upload Pattern

Manage upload state in the ViewModel: loading flag, local preview, error handling. Coordinate StorageService (file upload) with repository (database update).

```swift
import PhotosUI

@Observable
final class ProfileViewModel {
    var selectedPhotoItem: PhotosPickerItem?
    var localPreviewImage: UIImage?
    var isUploading = false

    private let userRepository: UserRepository
    private var user: User

    init(user: User, userRepository: UserRepository) {
        self.user = user
        self.userRepository = userRepository
    }

    func uploadPhoto() async {
        guard let item = selectedPhotoItem else { return }
        isUploading = true
        defer { isUploading = false }

        do {
            guard let data = try await item.loadTransferable(type: Data.self) else { return }

            // Show local preview immediately
            localPreviewImage = UIImage(data: data)

            // Upload → get public URL
            let url = try await StorageService.shared.uploadImage(
                data,
                bucket: "avatars",
                folder: "profiles",
                upsert: true
            )

            // Update database record with new URL
            user.imageURL = url
            try await userRepository.update(user)
        } catch {
            localPreviewImage = nil
            // Handle error (show alert, etc.)
        }
    }
}
```

## Composition with Repositories

Storage URLs are stored in database tables as `TEXT` columns. The domain model uses `URL?`.

- **DTO** (in repository file): `image_url: String?` — matches the DB column
- **Domain model** (in Models/): `imageURL: URL?` — Swift-native type
- **Mapping**: `init(dto:)` converts `String?` → `URL?` via `URL(string:)`

StorageService handles file operations only. Repositories handle database records. The ViewModel coordinates both:

1. Call `StorageService.uploadImage()` → receives public URL
2. Set URL on domain model
3. Call `repository.update(model)` → persists URL to database
