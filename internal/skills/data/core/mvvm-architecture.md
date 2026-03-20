---
description: "MVVM architecture rules - @Observable ViewModels with in-memory data by default, Loadable<T> for async state, View responsibilities, data access patterns"
---
# MVVM Architecture Rules

## Loadable Enum (Required for Async State)

Every app that performs async operations MUST include this enum as `Shared/Loadable.swift`:

```swift
// Shared/Loadable.swift
enum Loadable<T> {
    case notInitiated
    case loading
    case success(T)
    case failure(Error)

    var value: T? {
        if case .success(let v) = self { return v }
        return nil
    }

    var error: Error? {
        if case .failure(let e) = self { return e }
        return nil
    }

    var isLoading: Bool {
        if case .loading = self { return true }
        return false
    }
}
```

**Rule**: All async operations MUST use `Loadable<T>` for state tracking — never use `var isLoading: Bool` + `var errorMessage: String?`.

## ViewModel Pattern (In-Memory Default)
Every ViewModel follows this exact pattern:

```swift
import SwiftUI

@Observable
@MainActor
class NotesListViewModel {
    var notes: [Note] = Note.sampleData
    var searchText = ""

    var filteredNotes: [Note] {
        if searchText.isEmpty { return notes }
        return notes.filter { $0.title.localizedCaseInsensitiveContains(searchText) }
    }

    func addNote(title: String) {
        let note = Note(title: title)
        notes.insert(note, at: 0)
    }

    func deleteNote(_ note: Note) {
        notes.removeAll { $0.id == note.id }
    }
}
```

## ViewModel Pattern (Async Data)
When a ViewModel loads data asynchronously, use `Loadable<T>` and start loading in `init()`:

```swift
import SwiftUI

@Observable
@MainActor
class NotesListViewModel {
    var notes: Loadable<[Note]> = .loading  // Start as .loading — init fires immediately
    var searchText = ""

    init() {
        Task { await loadNotes() }
    }

    var filteredNotes: [Note] {
        guard let allNotes = notes.value else { return [] }
        if searchText.isEmpty { return allNotes }
        return allNotes.filter { $0.title.localizedCaseInsensitiveContains(searchText) }
    }

    func loadNotes() async {
        notes = .loading
        do {
            notes = .success(try await fetchNotes())
        } catch {
            notes = .failure(error)
        }
    }
}
```

**Init Loading Rule**: Use `init() { Task { await load() } }` for first data load. Do NOT use `.task` or `.onAppear` for initial loads — they fire on appear, not on creation, causing a flash of empty UI before data arrives.

## ViewModel Rules
- Annotate with `@Observable` and `@MainActor`
- ViewModel files contain **ONLY** the `@Observable` class — no other type declarations
- Handle business logic: CRUD, filtering, sorting, validation
- Initialize data arrays from model's `static sampleData` so app looks alive on first launch
- Use `Loadable<T>` for any property populated by async operations — never separate `isLoading` + `error` booleans
- May import `SwiftUI` and `UIKit` when needed for framework types, animation helpers, or required platform bridges
- Must NOT contain Views, `UIView`/`UIViewController` declarations, `body`, or `#Preview`

## Tab ViewModel Stability
**NEVER** create a ViewModel inside a tab content view — SwiftUI recreates `@State` on every tab switch, causing data reloads.

Create all tab ViewModels at the MainView level and pass them down:
```swift
struct MainView: View {
    @State private var roomsVM = RoomsViewModel()
    @State private var profileVM = ProfileViewModel()

    var body: some View {
        TabView {
            Tab("Rooms", systemImage: "bubble.left.and.bubble.right") {
                RoomsView(viewModel: roomsVM)
            }
            Tab("Profile", systemImage: "person") {
                ProfileView(viewModel: profileVM)
            }
        }
    }
}
```

Child tab views accept the ViewModel as a parameter:
```swift
struct RoomsView: View {
    var viewModel: RoomsViewModel
    var body: some View { ... }
}
```

## CRITICAL: Loadable State Handling
Every view that displays async data MUST `switch` on ALL 4 Loadable cases:
1. `.loading` → `ProgressView`
2. `.success(data)` where data is empty → `ContentUnavailableView` with action
3. `.success(data)` → content view
4. `.failure(error)` → error view with retry button

Never skip any case. Never use `if let` to unwrap only the success case.

## View Responsibilities
- Views own `@State` for **local UI state** (sheet presented, text field binding, animation flags)
- Views reference ViewModels via `@State var viewModel = SomeViewModel()` (or accept as parameter in tab views)
- Every View file **MUST** include a `#Preview` block with sample data

## Data Access Patterns
| What | Where | How |
|------|-------|-----|
| App data (default) | ViewModel | In-memory arrays initialized from sampleData |
| Async data | ViewModel | `Loadable<T>` enum — `.notInitiated` → `.loading` → `.success(T)` / `.failure(Error)` |
| Simple flags/settings | View or ViewModel | `@AppStorage` |
| Transient UI state | View | `@State` |
| Persistent data (only if user asks) | ViewModel | SwiftData `@Query` / `modelContext` |
