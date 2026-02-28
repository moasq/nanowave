# Apple Compliance Checklist

## Contents
- Mandatory Requirements
- Price Display Rules
- UI Element Rules
- Legal Requirements
- Common Rejection Reasons

## Mandatory Requirements

Every paywall must include ALL of the following:

- [ ] Close button visible immediately (top-leading or top-trailing)
- [ ] Full billed amount displayed prominently (minimum 16pt)
- [ ] No price toggles — tappable cards only
- [ ] No countdown timers or "limited time" urgency
- [ ] Schedule 2, Section 3.8(b) disclosure text
- [ ] Terms of Service link (tappable, opens in-app)
- [ ] Privacy Policy link (tappable, opens in-app)
- [ ] Restore Purchases button (visible without scrolling)
- [ ] All prices from StoreKit (never hardcoded)
- [ ] Subscription duration clearly stated next to price

## Price Display Rules

- The full recurring price must be the most prominent price element
- If showing a per-day/per-week breakdown, the full price must be larger
- Free trial terms must show when billing starts
- Price must come from `product.localizedPriceString` (handles currency/locale)

## UI Element Rules

- Plan selection: use tappable cards with clear selected state
- No toggles, switches, or radio buttons for plan selection
- CTA button must show what will happen ("Subscribe" not "Continue")
- Loading state on CTA during purchase (disabled + spinner)

## Legal Requirements

Footer must contain:
1. Schedule 2 disclosure text (see disclosure-text.md)
2. "Terms of Service" as a tappable link
3. "Privacy Policy" as a tappable link
4. "Restore Purchases" as a tappable button

## Common Rejection Reasons

- Close button hidden or delayed
- Price not prominent enough (too small, wrong color)
- Missing restore purchases
- Hardcoded prices that don't match store
- Missing legal disclosure text
- Toggle-based plan selection
