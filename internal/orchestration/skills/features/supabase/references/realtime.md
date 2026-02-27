# Realtime

## Contents
- Enabling Realtime on tables via SQL
- Publication management
- Replica identity configuration
- Swift client subscription patterns

## Enabling Realtime on a Table

Supabase Realtime uses a PostgreSQL publication named `supabase_realtime`. Add tables to this publication to enable real-time subscriptions.

### Add a Table to Realtime

```sql
ALTER PUBLICATION supabase_realtime ADD TABLE public.messages;
```

### Add Multiple Tables

```sql
ALTER PUBLICATION supabase_realtime ADD TABLE public.messages, public.posts, public.comments;
```

### Remove a Table from Realtime

```sql
ALTER PUBLICATION supabase_realtime DROP TABLE public.messages;
```

### Check Which Tables Have Realtime Enabled

```sql
SELECT schemaname, tablename
FROM pg_publication_tables
WHERE pubname = 'supabase_realtime';
```

## Replica Identity

By default, UPDATE and DELETE events only include the primary key of changed rows. To receive the full old row data, set `REPLICA IDENTITY FULL`:

```sql
ALTER TABLE public.messages REPLICA IDENTITY FULL;
```

This is recommended for tables where you need to detect what changed in the row.

## Swift Client Patterns

### Subscribe to INSERT Events

```swift
let channel = client.channel("messages")

let changes = channel.postgresChange(
    InsertAction.self,
    schema: "public",
    table: "messages"
)

await channel.subscribe()

for await insert in changes {
    let message = try insert.decodeRecord(as: Message.self)
    // Handle new message
}
```

### Subscribe to All Changes

```swift
let changes = channel.postgresChange(
    AnyAction.self,
    schema: "public",
    table: "messages"
)

for await change in changes {
    switch change {
    case let .insert(action):
        let record = try action.decodeRecord(as: Message.self)
    case let .update(action):
        let record = try action.decodeRecord(as: Message.self)
    case let .delete(action):
        let old = try action.decodeOldRecord(as: Message.self)
    }
}
```

### Filter by Column Value

```swift
let changes = channel.postgresChange(
    InsertAction.self,
    schema: "public",
    table: "messages",
    filter: .eq("room_id", value: roomId)
)
```

## Pipeline Integration

When the analysis detects real-time features (chat, live updates, collaborative editing), the pipeline enables Realtime on relevant tables after table creation:

```sql
-- Enable realtime on tables that need live updates
ALTER PUBLICATION supabase_realtime ADD TABLE public.messages;
ALTER TABLE public.messages REPLICA IDENTITY FULL;
```

This is executed via `POST /v1/projects/{ref}/database/query`.
