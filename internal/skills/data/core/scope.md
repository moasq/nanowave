---
name: "scope"
description: "Scope rules: build the minimum functional app, limit screens, prioritize quality over quantity."
---
# Scope Rules — Build the Minimum Functional App

## Golden Rule: Small and Working > Big and Broken

A small app that works perfectly is ALWAYS better than a big app with broken features. Your goal is to impress the user with a polished, functional experience — not to overwhelm them with stub screens.

## Screen Limits

- Default to **1–2 screens maximum** unless the user explicitly asks for more.
- Pick only the **MOST CRITICAL features** that make the app functional.
- If the user mentions specific features, build those and ONLY those.

### Examples

| User says | Build | Do NOT build |
|---|---|---|
| "notes app" | list + editor (2 screens) | folders, tags, search, sharing, trash, settings |
| "music app" | library list + now playing (2 screens) | browse, radio, search, social, lyrics |
| "TikTok clone" | scrollable video feed (1 screen) | discover, create, inbox, profile, comments |
| "weather app" | current weather + forecast (1 screen) | hourly breakdown, maps, alerts, settings |
| "todo app" | task list with add/complete (1 screen) | categories, due dates, recurring, sharing |
| "chat app" | conversation list + chat (2 screens) | contacts, settings, profile, media gallery |

## Quality Over Quantity

- Every screen MUST be fully functional — real navigation, real data flow, real interactions.
- Do NOT create stub/placeholder views that show "Coming soon" or "TODO".
- Do NOT create empty tab views just to fill a TabBar.
- Prefer fewer polished screens over many incomplete screens.
- Build EXACTLY what the user asked for, nothing more. User intent is king.

## Feature Scoping

- When the user asks for an "app clone" (e.g., "TikTok clone", "Spotify clone"), build the ONE defining feature that makes that app recognizable, not every feature the real app has.
- The defining feature of TikTok is the vertical video feed. The defining feature of Spotify is the music library + player. The defining feature of Instagram is the photo feed.
- Additional features can be added later via edit requests — that's what the edit flow is for.
