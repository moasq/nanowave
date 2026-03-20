# Common Mistakes

## Contents
- Scope mistakes
- Feature quality mistakes
- Output mistakes

## Scope Mistakes

- Adding extra features the user did not ask for (search, categories, settings, dark mode when not requested).
- Over-engineering — building 10 features when 3 do the job.
- Including features that require external services (REST APIs, cloud backends, third-party packages).
- Including API keys, secrets, or tokens.

## Feature Quality Mistakes

- Deferring cheap explicit requests like dark mode or localization (these are 1-2 files).
- Returning vague feature names with no user action (e.g. "Data Management" instead of "Save Recipe").
- Features that don't map to a real user interaction.
- Missing the critical piece the user implied (e.g. a list app without a way to add items).

## Output Mistakes

- Forgetting core_flow.
- Populating deferred with features the user never mentioned.
- Returning text instead of valid JSON.
- Using app_name that's too generic or too long.
