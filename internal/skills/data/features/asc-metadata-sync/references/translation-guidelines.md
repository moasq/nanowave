# Translation Guidelines

## Supported Locales

App Store Connect locales:
```
ar-SA, ca, cs, da, de-DE, el, en-AU, en-CA, en-GB, en-US,
es-ES, es-MX, fi, fr-CA, fr-FR, he, hi, hr, hu, id, it,
ja, ko, ms, nl-NL, no, pl, pt-BR, pt-PT, ro, ru, sk,
sv, th, tr, uk, vi, zh-Hans, zh-Hant
```

## Translation Rules

- **Tone & Register**: Always use formal, polite language. Use formal "you" forms where the language distinguishes them (Russian: «вы», German: «Sie», French: «vous», Spanish: «usted», Dutch: «u», Italian: «Lei», Portuguese: «você» formal). App Store descriptions are professional marketing copy — never use casual or informal register.
- **description**: Translate naturally, adapt tone to local market. Keep formatting (line breaks, bullet points, emoji). Stay within 4000 chars.
- **keywords**: Do NOT literally translate. Research what users in that locale would search for. Comma-separated, max 100 chars total. No duplicates, no app name (Apple adds it automatically).
- **whatsNew**: Translate release notes. Keep it concise. Max 4000 chars.
- **promotionalText**: Translate marketing hook. Max 170 chars. Can be updated without a new version.
- **subtitle**: Translate or adapt tagline. Max 30 chars — very tight, may need creative adaptation.
- **name**: Usually keep the original app name. Only translate if the user explicitly asks. Max 30 chars.

## LLM Translation Prompt Template

For each target locale:

```
Translate the following App Store metadata from {source_locale} to {target_locale}.

Rules:
- description: Natural, fluent translation. Preserve formatting. Max 4000 chars.
- keywords: Do NOT literally translate. Choose keywords native speakers would search for. Comma-separated, max 100 chars total. Do not include the app name.
- whatsNew: Translate release notes naturally. Max 4000 chars.
- promotionalText: Translate marketing tagline. Max 170 chars.
- subtitle: Adapt tagline creatively to fit 30 chars max.
- name: Keep the original app name unless explicitly requested to translate it. Max 30 chars.
- Use formal, polite language and formal "you" forms. App Store copy is professional marketing — never use informal register.
- Respect cultural context.

Source ({source_locale}):
description: """
{description}
"""

keywords: {keywords}

whatsNew: """
{whatsNew}
"""

promotionalText: {promotionalText}

name: {name}

subtitle: {subtitle}
```

## Full Example: Add nl-NL and ru

```bash
# 1) Resolve IDs
asc apps list --output table
APP_ID="APP_ID_HERE"
asc versions list --app "$APP_ID" --state PREPARE_FOR_SUBMISSION --output table
VERSION_ID="VERSION_ID_HERE"
asc app-infos list --app "$APP_ID" --output table
APP_INFO_ID="APP_INFO_ID_HERE"

# 2) Download English source
asc localizations download --version "$VERSION_ID" --path "./localizations"
asc localizations download --app "$APP_ID" --type app-info --app-info "$APP_INFO_ID" --path "./app-info-localizations"

# 3) Read en-US.strings, translate to nl-NL and ru (LLM step)

# 4) Write nl-NL.strings and ru.strings to both directories

# 5) Upload all
asc localizations upload --version "$VERSION_ID" --path "./localizations"
asc localizations upload --app "$APP_ID" --type app-info --app-info "$APP_INFO_ID" --path "./app-info-localizations"

# 6) Verify
asc localizations list --version "$VERSION_ID" --output table
asc localizations list --app "$APP_ID" --type app-info --app-info "$APP_INFO_ID" --output table
```
