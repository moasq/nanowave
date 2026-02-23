---
name: "camera"
description: "Camera and photo capture: PhotosPicker, AVCaptureSession, UIViewControllerRepresentable wrapper, permissions. Use when implementing app features related to camera."
---
# Camera

CAMERA & PHOTOS:
- PhotosPicker (PhotosUI) for gallery selection — no permissions needed for limited access
- For camera capture: AVCaptureSession + AVCapturePhotoOutput + UIViewControllerRepresentable wrapper
- Camera requires NSCameraUsageDescription permission (add CONFIG_CHANGES)
- Full photo library requires NSPhotoLibraryUsageDescription
- Use @State private var selectedItem: PhotosPickerItem? with .onChange to load
- Load image: try await item.loadTransferable(type: Data.self)
