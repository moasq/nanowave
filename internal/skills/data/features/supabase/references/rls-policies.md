# Row Level Security (RLS) Policies

## Contents
- RLS fundamentals
- Common policy patterns
- Auth-linked table policies
- Content table policies
- Join table policies
- Service role considerations

## CRITICAL: Always Enable RLS

Every public table MUST have RLS enabled. Without RLS, any user with the anon key can read/write all data.

```sql
ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.posts ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.follows ENABLE ROW LEVEL SECURITY;
```

Enable RLS immediately after creating each table — never leave a table unprotected.

## Core Concept: auth.uid()

`auth.uid()` returns the authenticated user's UUID from the JWT. Use it to restrict access:

```sql
-- Only the owner can update their row
CREATE POLICY "users can update own profile"
  ON public.profiles FOR UPDATE
  USING (auth.uid() = id);
```

## Profile Table Policies

Profiles are public-read, owner-write:

```sql
-- Anyone can read profiles
CREATE POLICY "profiles are viewable by everyone"
  ON public.profiles FOR SELECT
  USING (true);

-- Users can insert their own profile (during signup)
CREATE POLICY "users can insert own profile"
  ON public.profiles FOR INSERT
  WITH CHECK (auth.uid() = id);

-- Users can update only their own profile
CREATE POLICY "users can update own profile"
  ON public.profiles FOR UPDATE
  USING (auth.uid() = id)
  WITH CHECK (auth.uid() = id);

-- Users can delete only their own profile
CREATE POLICY "users can delete own profile"
  ON public.profiles FOR DELETE
  USING (auth.uid() = id);
```

## Content Table Policies (Posts, Notes, Items)

Content is typically public-read, owner-write:

```sql
-- Anyone can read posts
CREATE POLICY "posts are viewable by everyone"
  ON public.posts FOR SELECT
  USING (true);

-- Authenticated users can create posts (own author_id only)
CREATE POLICY "authenticated users can create posts"
  ON public.posts FOR INSERT
  WITH CHECK (auth.uid() = author_id);

-- Users can update only their own posts
CREATE POLICY "users can update own posts"
  ON public.posts FOR UPDATE
  USING (auth.uid() = author_id)
  WITH CHECK (auth.uid() = author_id);

-- Users can delete only their own posts
CREATE POLICY "users can delete own posts"
  ON public.posts FOR DELETE
  USING (auth.uid() = author_id);
```

## Join Table Policies (Follows, Likes)

Join tables are public-read, actor-write:

```sql
-- Anyone can see follows
CREATE POLICY "follows are viewable by everyone"
  ON public.follows FOR SELECT
  USING (true);

-- Users can follow others (insert where they are the follower)
CREATE POLICY "users can follow others"
  ON public.follows FOR INSERT
  WITH CHECK (auth.uid() = follower_id);

-- Users can unfollow (delete where they are the follower)
CREATE POLICY "users can unfollow"
  ON public.follows FOR DELETE
  USING (auth.uid() = follower_id);
```

Note: No UPDATE policy — follows are insert/delete only.

## Private Content Policies

For content only visible to the owner (e.g., drafts, private notes):

```sql
-- Only owner can see their private notes
CREATE POLICY "users can view own notes"
  ON public.notes FOR SELECT
  USING (auth.uid() = user_id);

-- Only owner can CRUD their notes
CREATE POLICY "users can manage own notes"
  ON public.notes FOR ALL
  USING (auth.uid() = user_id)
  WITH CHECK (auth.uid() = user_id);
```

## Shared Content Policies

For content shared between specific users (e.g., group members, collaborators):

```sql
-- Members can view shared content
CREATE POLICY "members can view shared items"
  ON public.shared_items FOR SELECT
  USING (
    auth.uid() IN (
      SELECT user_id FROM public.memberships
      WHERE group_id = shared_items.group_id
    )
  );
```

## Authenticated-Only Access

Restrict to logged-in users only (no anonymous access):

```sql
-- Only authenticated users can read
CREATE POLICY "authenticated access only"
  ON public.messages FOR SELECT
  USING (auth.role() = 'authenticated');
```

## Policy Naming Convention

Use descriptive names that explain the rule:
- `"profiles are viewable by everyone"` — SELECT on profiles
- `"users can update own profile"` — UPDATE on profiles
- `"authenticated users can create posts"` — INSERT on posts
- `"users can follow others"` — INSERT on follows

## Security Anti-Patterns

**NEVER do:**
- Leave RLS disabled on any public table
- Use `USING (true)` for INSERT/UPDATE/DELETE (allows anyone to modify data)
- Skip `WITH CHECK` on INSERT/UPDATE (allows inserting data for other users)
- Use `auth.uid()` without `= column_name` comparison (meaningless check)

**ALWAYS do:**
- Enable RLS on every table immediately after creation
- Use `WITH CHECK` on INSERT to validate the user is setting their own ID
- Use both `USING` and `WITH CHECK` on UPDATE
- Test policies by attempting operations as different users
