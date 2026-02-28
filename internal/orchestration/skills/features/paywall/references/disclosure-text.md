# Disclosure Text

## Schedule 2, Section 3.8(b) Boilerplate

This text is MANDATORY in every subscription paywall footer:

```
Payment will be charged to your Apple ID account at confirmation of purchase.
Subscription automatically renews unless canceled at least 24 hours before the
end of the current period. Your account will be charged for renewal within 24
hours prior to the end of the current period. You can manage and cancel your
subscriptions by going to Settings > Apple ID > Subscriptions.
```

## Usage

Place in a `Text` view with caption styling:

```swift
Text("Payment will be charged to your Apple ID account at confirmation of purchase. Subscription automatically renews unless canceled at least 24 hours before the end of the current period. Your account will be charged for renewal within 24 hours prior to the end of the current period. You can manage and cancel your subscriptions by going to Settings > Apple ID > Subscriptions.")
    .font(AppTheme.Fonts.caption)
    .foregroundStyle(AppTheme.Colors.textTertiary)
    .multilineTextAlignment(.center)
```

## Additional Required Links

Below the disclosure text, include:

```swift
HStack(spacing: AppTheme.Spacing.md) {
    Link("Terms of Service", destination: URL(string: "https://example.com/terms")!)
    Link("Privacy Policy", destination: URL(string: "https://example.com/privacy")!)
}
.font(AppTheme.Fonts.caption)
```

Replace URLs with the app's actual Terms and Privacy Policy URLs.
