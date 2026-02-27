# Schema Setup

## Contents
- Backend-first execution order
- Table creation via MCP
- Column type mapping
- Foreign keys and constraints
- Triggers for automatic timestamps
- Profile table pattern (linked to auth)

## CRITICAL: Backend-First Execution Order

You MUST create ALL Supabase tables, RLS policies, and storage buckets BEFORE writing ANY Swift code. The order is:

1. Create tables with `mcp__supabase__execute_sql`
2. Enable RLS on every table
3. Create RLS policies for every table
4. Create storage buckets and their policies
5. Verify with `mcp__supabase__list_tables`
6. THEN write Swift code

Never write Swift code that references a table that doesn't exist yet.

## Creating Tables via MCP

Use `mcp__supabase__execute_sql` for all schema operations. Always use `IF NOT EXISTS` for idempotency.

### Profiles Table (Auth-Linked)

Every app with user accounts needs a profiles table linked to `auth.users`:

```sql
CREATE TABLE IF NOT EXISTS public.profiles (
  id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
  username TEXT UNIQUE NOT NULL,
  avatar_url TEXT,
  created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
  updated_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER profiles_updated_at
  BEFORE UPDATE ON public.profiles
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

Key rules:
- `id` is UUID referencing `auth.users(id)` — NOT auto-generated
- Use `ON DELETE CASCADE` so deleting a user cleans up the profile
- Always include `created_at` and `updated_at` with defaults

### Content Tables (User-Owned)

Tables for user-generated content (posts, notes, items):

```sql
CREATE TABLE IF NOT EXISTS public.posts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  author_id UUID NOT NULL REFERENCES public.profiles(id) ON DELETE CASCADE,
  image_url TEXT NOT NULL,
  caption TEXT,
  like_count INTEGER DEFAULT 0 NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
  updated_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_posts_author ON public.posts(author_id);
CREATE INDEX IF NOT EXISTS idx_posts_created ON public.posts(created_at DESC);

CREATE TRIGGER posts_updated_at
  BEFORE UPDATE ON public.posts
  FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

Key rules:
- `id` is `UUID DEFAULT gen_random_uuid()` for auto-generation
- Foreign key to `profiles(id)` with `ON DELETE CASCADE`
- Add indexes for columns used in WHERE/ORDER BY

### Join Tables (Relationships)

For many-to-many relationships (follows, likes, memberships):

```sql
CREATE TABLE IF NOT EXISTS public.follows (
  follower_id UUID NOT NULL REFERENCES public.profiles(id) ON DELETE CASCADE,
  followed_id UUID NOT NULL REFERENCES public.profiles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
  PRIMARY KEY (follower_id, followed_id),
  CHECK (follower_id != followed_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_followed ON public.follows(followed_id);
```

Key rules:
- Composite primary key prevents duplicates
- `CHECK` constraint prevents self-referential relationships
- Index the "reverse lookup" column for query performance

## Column Type Mapping

Map Swift types to PostgreSQL types:

| Swift Type | PostgreSQL Type | Notes |
|------------|----------------|-------|
| `UUID` | `UUID` | Use `DEFAULT gen_random_uuid()` for auto-gen |
| `String` | `TEXT` | Never use VARCHAR — TEXT is preferred in Postgres |
| `Int` | `INTEGER` | Use `BIGINT` for large counters |
| `Double` | `DOUBLE PRECISION` | |
| `Bool` | `BOOLEAN` | |
| `Date` | `TIMESTAMPTZ` | Always use timezone-aware timestamps |
| `URL` (as String) | `TEXT` | Store URLs as text |
| `[String]` | `TEXT[]` | PostgreSQL array type |
| Optional | Column without `NOT NULL` | Nullable by default |
| Non-optional | Column with `NOT NULL` | Explicitly mark required |

## CodingKeys → Column Names

Swift models use `CodingKeys` to map camelCase to snake_case. The SQL column names MUST match the snake_case values:

```swift
// Swift model
struct Post: Codable {
    let id: UUID
    let authorId: UUID      // → author_id
    let imageUrl: String    // → image_url
    let likeCount: Int      // → like_count
    let createdAt: Date     // → created_at

    enum CodingKeys: String, CodingKey {
        case id
        case authorId = "author_id"
        case imageUrl = "image_url"
        case likeCount = "like_count"
        case createdAt = "created_at"
    }
}
```

Corresponding SQL:
```sql
CREATE TABLE public.posts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  author_id UUID NOT NULL REFERENCES public.profiles(id) ON DELETE CASCADE,
  image_url TEXT NOT NULL,
  like_count INTEGER DEFAULT 0 NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now() NOT NULL
);
```

## Denormalized Counters

For count fields (follower_count, post_count), use database triggers to keep them accurate:

```sql
-- Auto-increment post count when a post is inserted
CREATE OR REPLACE FUNCTION increment_post_count()
RETURNS TRIGGER AS $$
BEGIN
  UPDATE public.profiles SET post_count = post_count + 1 WHERE id = NEW.author_id;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_post_insert
  AFTER INSERT ON public.posts
  FOR EACH ROW EXECUTE FUNCTION increment_post_count();

-- Auto-decrement on delete
CREATE OR REPLACE FUNCTION decrement_post_count()
RETURNS TRIGGER AS $$
BEGIN
  UPDATE public.profiles SET post_count = GREATEST(post_count - 1, 0) WHERE id = OLD.author_id;
  RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_post_delete
  AFTER DELETE ON public.posts
  FOR EACH ROW EXECUTE FUNCTION decrement_post_count();
```

Use `SECURITY DEFINER` so the trigger runs with the function owner's privileges, bypassing RLS.

## Verification

After creating all tables, verify with:
```sql
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';
```

Or use `mcp__supabase__list_tables` to confirm all tables exist before proceeding to Swift code.
