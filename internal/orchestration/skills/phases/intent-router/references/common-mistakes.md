# Common Mistakes

- Turning hints into hard requirements.
- Inferring watchOS, tvOS, or iPad when the user did not say it.
- Returning prose instead of JSON.
- Using advanced terms in reason; keep it plain.
- Returning "multiplatform" as platform_hint — always pick ONE platform (ios, watchos, or tvos).
- Missing `has_asc_intent` when the user mentions publishing, TestFlight, App Store submission, or metadata. Any distribution/publishing mention = `true`.
- Setting `has_asc_intent: true` for pure build/edit/fix requests that don't mention publishing or distribution.
