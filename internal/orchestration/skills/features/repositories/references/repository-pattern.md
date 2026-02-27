# Repository Pattern

## Contents
- Protocol definition
- Concrete implementation with DTO
- Naming conventions
- ViewModel consumption
- Composition root

## Protocol Definition

Repository protocols define the data access contract. Methods are async/throws and return domain models only.

```swift
// Repositories/Users/UserRepository.swift
protocol UserRepository {
    func getAll() async throws -> [User]
    func get(id: UUID) async throws -> User
    func create(username: String, email: String) async throws -> User
    func update(_ user: User) async throws
    func delete(id: UUID) async throws
}
```

Rules:
- One protocol per entity
- Return domain models, never DTOs
- Create methods take individual parameters (not the full model — ID is auto-generated)
- Update methods take the domain model (ID included for the WHERE clause)

## Concrete Implementation

Each concrete implementation file contains four things: the DTO, the insert DTO, the domain model mapping extension, and the implementation struct.

```swift
// Repositories/Users/SupabaseUserRepository.swift

import Supabase

// DTO — matches database schema exactly. NEVER used outside this file.
private struct UserDTO: Codable {
    let id: UUID
    let username: String
    let email: String
    let status: String
    let avatar_url: String?
    let created_at: Date
}

// Insert DTO — omits auto-generated fields (id, created_at)
private struct InsertUserDTO: Encodable {
    let username: String
    let email: String
}

// Mapping — domain model init from DTO
extension User {
    init(dto: UserDTO) {
        self.init(
            id: dto.id,
            username: dto.username,
            email: dto.email,
            status: UserStatus(rawValue: dto.status) ?? .unknown,
            avatarURL: dto.avatar_url.flatMap(URL.init(string:)),
            createdAt: dto.created_at
        )
    }
}

struct SupabaseUserRepository: UserRepository {
    private let client = SupabaseService.shared.client

    func getAll() async throws -> [User] {
        let dtos: [UserDTO] = try await client.from("users")
            .select()
            .execute()
            .value
        return dtos.map(User.init(dto:))
    }

    func get(id: UUID) async throws -> User {
        let dto: UserDTO = try await client.from("users")
            .select()
            .eq("id", value: id)
            .single()
            .execute()
            .value
        return User(dto: dto)
    }

    func create(username: String, email: String) async throws -> User {
        let dto: UserDTO = try await client.from("users")
            .insert(InsertUserDTO(username: username, email: email))
            .select()
            .single()
            .execute()
            .value
        return User(dto: dto)
    }

    func update(_ user: User) async throws {
        try await client.from("users")
            .update(["username": user.username, "email": user.email])
            .eq("id", value: user.id)
            .execute()
    }

    func delete(id: UUID) async throws {
        try await client.from("users")
            .delete()
            .eq("id", value: id)
            .execute()
    }
}
```

## Naming Conventions

| Type | Pattern | Example |
|------|---------|---------|
| Protocol | `{Entity}Repository` | `UserRepository`, `PostRepository` |
| Concrete | `Supabase{Entity}Repository` | `SupabaseUserRepository` |
| DTO | `{Entity}DTO` | `UserDTO`, `PostDTO` |
| Insert DTO | `Insert{Entity}DTO` | `InsertUserDTO` |
| Protocol file | `Repositories/{Entity}/{Entity}Repository.swift` | `Repositories/Users/UserRepository.swift` |
| Concrete file | `Repositories/{Entity}/Supabase{Entity}Repository.swift` | `Repositories/Users/SupabaseUserRepository.swift` |

## ViewModel Consumption

ViewModels depend on the protocol type — never the concrete implementation.

```swift
@Observable @MainActor
class UserListViewModel {
    private let userRepository: UserRepository  // Protocol type — NEVER concrete
    var users: [User] = []                      // Domain model — NEVER DTO
    var error: Error?

    init(userRepository: UserRepository) {
        self.userRepository = userRepository
    }

    func loadUsers() async {
        do {
            users = try await userRepository.getAll()
        } catch {
            self.error = error
        }
    }

    func deleteUser(id: UUID) async {
        do {
            try await userRepository.delete(id: id)
            users.removeAll { $0.id == id }
        } catch {
            self.error = error
        }
    }
}
```

## Composition Root

The App entry point is the ONLY place that knows about concrete repository types. It creates them and passes them down via init injection.

```swift
@main
struct MyApp: App {
    var body: some Scene {
        WindowGroup {
            let userRepository = SupabaseUserRepository()
            let viewModel = UserListViewModel(userRepository: userRepository)
            UserListView(viewModel: viewModel)
        }
    }
}
```

Rules:
- Concrete types (`SupabaseUserRepository`) appear ONLY in the App file
- ViewModels and Views never import or reference concrete types
- Pass repositories through init parameters, not environment or singletons
