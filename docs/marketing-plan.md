# Nanowave Open-Source Marketing Plan

## Executive Summary

Nanowave is an AI-powered CLI that generates complete, compilable Apple apps from a single sentence — covering iOS, watchOS, tvOS, macOS, and visionOS. It runs on your existing Claude Pro/Max subscription, handles everything from architecture planning to App Store submission, and is MIT-licensed.

This plan covers launch strategy across Hacker News, Reddit, and other developer communities.

---

## 1. Hacker News — Show HN Post

### Why HN Works for Nanowave

Your previous Show HN (production-saas-starter) got 4 hours on the front page by following Tom's exact formula. Nanowave is an even stronger HN candidate because:
- It's a **deeply technical tool** (multi-phase AI pipeline, not a wrapper)
- It solves a **real developer pain point** (getting from idea to working app is slow)
- The **engineering story is compelling** (skill system, auto-fix loop, type-safe contracts)
- It's **MIT-licensed and genuinely open source**

### Draft Post

**Title:** `Show HN: Nanowave – Describe an app in one sentence, get a compiled Xcode project (MIT, Go + Claude)`

**Comment (post the link only, put this as the first comment — same strategy as last time):**

---

Hi HN, I'm Mohammed. I built Nanowave — an open-source CLI that generates complete Apple apps from a natural language description. You type one sentence, and it outputs a compiled Xcode project that builds and runs on the simulator.

**What it does:**

```
$ nanowave
> A workout tracker that logs exercises, tracks sets and reps, and shows weekly progress with charts

  ✓ Analyzed: FitTrack
  ✓ Plan ready (14 files, 5 models)
  ✓ Build complete — 14 files
  ✓ FitTrack is ready!
```

That's it. One sentence in, a full SwiftUI app out — with models, navigation, charts, persistence, the lot. It also handles editing existing projects, auto-fixing compilation errors, and submitting to the App Store entirely from the terminal.

**Why I built it:**

I kept having app ideas and hitting the same wall: even for a simple iOS app, you spend hours on boilerplate — setting up the Xcode project, writing navigation scaffolding, creating models, connecting persistence. The actual unique logic is 20% of the work; the other 80% is infrastructure every app needs.

I wanted something where I could describe the app I'm imagining and immediately have a working starting point — not a template with placeholder TODOs, but a real app with real data models and real UI that compiles and runs.

**The engineering challenge — why this is hard:**

Generating code that *looks right* is easy. Generating code that *compiles* is the hard part. My core insight was that this requires a structured pipeline, not a single prompt:

1. **Analyze** — Extracts app name, features, data models, and navigation from your description
2. **Plan** — Produces a file-level build plan (which files to create, their dependencies, what models they need)
3. **Build** — Generates Swift source files, asset catalogs, and the Xcode project config
4. **Fix** — Compiles with `xcodebuild` and iteratively repairs errors until the build is green
5. **Run** — Boots the iOS Simulator and launches

Each phase has typed contracts (`IntentDecision`, `AnalysisResult`, `PlannerResult`, `BuildResult`) — the AI output is parsed into Go structs, not string-matched. This is critical because string-matching AI output is fragile; typed contracts fail loudly at the boundary instead of silently producing garbage.

**The skill system (100+ embedded skills):**

The hardest part was getting consistently good SwiftUI output. Raw prompting produces code that *looks* like SwiftUI but uses deprecated APIs, wrong navigation patterns, or hardcoded values.

My solution was a skill system: 100+ embedded markdown files covering SwiftUI patterns, components, navigation, layout, accessibility, watchOS-specific patterns, tvOS focus management, and more. These are `//go:embed`-ded into the binary and composed into prompts based on what the app needs. If your app needs charts, it gets the Swift Charts skill. If it targets watchOS, it gets watchOS-specific component and navigation skills. This means the AI always has correct, up-to-date API references for exactly what it's generating.

**Platform support:**

Mention any Apple platform in your prompt — iOS, iPad, Apple Watch, Mac, Apple TV, Vision Pro — and it generates platform-appropriate code. The platform detection uses typed constants and a validation layer, not string matching on user input.

**App Store submission from the terminal:**

This is the part I'm most proud of. After your app builds, you can type `/connect publish to the App Store` and Nanowave handles:
- Code signing setup
- App Store Connect authentication
- Bundle ID registration
- App metadata and screenshots (auto-captured from simulator or uploaded via browser)
- Privacy declarations
- Submission for App Review

All without opening a browser or the App Store Connect web UI.

**Tech stack:**
- CLI: Go 1.26 + Cobra
- AI: Claude Code (runs on your existing Claude Pro/Max subscription — no separate API costs)
- Project generation: XcodeGen (via embedded MCP server)
- Build: xcodebuild with iterative error repair
- App Store: App Store Connect API with SRP authentication

**How to try it:**

```bash
brew install moasq/tap/nanowave
nanowave
```

On first launch it detects and installs any missing dependencies (Xcode CLI tools, XcodeGen, etc).

**Feedback I want:**

I'd love feedback from:
- iOS/macOS developers on the quality of generated SwiftUI code
- Go developers on the pipeline architecture and type contracts
- Anyone who's worked with AI code generation on what fails and what works

GitHub: https://github.com/moasq/nanowave

---

### HN Posting Checklist

- [ ] Post as a **link submission** (URL = GitHub repo)
- [ ] Title must start with **"Show HN:"** — this is critical (you forgot last time)
- [ ] Immediately add the technical comment above as the first comment
- [ ] Post on a **Tuesday or Wednesday, 8-10 AM EST** — peak HN traffic
- [ ] Email Tom (hn@ycombinator.com) after posting: "Hi Tom, I just submitted a new Show HN for my latest open-source project. Here's the link: [URL]. Thanks!"
- [ ] Stay online for the first 2-3 hours to respond to every comment
- [ ] Answer technical questions with depth — HN rewards specificity
- [ ] Don't be defensive about criticism — acknowledge limitations honestly
- [ ] If someone asks "why not just use Cursor/Copilot?" — explain the structured pipeline vs. autocomplete

### What Makes This Different From Your Previous HN Post

| production-saas-starter | Nanowave |
|---|---|
| A starter template (many exist) | Novel tool (few competitors in this exact space) |
| Backend-focused | Full pipeline from prompt → App Store |
| Static code you clone | Dynamic generation from natural language |
| "Here's code" | "Here's a working app from one sentence" |

Nanowave has a stronger "wow factor" — the demo is visceral. One sentence → compiled app is immediately impressive.

---

## 2. Reddit — Multi-Subreddit Strategy

### Target Subreddits (in priority order)

#### Tier 1 — High-impact, high-relevance

| Subreddit | Subscribers | Angle | Post Type |
|---|---|---|---|
| r/iOSProgramming | ~120K | "I built a CLI that generates SwiftUI apps from a description" | Show-off / project share |
| r/SwiftUI | ~60K | "Open-source tool that generates SwiftUI projects with correct patterns" | Tool announcement |
| r/opensource | ~50K | "MIT-licensed AI app generator — from prompt to App Store" | Project share |
| r/programming | ~6M | "Show: One-sentence app descriptions → compiled Xcode projects" | Project share |
| r/MachineLearning | ~3M | Pipeline architecture discussion — "How I structured an AI code gen pipeline that actually compiles" | Technical discussion |

#### Tier 2 — Good secondary reach

| Subreddit | Angle |
|---|---|
| r/SideProject | "I open-sourced my AI app generator" |
| r/golang | "Go CLI that orchestrates a multi-phase AI pipeline" — focus on the Go architecture |
| r/apple | "Generate apps for any Apple platform from your terminal" |
| r/CommandLine | "Terminal-first Apple app development" |
| r/selfhosted | "Self-hosted AI app generator — runs on your Claude subscription" |

#### Tier 3 — Niche but engaged

| Subreddit | Angle |
|---|---|
| r/visionosdev | "Generate Vision Pro apps from a description" |
| r/ClaudeAI | "Built an entire app generation pipeline on Claude Code" |
| r/IndieDev / r/IndieHackers | "Ship iOS apps in minutes instead of weeks" |
| r/WatchOS | "Generate watchOS apps from a single sentence" |

### Reddit Post Templates

#### r/iOSProgramming (Primary)

**Title:** I open-sourced a CLI that generates complete SwiftUI apps from a single sentence description

**Body:**

Hey everyone,

I've been working on Nanowave — an open-source CLI tool (MIT) that takes a natural language description and generates a full, compilable Xcode project with SwiftUI.

You describe what you want:

```
> A recipe manager with categories, ingredient lists, cooking timers, and a favorites system
```

And it outputs a complete Xcode project with:
- SwiftUI views with proper navigation
- SwiftData models
- Asset catalogs
- The right Apple frameworks (Charts, MapKit, HealthKit, etc. based on what your app needs)
- A project that actually compiles and runs on the simulator

It's not just code generation — there's a multi-phase pipeline (analyze → plan → build → fix → run) that iteratively compiles and repairs until the build is green. It also has 100+ embedded "skills" that teach it correct SwiftUI patterns, so it doesn't generate deprecated APIs or wrong navigation stacks.

Platforms: iPhone, iPad, Apple Watch, Mac, Apple TV, Vision Pro.

It can also handle App Store submission entirely from the terminal — code signing, metadata, screenshots, the works.

Built with Go, runs on Claude Code (uses your existing Claude Pro/Max subscription).

GitHub: https://github.com/moasq/nanowave

Install: `brew install moasq/tap/nanowave`

Would love feedback from iOS devs on the generated code quality. What patterns would you want it to get right?

---

#### r/golang

**Title:** I built a multi-phase AI orchestration pipeline in Go — here's the architecture

**Body:**

I've been building Nanowave, an open-source CLI that generates Apple apps from natural language descriptions. The interesting part (for this sub) is the Go architecture.

The core is a **multi-phase pipeline** where each phase has typed input/output contracts:

```
IntentDecision → AnalysisResult → PlannerResult → BuildResult
```

Each phase's AI output is parsed with a generic `parseClaudeJSON[T]()` function that:
1. Extracts JSON from markdown fences (AI sometimes wraps output in ```json blocks)
2. Deserializes into typed Go structs
3. Fails explicitly at the boundary — no silent degradation

The **string matching policy** was a key design decision: we use typed constants and map lookups for finite sets (platform names, operation types), but never string-match on unbounded input like user prompts or AI-generated descriptions. This sounds obvious but it's the #1 mistake I see in AI tool codebases.

Other Go patterns:
- 100+ skill files are `//go:embed`-ded and composed into prompts at runtime based on the app's needs
- Cobra for CLI commands
- An embedded MCP (Model Context Protocol) server for XcodeGen project generation
- Modular architecture: `internal/orchestration/`, `internal/claude/`, `internal/asc/` (App Store Connect)

MIT licensed, ~15K lines of Go: https://github.com/moasq/nanowave

Curious what Go devs think about the pipeline architecture and the generic JSON parsing approach.

---

### Reddit Posting Schedule

**Do NOT post to all subreddits at once.** Reddit's spam detection will flag you.

| Day | Subreddit | Time (EST) |
|---|---|---|
| Day 1 (Tue) | r/iOSProgramming | 9 AM |
| Day 1 (Tue) | r/SwiftUI | 12 PM |
| Day 2 (Wed) | r/opensource | 9 AM |
| Day 3 (Thu) | r/programming | 10 AM |
| Day 4 (Fri) | r/golang | 9 AM |
| Day 5+ | Tier 2 & 3 subreddits | Space 1-2 per day |

### Reddit Rules to Follow

1. **Check each subreddit's rules** before posting — some ban self-promotion, some require flair
2. **Don't use marketing language** — Reddit hates "revolutionary" and "game-changing"
3. **Be honest about limitations** — "it's not perfect, the generated code sometimes needs tweaking" earns respect
4. **Engage with every comment** in the first 2 hours
5. **Don't link-drop** — always include substantial text explaining what it is and why
6. **If people criticize it, agree where valid** and explain your reasoning where you disagree
7. **Have your account active in these communities** before posting — participate in discussions for a week or two first if you're not already active

---

## 3. Other Communities

### Dev.to

**Title:** "How I Built an AI Pipeline That Generates Compilable Apple Apps"

Focus on the engineering story — Dev.to rewards tutorial-style content. Structure:
1. The problem (boilerplate in iOS development)
2. Why a single prompt doesn't work (AI generates code that doesn't compile)
3. The pipeline architecture (analyze → plan → build → fix)
4. The skill system (teaching AI correct SwiftUI patterns)
5. Results and what's next
6. Call for contributors

### Lobste.rs

Similar to HN but smaller and more technical. Post the GitHub link with a concise technical description. Lobsters requires an invitation to join — if you don't have one, skip this for now.

### Product Hunt

Good for the "describe your app, get an app" angle. Focus on the user experience rather than the technical internals. Schedule a launch day and prepare:
- A 1-minute demo GIF/video showing: type description → app appears in simulator
- Gallery images of generated apps
- A concise tagline: "Describe your app. Nanowave writes the Swift."

### Twitter/X

Thread format works well:

```
Tweet 1: I open-sourced an AI tool that generates complete Apple apps from a single sentence.

One sentence → compiled Xcode project → running on the simulator.

It's called Nanowave, it's MIT-licensed, and it runs on your Claude subscription.

🧵 Here's how it works and what I learned building it:

Tweet 2: The core insight: generating code that LOOKS right is easy. Generating code that COMPILES is hard.

A single prompt produces SwiftUI that uses deprecated APIs, wrong navigation patterns, and hardcoded values.

My solution: a structured pipeline with typed contracts at every boundary.

Tweet 3: [Diagram/image of the pipeline: analyze → plan → build → fix → run]

Each phase has a Go struct defining its exact output shape. The AI output is parsed into these structs — not string-matched. This fails loudly instead of silently producing garbage.

Tweet 4: The secret sauce: 100+ embedded "skill" files.

Instead of hoping the AI knows current SwiftUI APIs, I embed markdown references covering components, navigation, layout, accessibility, and platform-specific patterns.

The right skills are composed into the prompt based on what the app needs.

Tweet 5: It supports every Apple platform: iPhone, iPad, Apple Watch, Mac, Apple TV, Vision Pro.

And after your app builds, you can submit to the App Store entirely from the terminal — code signing, metadata, screenshots, everything.

Tweet 6: Try it:

brew install moasq/tap/nanowave

MIT licensed. Built with Go.

GitHub: https://github.com/moasq/nanowave

I'd love feedback on the generated code quality and the pipeline architecture. PRs welcome.
```

Tag: @AnthropicAI @ClaudeAI — they may retweet open-source projects built on Claude.

### Bluesky / Mastodon

Cross-post the Twitter thread. Both have active developer communities.

### YouTube / Screen Recordings

Create a **2-minute demo video** showing:
1. `brew install moasq/tap/nanowave`
2. Type a description
3. Watch the pipeline run
4. See the app in the simulator
5. Edit the app with a follow-up prompt
6. Show the App Store submission flow

This video becomes the asset you link everywhere. Upload to YouTube and embed in the GitHub README.

---

## 4. Timing and Sequence

### Recommended Launch Order

```
Week 0 (Prep):
  - Record demo video
  - Create demo GIF for GitHub README
  - Ensure README is polished (it already is)
  - Prepare all post drafts
  - Be active in target subreddits (comment, help people)

Week 1:
  Mon — Final review of all drafts
  Tue — Hacker News Show HN (morning EST) + email Tom
  Tue — r/iOSProgramming + r/SwiftUI (afternoon, after HN traction)
  Wed — r/opensource
  Thu — r/programming + Twitter/X thread
  Fri — r/golang

Week 2:
  Mon — Dev.to article
  Tue — Product Hunt launch
  Wed-Fri — Tier 2 & 3 Reddit subreddits (one per day)

Week 3+:
  - Respond to GitHub issues from new users
  - Write follow-up posts based on community feedback
  - "What I learned launching an open-source AI tool on HN" (great Dev.to follow-up)
```

### Key Timing Notes

- **HN first** — it drives the most technical, engaged traffic and sets the narrative
- **Don't launch on Monday or Friday** on HN — lower traffic
- **Stay online for 3+ hours** after each major post — early engagement determines visibility
- **Cross-reference** — "This got great discussion on HN [link]" in Reddit posts adds social proof

---

## 5. What Makes Nanowave Compelling (Key Talking Points)

Use these angles depending on the audience:

| Audience | Lead with |
|---|---|
| iOS devs | "Generates correct SwiftUI with proper patterns — not deprecated boilerplate" |
| Go devs | "Multi-phase pipeline with typed contracts and embedded skill system" |
| AI/ML community | "Structured pipeline that compiles, not just generates" |
| Indie hackers | "Go from idea to App Store in minutes, not weeks" |
| Open-source community | "MIT-licensed, no API costs (runs on Claude subscription), 100+ embedded skills" |
| General devs | "One sentence → compiled Apple app that runs on the simulator" |

### Common Objections and How to Handle Them

| Objection | Response |
|---|---|
| "The generated code is probably bad" | "The skill system embeds 100+ reference files covering correct SwiftUI patterns. Plus the fix loop compiles and iteratively repairs until the build is green. Try it and see — happy to look at the output together." |
| "Why not just use Cursor/Copilot?" | "Those are autocomplete tools — great for writing code faster. Nanowave is an architecture tool — it plans the entire app structure, generates all files, and ensures they compile together. Different problem." |
| "This will make developers obsolete" | "It generates a starting point, not a finished product. You still need to customize, add business logic, and ship. It eliminates the 80% boilerplate so you can focus on the 20% that makes your app unique." |
| "It requires Claude Pro/Max?" | "Yes, it runs on Claude Code through your existing subscription. No separate API billing. If you already have Claude Pro ($20/mo) or Max, there's zero additional cost." |
| "Why Go and not Swift/Rust/Python?" | "Go gives me a single static binary, fast compilation, great concurrency for parallel AI calls, and type-safe JSON parsing with generics. Perfect for a CLI that orchestrates AI." |

---

## 6. Metrics to Track

- GitHub stars (before/after each post)
- GitHub traffic (referrer breakdown shows which platform drove visits)
- Homebrew install counts
- HN score and time on front page
- Reddit upvotes and comment quality
- Twitter impressions and retweets
- New GitHub issues and PRs (indicates real usage)

---

## 7. Post-Launch Community Building

- **Respond to every GitHub issue** within 24 hours during the first month
- **Label good-first-issues** to attract contributors
- **Create a CONTRIBUTING.md** if one doesn't exist
- **Share interesting generated apps** on Twitter — "Someone used Nanowave to generate [cool app] — here's what it looks like"
- **Write a "Lessons from launching on HN" follow-up** — these always do well on Dev.to and HN itself
- **Consider a Discord or GitHub Discussions** for community if traction warrants it
