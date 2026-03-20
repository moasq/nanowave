# Storage Setup

## Contents
- Creating storage buckets via MCP
- Public vs private buckets
- Storage policies (RLS for buckets)
- MIME type restrictions
- Path structure conventions

## Creating Storage Buckets

Use `mcp__supabase__execute_sql` to create buckets. Buckets must be created BEFORE any Swift code references them.

### Public Bucket (Images, Avatars)

For content that should be publicly accessible (post images, avatars):

```sql
INSERT INTO storage.buckets (id, name, public)
VALUES ('post-images', 'post-images', true)
ON CONFLICT (id) DO NOTHING;
```

### Private Bucket (Documents, User Files)

For content that requires authentication to access:

```sql
INSERT INTO storage.buckets (id, name, public)
VALUES ('user-documents', 'user-documents', false)
ON CONFLICT (id) DO NOTHING;
```

### Bucket with File Restrictions

Restrict uploads by size and MIME type at the bucket level:

```sql
INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES ('profile-images', 'profile-images', true, 5242880, ARRAY['image/jpeg', 'image/png', 'image/webp'])
ON CONFLICT (id) DO NOTHING;
```

### Bucket Column Reference

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `id` | text | required | Primary key |
| `name` | text | required | Unique bucket name |
| `public` | boolean | `false` | Public download access (uploads still need RLS) |
| `file_size_limit` | bigint | NULL | Max file size in **bytes** |
| `allowed_mime_types` | text[] | NULL | Array of permitted MIME types |

**Important**: Direct `DELETE` from `storage.buckets` is blocked by a trigger. Bucket deletion must go through the Storage API.

## Storage Policies

Storage buckets use the same RLS system as tables. Policies go on `storage.objects`.

### Public Read, Authenticated Upload

Standard pattern for public media (post images, avatars):

```sql
-- Anyone can view files in public bucket
CREATE POLICY "public read for post-images"
  ON storage.objects FOR SELECT
  USING (bucket_id = 'post-images');

-- Authenticated users can upload to their own folder
CREATE POLICY "authenticated upload to post-images"
  ON storage.objects FOR INSERT
  WITH CHECK (
    bucket_id = 'post-images'
    AND auth.role() = 'authenticated'
    AND (storage.foldername(name))[1] = auth.uid()::text
  );

-- Users can update their own files
CREATE POLICY "users can update own files in post-images"
  ON storage.objects FOR UPDATE
  USING (
    bucket_id = 'post-images'
    AND auth.uid()::text = (storage.foldername(name))[1]
  );

-- Users can delete their own files
CREATE POLICY "users can delete own files in post-images"
  ON storage.objects FOR DELETE
  USING (
    bucket_id = 'post-images'
    AND auth.uid()::text = (storage.foldername(name))[1]
  );
```

### Avatar Bucket Pattern

Avatars with upsert support (user replaces their avatar):

```sql
INSERT INTO storage.buckets (id, name, public)
VALUES ('avatars', 'avatars', true)
ON CONFLICT (id) DO NOTHING;

CREATE POLICY "public read for avatars"
  ON storage.objects FOR SELECT
  USING (bucket_id = 'avatars');

CREATE POLICY "users can upload own avatar"
  ON storage.objects FOR INSERT
  WITH CHECK (
    bucket_id = 'avatars'
    AND auth.role() = 'authenticated'
    AND (storage.foldername(name))[1] = auth.uid()::text
  );

CREATE POLICY "users can update own avatar"
  ON storage.objects FOR UPDATE
  USING (
    bucket_id = 'avatars'
    AND auth.uid()::text = (storage.foldername(name))[1]
  );
```

### Private Bucket Pattern

For user-private files (documents, exports):

```sql
-- Only owner can read their files
CREATE POLICY "users can read own files"
  ON storage.objects FOR SELECT
  USING (
    bucket_id = 'user-documents'
    AND auth.uid()::text = (storage.foldername(name))[1]
  );

-- Only owner can upload to their folder
CREATE POLICY "users can upload own files"
  ON storage.objects FOR INSERT
  WITH CHECK (
    bucket_id = 'user-documents'
    AND auth.role() = 'authenticated'
    AND (storage.foldername(name))[1] = auth.uid()::text
  );
```

## Path Structure Convention

Always organize storage by user ID as the top-level folder:

```
{bucket}/
  {user-id}/
    {unique-filename}.{ext}
```

In Swift:
```swift
let userId = try await client.auth.session.user.id
let path = "\(userId)/\(UUID().uuidString).jpg"
try await client.storage.from("post-images")
    .upload(path, data: imageData, options: FileOptions(contentType: "image/jpeg"))
```

This structure ensures storage policies can enforce per-user access using `storage.foldername(name)[1]`.

## MIME Type Safety

Restrict uploads by content type where appropriate:

```sql
-- Only allow image uploads (optional — enforced at bucket level or policy level)
CREATE POLICY "only images in post-images"
  ON storage.objects FOR INSERT
  WITH CHECK (
    bucket_id = 'post-images'
    AND (storage.extension(name) IN ('jpg', 'jpeg', 'png', 'gif', 'webp'))
  );
```

In Swift, always set the correct content type:
```swift
FileOptions(contentType: "image/jpeg", upsert: false)
FileOptions(contentType: "image/png", upsert: true)  // upsert for avatar replacement
```

## Verification

After creating buckets and policies, verify:

```sql
-- List all buckets
SELECT id, name, public FROM storage.buckets;

-- List policies on storage.objects
SELECT policyname, cmd FROM pg_policies WHERE tablename = 'objects' AND schemaname = 'storage';
```

Or use `mcp__supabase__list_storage_buckets` to confirm bucket creation.
