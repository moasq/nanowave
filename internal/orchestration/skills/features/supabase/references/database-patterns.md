# Database Patterns

## Contents
- Codable models (NOT @Model)
- CRUD operations
- Insert with return value
- Filtering and ordering
- Foreign key joins
- Realtime subscriptions

## Codable Models

> **Note:** When the `repositories` skill is loaded, these Codable structs become DTOs inside concrete repository files (e.g. `SupabaseUserRepository.swift`), and domain models in `Models/` use Swift-native types (enums, URL, Date) instead of raw Codable types. See the repositories skill for the full pattern.

Models use `Codable` — NOT `@Model`. Supabase is the persistence layer.
Use `CodingKeys` to map Swift camelCase to Postgres snake_case.

```swift
struct Todo: Codable, Identifiable {
    let id: Int
    let title: String
    let isComplete: Bool
    let userId: UUID

    enum CodingKeys: String, CodingKey {
        case id, title
        case isComplete = "is_complete"
        case userId = "user_id"
    }
}
```

Use separate `Encodable` structs for inserts (omit auto-generated fields like `id`, `created_at`):

```swift
struct NewTodo: Encodable {
    let title: String
    let isComplete: Bool
    let userId: UUID

    enum CodingKeys: String, CodingKey {
        case title
        case isComplete = "is_complete"
        case userId = "user_id"
    }
}
```

## CRUD Operations

All operations are typed and async/await.

```swift
let client = SupabaseService.shared.client

// Read all
let todos: [Todo] = try await client.from("todos")
    .select()
    .execute()
    .value

// Read with filter
let incomplete: [Todo] = try await client.from("todos")
    .select()
    .eq("is_complete", value: false)
    .order("created_at", ascending: false)
    .execute()
    .value

// Insert (fire and forget)
try await client.from("todos")
    .insert(NewTodo(title: "Buy groceries", isComplete: false, userId: userId))
    .execute()

// Insert and return created object
let created: Todo = try await client.from("todos")
    .insert(NewTodo(title: "Buy groceries", isComplete: false, userId: userId))
    .select()
    .single()
    .execute()
    .value

// Insert multiple rows
let created: [Todo] = try await client.from("todos")
    .insert([
        NewTodo(title: "Task 1", isComplete: false, userId: userId),
        NewTodo(title: "Task 2", isComplete: false, userId: userId),
    ])
    .select()
    .execute()
    .value

// Update
try await client.from("todos")
    .update(["is_complete": true])
    .eq("id", value: todoId)
    .execute()

// Delete
try await client.from("todos")
    .delete()
    .eq("id", value: todoId)
    .execute()

// Upsert (insert or update on conflict)
try await client.from("todos")
    .upsert(todo, onConflict: "id")
    .execute()
```

## Filtering and Ordering

```swift
// Multiple filters (chained = AND)
let results: [Item] = try await client.from("items")
    .select()
    .eq("category", value: "books")
    .gte("price", value: 10)
    .order("price", ascending: true)
    .limit(20)
    .execute()
    .value

// OR filter
let results: [Todo] = try await client.from("todos")
    .select()
    .or("is_complete.eq.true,priority.gte.5")
    .execute()
    .value

// IN filter
let results: [Todo] = try await client.from("todos")
    .select()
    .in("id", values: [1, 2, 3])
    .execute()
    .value

// NULL check
let results: [Todo] = try await client.from("todos")
    .select()
    .is("deleted_at", value: nil)
    .execute()
    .value

// Text search
let results: [Post] = try await client.from("posts")
    .select()
    .textSearch("title", query: "swift", config: "english")
    .execute()
    .value

// Count without data
let count = try await client.from("todos")
    .select("*", head: true, count: .exact)
    .execute()
    .count
```

## Foreign Key Joins

Query related tables using PostgREST relationship syntax:

```swift
struct TodoWithUser: Codable {
    let id: Int
    let title: String
    let user: User

    struct User: Codable {
        let id: UUID
        let email: String
    }
}

// Join with foreign key (alias:table(columns))
let todos: [TodoWithUser] = try await client.from("todos")
    .select("id, title, user:users(id, email)")
    .execute()
    .value

// Nested join — posts with author profile
let posts: [PostWithAuthor] = try await client.from("posts")
    .select("*, author:profiles(*)")
    .order("created_at", ascending: false)
    .execute()
    .value
```

## Realtime Subscriptions

Listen for database changes in real time.

```swift
let channel = client.channel("my-channel")

// Subscribe to all change types
let subscription = channel.onPostgresChange(
    AnyAction.self,
    schema: "public",
    table: "todos"
) { action in
    switch action {
    case .insert(let insert):
        print("New: \(insert.record)")
    case .update(let update):
        print("Updated: \(update.record), was: \(update.oldRecord)")
    case .delete(let delete):
        print("Deleted: \(delete.oldRecord)")
    }
}

// Subscribe to specific events with filter
let insertSub = channel.onPostgresChange(
    InsertAction.self,
    schema: "public",
    table: "todos",
    filter: "user_id=eq.\(userId)"
) { insert in
    print("New todo: \(insert.record)")
}

// Start listening
try await channel.subscribeWithError()

// Clean up when done
await channel.unsubscribe()
await client.removeChannel(channel)
```
