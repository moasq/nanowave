# DTO Mapping

## Contents
- DTO rules
- Domain model rules
- Mapping patterns
- Enum mapping
- Common type conversions

## DTO Rules

DTOs match the database schema exactly. They exist only inside concrete repository files.

- Match database column names exactly (snake_case fields or CodingKeys)
- Use only Codable-friendly types: `String`, `Int`, `Double`, `Bool`, `UUID`, `Date`, `String?`
- NEVER use Swift enums — use `String` for status fields (backend returns strings)
- NEVER use `URL` — use `String?` for URL fields
- NEVER add computed properties or business logic
- Mark optional what the DB allows as NULL
- Mark DTOs as `private` to prevent use outside the file

```swift
// DTO — inside SupabasePostRepository.swift
private struct PostDTO: Codable {
    let id: UUID
    let title: String
    let body: String
    let status: String          // "draft", "published", "archived"
    let author_id: UUID
    let image_url: String?
    let like_count: Int
    let created_at: Date
}
```

Insert DTOs omit auto-generated fields:

```swift
private struct InsertPostDTO: Encodable {
    let title: String
    let body: String
    let author_id: UUID
}
```

## Domain Model Rules

Domain models live in `Models/` and use Swift-native types.

- Use Swift enums for status/category fields
- Use `URL` for link fields
- Use `Date` for timestamps
- Conform to `Identifiable` (NOT `Codable` — domain models don't need serialization)
- Add computed properties for display logic
- Minimize optionals — provide defaults where semantically appropriate
- Include `static let sample` for SwiftUI previews

```swift
// Models/Post.swift
enum PostStatus: String {
    case draft, published, archived, unknown
}

struct Post: Identifiable {
    let id: UUID
    let title: String
    let body: String
    let status: PostStatus      // Proper Swift enum
    let authorID: UUID
    let imageURL: URL?          // Proper URL type
    let likeCount: Int
    let createdAt: Date

    static let sample = Post(
        id: UUID(),
        title: "Sample Post",
        body: "This is a sample post.",
        status: .published,
        authorID: UUID(),
        imageURL: nil,
        likeCount: 42,
        createdAt: .now
    )
}
```

## Mapping Patterns

The `init(dto:)` extension on the domain model lives inside the concrete repository file, co-located with the DTO.

```swift
// Inside SupabasePostRepository.swift
extension Post {
    init(dto: PostDTO) {
        self.init(
            id: dto.id,
            title: dto.title,
            body: dto.body,
            status: PostStatus(rawValue: dto.status) ?? .unknown,   // String → enum
            authorID: dto.author_id,                                 // snake_case → camelCase
            imageURL: dto.image_url.flatMap(URL.init(string:)),     // String? → URL?
            likeCount: dto.like_count,
            createdAt: dto.created_at
        )
    }
}
```

Usage in repository methods:

```swift
func getAll() async throws -> [Post] {
    let dtos: [PostDTO] = try await client.from("posts")
        .select()
        .execute()
        .value
    return dtos.map(Post.init(dto:))
}
```

## Enum Mapping

- Always include an `.unknown` case for forward compatibility
- Use `rawValue` init with fallback: `PostStatus(rawValue: dto.status) ?? .unknown`
- Never force-unwrap enum init

```swift
// Good — safe with fallback
status: PostStatus(rawValue: dto.status) ?? .unknown

// Bad — will crash on unexpected values
status: PostStatus(rawValue: dto.status)!
```

## Common Type Conversions

| DTO Type | Domain Type | Mapping |
|----------|-------------|---------|
| `String` | enum | `MyEnum(rawValue: dto.field) ?? .unknown` |
| `String?` | `URL?` | `dto.field.flatMap(URL.init(string:))` |
| `String` | `UUID` | Already `UUID` in DTO if column is `uuid` |
| `Int` | `Int` | Direct assignment |
| `Date` | `Date` | Direct assignment (Supabase SDK handles ISO 8601) |
| snake_case field | camelCase property | Direct assignment in `init(dto:)` |
