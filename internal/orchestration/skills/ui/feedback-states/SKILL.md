---
name: "feedback-states"
description: "Feedback state patterns: loading indicators, error handling UI, success confirmations, disabled states, skeleton views. Use when adding loading spinners, error alerts, success feedback, or managing empty/error/loading view states. Triggers: ProgressView, loading, error, alert, empty state, skeleton, disabled, feedback."
---
# Feedback States

## MANDATORY: Every Async View Must Handle All 4 States

Every view displaying async data MUST use a `switch` on `Loadable<T>` covering ALL 4 states. This is not optional.

```swift
switch viewModel.items {
case .notInitiated, .loading:
    ProgressView("Loading...")
case .success(let items) where items.isEmpty:
    ContentUnavailableView("No Items Yet", systemImage: "tray",
        description: Text("Tap + to create your first item"))
case .success(let items):
    List(items) { item in ItemRow(item: item) }
case .failure(let error):
    ContentUnavailableView {
        Label("Load Failed", systemImage: "exclamationmark.triangle")
    } description: {
        Text(error.localizedDescription)
    } actions: {
        Button("Retry") { Task { await viewModel.loadItems() } }
    }
}
```

**Rules**:
- NEVER use `if let` to unwrap only the success case — all 4 states must be handled
- Empty state MUST use `ContentUnavailableView` with an action button
- Error state MUST include a retry button
- Loading state MUST show `ProgressView`

## Upload / Mutation State Handling

Every mutation button (save, delete, upload, send) MUST:
1. Disable the trigger while in-progress
2. Show an inline spinner replacing the button label
3. Provide success/failure feedback after completion

```swift
Button {
    Task { await viewModel.save() }
} label: {
    if viewModel.saveState.isLoading {
        ProgressView()
            .controlSize(.small)
    } else {
        Text("Save")
    }
}
.disabled(viewModel.saveState.isLoading)
```

LOADING PATTERNS:

1. Inline button spinner (action on single element):
```swift
Button {
    Task { await save() }
} label: {
    if saveState.isLoading {
        ProgressView()
            .controlSize(.small)
    } else {
        Text("Save")
    }
}
.disabled(saveState.isLoading)
```

2. Full-screen loading (initial data load using Loadable):
```swift
switch viewModel.items {
case .notInitiated, .loading:
    ProgressView("Loading...")
case .success(let items):
    ContentListView(items: items)
case .failure(let error):
    ErrorView(error: error) {
        Task { await viewModel.loadItems() }
    }
}
```

3. Skeleton loading (content placeholders):
```swift
ForEach(Item.sampleData) { item in
    ItemRow(item: item)
}
.redacted(reason: .placeholder)
```

4. Pull-to-refresh (list content):
```swift
List { ... }
    .refreshable { await viewModel.refresh() }
```

5. Overlay loading (blocking operation):
```swift
.overlay {
    if operationState.isLoading {
        ZStack {
            Color.black.opacity(0.3)
            ProgressView()
                .controlSize(.large)
                .tint(.white)
        }
        .ignoresSafeArea()
    }
}
```

LOADING RULES:
- Show indicator for operations > 300ms.
- ALWAYS disable the triggering button while loading (prevents double-taps).
- Never block the entire UI for a partial operation — use inline spinner.
- Match loading style to scope: button-level → inline, screen-level → full-screen.
- Use `Loadable<T>` for all async state — never `var isLoading: Bool` + `var errorMessage: String?`.

ERROR HANDLING UI:

1. Inline validation (below form fields):
```swift
if let error = emailError {
    HStack(spacing: 4) {
        Image(systemName: "exclamationmark.circle.fill")
        Text(error)
    }
    .font(AppTheme.Fonts.caption)
    .foregroundStyle(AppTheme.Colors.error)
}
```

2. Alert for blocking errors (require user acknowledgment):
```swift
.alert("Error", isPresented: $showError) {
    Button("Retry") { Task { await retry() } }
    Button("Cancel", role: .cancel) { }
} message: {
    Text(errorMessage)
}
```

3. Banner for non-blocking errors (dismissible):
```swift
if let error = bannerError {
    HStack {
        Image(systemName: "exclamationmark.triangle.fill")
            .foregroundStyle(AppTheme.Colors.warning)
        Text(error)
            .font(AppTheme.Fonts.subheadline)
        Spacer()
        Button("Dismiss") { bannerError = nil }
            .font(AppTheme.Fonts.caption)
    }
    .padding(AppTheme.Spacing.small)
    .background(.orange.opacity(0.1))
    .clipShape(RoundedRectangle(cornerRadius: 8))
    .padding(.horizontal, AppTheme.Spacing.medium)
}
```

ERROR HANDLING RULES:
- Inline validation: show immediately as user types or on field blur.
- Alert: use for errors that block progress (network failure, permission denied).
- Banner: use for non-critical errors (sync failed, partial data).
- ALWAYS provide a retry path — never leave users stuck.
- Error messages: describe what happened + what the user can do.

SUCCESS FEEDBACK:
- Haptic: UINotificationFeedbackGenerator().notificationOccurred(.success).
- Visual: brief animation (checkmark, scale bounce, color flash).
- NEVER use modal alert for success — too disruptive.
- Subtle confirmation: toast, inline checkmark, or haptic alone.
```swift
// Brief success animation
withAnimation(.spring(response: 0.3)) {
    showSuccess = true
}
DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
    withAnimation { showSuccess = false }
}
```

DISABLED STATE:
- .disabled(condition) — SwiftUI auto-handles opacity reduction.
- Always explain WHY something is disabled (tooltip, caption text, or label).
- Example: "Fill in all required fields to continue" below a disabled button.
- Don't hide actions — show them disabled with explanation.

NETWORK/SYSTEM ERROR PATTERN (ViewModel with Loadable):
```swift
@MainActor @Observable
class ItemViewModel {
    var items: Loadable<[Item]> = .notInitiated

    func loadItems() async {
        items = .loading
        do {
            items = .success(try await fetchItems())
        } catch {
            items = .failure(error)
        }
    }

    var userFacingError: String? {
        if case .failure = items {
            return "Couldn't load items. Pull to refresh to try again."
        }
        return nil
    }
}
```

EMPTY VS ERROR VS LOADING (switch on Loadable):
```swift
switch viewModel.items {
case .notInitiated, .loading:
    ProgressView("Loading...")
case .success(let items) where items.isEmpty:
    ContentUnavailableView("No Items Yet", systemImage: "tray",
        description: Text("Tap + to create your first item"))
case .success(let items):
    List(items) { item in ItemRow(item: item) }
case .failure(let error):
    ContentUnavailableView {
        Label("Load Failed", systemImage: "exclamationmark.triangle")
    } description: {
        Text(error.localizedDescription)
    } actions: {
        Button("Retry") { Task { await viewModel.loadItems() } }
    }
}
```

- These are four distinct states — never conflate them.
- `Loadable<T>` makes each state explicit and compiler-enforced via `switch`.
