---
name: "navigation"
description: "Navigation architecture: NavigationStack, NavigationSplitView, TabView, sheets, fullScreenCover, type-safe routing, programmatic navigation. Use when setting up app navigation, adding screens, presenting modals, or building tab-based flows. Triggers: NavigationStack, NavigationLink, TabView, sheet, fullScreenCover, NavigationPath, navigationDestination."
---
# Navigation Pattern Rules

## Pattern Selection Guide

| Pattern | When to Use |
|---------|-------------|
| `NavigationStack` | Hierarchical drill-down (list → detail → edit) |
| `TabView` with `Tab` API | 3+ distinct top-level peer sections |
| `.sheet(item:)` | Creation forms, secondary actions, settings |
| `.fullScreenCover` | Immersive experiences (media player, onboarding) |
| `NavigationStack` + `.sheet` | Most MVPs with 2-4 features |

## NavigationStack
Use for hierarchical navigation with a back button:

```swift
NavigationStack {
    List(items) { item in
        NavigationLink(value: item) {
            ItemRow(item: item)
        }
    }
    .navigationDestination(for: Item.self) { item in
        ItemDetailView(item: item)
    }
    .navigationTitle("Items")
}
```

## TabView with Tab API
Use when the app has 3+ distinct, peer-level sections:

```swift
TabView {
    Tab("Home", systemImage: "house") {
        HomeView()
    }
    Tab("Search", systemImage: "magnifyingglass") {
        SearchView()
    }
    Tab("Profile", systemImage: "person") {
        ProfileView()
    }
}
```

## Sheets
Choose the right sheet variant based on context:

- **`.sheet(item:)`** — for editing or viewing an existing item (the item drives the sheet)
- **`.sheet(isPresented:)`** — acceptable for creation forms and simple actions (no item yet)

```swift
// Editing/viewing an existing item — use item-driven
@State private var editingItem: Item?

.sheet(item: $editingItem) { item in
    EditItemView(item: item)
}

// Creating a new item — isPresented is fine
@State private var showAddItem = false

.sheet(isPresented: $showAddItem) {
    AddItemView()
}
```

## Full Screen Cover
Use for immersive content that should cover the entire screen:

```swift
@State private var showOnboarding = false

.fullScreenCover(isPresented: $showOnboarding) {
    OnboardingView()
}
```

## Type-Safe Routing
Always use `navigationDestination(for:)` for type-safe routing:

```swift
// Define route types
.navigationDestination(for: Note.self) { note in
    NoteDetailView(note: note)
}
.navigationDestination(for: Category.self) { category in
    CategoryView(category: category)
}
```

---

## NavigationStack — Type-Safe Navigation

```swift
struct ContentView: View {
    var body: some View {
        NavigationStack {
            List {
                NavigationLink("Profile", value: Route.profile)
                NavigationLink("Settings", value: Route.settings)
            }
            .navigationDestination(for: Route.self) { route in
                switch route {
                case .profile:
                    ProfileView()
                case .settings:
                    SettingsView()
                }
            }
        }
    }
}

enum Route: Hashable {
    case profile
    case settings
}
```

## Programmatic Navigation

```swift
struct ContentView: View {
    @State private var navigationPath = NavigationPath()

    var body: some View {
        NavigationStack(path: $navigationPath) {
            List {
                Button("Go to Detail") {
                    navigationPath.append(DetailRoute.item(id: 1))
                }
            }
            .navigationDestination(for: DetailRoute.self) { route in
                switch route {
                case .item(let id):
                    ItemDetailView(id: id)
                }
            }
        }
    }
}

enum DetailRoute: Hashable {
    case item(id: Int)
}
```

---

## Sheet Patterns

### Item-Driven Sheets (Preferred)

```swift
// Good - item-driven
@State private var selectedItem: Item?

var body: some View {
    List(items) { item in
        Button(item.name) {
            selectedItem = item
        }
    }
    .sheet(item: $selectedItem) { item in
        ItemDetailSheet(item: item)
    }
}

// Avoid - boolean flag requires separate state
@State private var showSheet = false
@State private var selectedItem: Item?
```

**Why**: `.sheet(item:)` automatically handles presentation state and avoids optional unwrapping.

### Sheets Own Their Actions

Sheets should handle their own dismiss and actions internally.

```swift
struct EditItemSheet: View {
    @Environment(\.dismiss) private var dismiss
    @Environment(DataStore.self) private var store

    let item: Item
    @State private var name: String
    @State private var isSaving = false

    init(item: Item) {
        self.item = item
        _name = State(initialValue: item.name)
    }

    var body: some View {
        NavigationStack {
            Form {
                TextField("Name", text: $name)
            }
            .navigationTitle("Edit Item")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button(isSaving ? "Saving..." : "Save") {
                        Task { await save() }
                    }
                    .disabled(isSaving || name.isEmpty)
                }
            }
        }
    }

    private func save() async {
        isSaving = true
        await store.updateItem(item, name: name)
        dismiss()
    }
}
```

---

## Popover

```swift
@State private var showPopover = false

Button("Show Popover") {
    showPopover = true
}
.popover(isPresented: $showPopover) {
    PopoverContentView()
        .presentationCompactAdaptation(.popover)
}
```

---

## Alert with Actions

```swift
.alert("Delete Item?", isPresented: $showAlert) {
    Button("Delete", role: .destructive) { deleteItem() }
    Button("Cancel", role: .cancel) { }
} message: {
    Text("This action cannot be undone.")
}
```

---

## Confirmation Dialog

```swift
.confirmationDialog("Choose an option", isPresented: $showDialog) {
    Button("Option 1") { handleOption1() }
    Button("Option 2") { handleOption2() }
    Button("Cancel", role: .cancel) { }
}
```

---

## iPad-Specific Patterns

### NavigationSplitView (PRIMARY pattern for iPad)

For ANY list-detail flow, use `NavigationSplitView`. It shows sidebar+detail on iPad and auto-collapses to `NavigationStack` on iPhone.

#### Two-column (most common)
```swift
@State private var selectedItem: Item?

NavigationSplitView {
    List(items, selection: $selectedItem) { item in
        NavigationLink(value: item) {
            ItemRow(item: item)
        }
    }
    .navigationTitle("Items")
} detail: {
    if let selectedItem {
        ItemDetailView(item: selectedItem)
    } else {
        ContentUnavailableView("Select an Item", systemImage: "doc")
    }
}
```

#### Three-column (sidebar categories)
```swift
@State private var selectedCategory: Category?
@State private var selectedItem: Item?

NavigationSplitView {
    List(categories, selection: $selectedCategory) { category in
        Label(category.name, systemImage: category.icon)
    }
    .navigationTitle("Categories")
} content: {
    if let selectedCategory {
        List(selectedCategory.items, selection: $selectedItem) { item in
            NavigationLink(value: item) { ItemRow(item: item) }
        }
        .navigationTitle(selectedCategory.name)
    }
} detail: {
    if let selectedItem {
        ItemDetailView(item: selectedItem)
    } else {
        ContentUnavailableView("Select an Item", systemImage: "doc")
    }
}
```

#### Column visibility control
```swift
@State private var columnVisibility: NavigationSplitViewVisibility = .automatic

NavigationSplitView(columnVisibility: $columnVisibility) {
    SidebarView()
} detail: {
    DetailView()
}
```

### TabView + NavigationSplitView

For apps with 3+ top-level sections on iPad, use `TabView` wrapping `NavigationSplitView` inside each tab:

```swift
TabView {
    Tab("Library", systemImage: "books.vertical") {
        NavigationSplitView {
            LibraryListView()
        } detail: {
            ContentUnavailableView("Select a Book", systemImage: "book")
        }
    }
    Tab("Search", systemImage: "magnifyingglass") {
        SearchView()
    }
}
```

### iPad Presentation Differences

#### Popovers (auto-adaptive)
```swift
.popover(isPresented: $showOptions) {
    OptionsView()
        .frame(minWidth: 250, minHeight: 300)
}
```
- iPad (regular width): floating popover anchored to source view
- iPhone (compact width): automatically becomes a sheet

#### Sheets on iPad
```swift
.sheet(isPresented: $showSheet) {
    SheetContent()
        .presentationDetents([.medium, .large])
}
```
- iPad wide: centered floating modal (detents are ignored)
- iPad narrow/split: edge-attached (detents are respected)

#### Confirmation dialogs
Always use `.confirmationDialog` — becomes popover on iPad, action sheet on iPhone:
```swift
.confirmationDialog("Options", isPresented: $showDialog) {
    Button("Edit") { }
    Button("Delete", role: .destructive) { }
    Button("Cancel", role: .cancel) { }
}
```

### iPad Navigation Rules
1. ALWAYS use `NavigationSplitView` for list-detail flows — never bare `NavigationStack` on iPad
2. ALWAYS provide a detail placeholder (`ContentUnavailableView`) for iPad empty state
3. Use `.popover()` for contextual actions — SwiftUI adapts automatically
4. NEVER check `UIDevice.current.userInterfaceIdiom` for navigation decisions
