---
name: "repositories"
description: "Repository pattern: protocol-based data access, DTO-to-domain mapping, and clean layer separation. Use when implementing app features related to repositories."
---
# Repository Pattern

## Architecture Overview

The repository pattern separates data access from business logic. ViewModels depend on protocol abstractions — never concrete implementations or database types.

Data flow: `Supabase API → DTO (Codable) → Repository (maps) → Domain Model → ViewModel → View`

## Core Concepts

### Repository Protocols
- Define async/throws methods returning domain models
- Live in `Repositories/{Entity}/{Entity}Repository.swift` (e.g. `Repositories/Users/UserRepository.swift`)
- One protocol per entity (UserRepository, PostRepository)

### DTOs (Data Transfer Objects)
- Match database schema exactly (snake_case, Codable-friendly types)
- Live INSIDE concrete repository files — never exposed outside
- Separate insert DTOs omit auto-generated fields (id, created_at)

### Domain Models
- Live in `Models/` — use Swift-native types (enums, URL, Date)
- Conform to `Identifiable` (NOT `Codable` — they don't need serialization)
- Include `static let sample` for SwiftUI previews

### Concrete Implementations
- Live in `Repositories/{Entity}/Supabase{Entity}Repository.swift` (e.g. `Repositories/Users/SupabaseUserRepository.swift`)
- Contain: DTO struct, insert DTO, `init(dto:)` mapping extension, implementation
- Use `SupabaseService.shared.client` for all database calls

## Strict Rules

- **ViewModels use protocols only** — never `SupabaseUserRepository`, always `UserRepository`
- **DTOs never leak** — no DTO type appears in protocol signatures, ViewModels, or Views
- **Protocol methods return domain models** — `func getAll() async throws -> [User]`
- **Create methods take individual parameters** — not domain models (ID is auto-generated)
- **DI via init injection** — concrete types only appear in App entry point (composition root)

## Composition Root

The `@main` App struct is the only place that creates concrete repository instances and passes them down:

```swift
@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup {
            let userRepo = SupabaseUserRepository()
            ContentView(viewModel: UserListViewModel(userRepository: userRepo))
        }
    }
}
```

## Composition with Other Skills

- **Supabase skill** — provides the API patterns used inside concrete repository implementations
- **Authentication skill** — AuthService handles auth state; repositories handle data access
- When all three are loaded, the builder wires them: concrete repos use SupabaseService, ViewModels receive protocol abstractions

## References

- [Repository Pattern](references/repository-pattern.md) — protocol definitions, concrete implementations, ViewModel consumption
- [DTO Mapping](references/dto-mapping.md) — DTO structure, domain models, mapping patterns
