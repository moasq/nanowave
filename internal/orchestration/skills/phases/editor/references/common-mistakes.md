# Common Mistakes

- Breaking unrelated screens while making local edits.
- Creating unreferenced new views.
- Adding settings toggles that do not change behavior.
- Skipping rebuild validation.
- Introducing hardcoded colors (.white, Color.red) instead of using AppTheme.Colors.* tokens.
- Introducing hardcoded fonts (.font(.title2), .font(.system(size:))) instead of using AppTheme.Fonts.* tokens.
- Introducing hardcoded spacing (.padding(20)) instead of using AppTheme.Spacing.* tokens.
