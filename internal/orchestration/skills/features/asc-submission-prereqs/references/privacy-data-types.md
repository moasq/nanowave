---
name: asc-privacy-data-types
description: "Complete list of App Store privacy nutrition label data types and usage purposes. Use when helping users declare app privacy data collection."
---

# Contents

1. Data Type Categories
2. Usage Purposes
3. Optional Disclosure Criteria
4. Common App Patterns

## 1. Data Type Categories

### Contact Info
- Name (first or last name)
- Email Address (including hashed)
- Phone Number (including hashed)
- Physical Address
- Other User Contact Info

### Health & Fitness
- Health (HealthKit, Clinical Health Records, user-provided medical data)
- Fitness (Motion and Fitness API, exercise data)

### Financial Info
- Payment Info (card number, bank account — not needed if payment handled by Apple)
- Credit Info (credit score)
- Other Financial Info (salary, income, assets, debts)

### Location
- Precise Location (3+ decimal places latitude/longitude)
- Coarse Location (approximate, lower resolution)

### Sensitive Info
- Racial/ethnic data, sexual orientation, pregnancy, disability, religious beliefs, political opinion, genetic/biometric data

### Contacts
- Phone contacts, address book, social graph

### User Content
- Emails or Text Messages
- Photos or Videos
- Audio Data (voice/sound recordings)
- Gameplay Content
- Customer Support data
- Other User Content

### Browsing History
- Browsing History (websites viewed outside app)
- Search History (searches within app)

### Identifiers
- User ID (screen name, account ID, customer number)
- Device ID (advertising identifier, device-level ID)

### Purchases
- Purchase History (individual purchases, purchase tendencies)

### Usage Data
- Product Interaction (app launches, taps, clicks, scrolling, views)
- Advertising Data (ads user has seen)
- Other Usage Data

### Diagnostics
- Crash Data (crash logs)
- Performance Data (launch time, hang rate, energy use)
- Other Diagnostic Data

### Surroundings
- Environment Scanning (mesh, planes, scene classification)

### Body
- Hands (hand structure and movements)
- Head (head movement)

### Other Data
- Any data not covered above

## 2. Usage Purposes

| Purpose | Definition |
|---|---|
| Third-Party Advertising | Displaying third-party ads; sharing data for ad display |
| Developer's Advertising | First-party ads, marketing communications |
| Analytics | Evaluating behavior, measuring audience, understanding features |
| Product Personalization | Customizing experience (recommendations, suggestions) |
| App Functionality | Auth, enabling features, fraud prevention, security, support |
| Other Purposes | Any purpose not listed above |

## 3. Optional Disclosure Criteria

Data is optional to disclose ONLY if ALL criteria are met:
1. Not used for tracking (not linked with third-party data for ads)
2. Not used for advertising/marketing
3. Collection is infrequent and not part of primary functionality
4. User affirmatively chooses to provide data each time

Example: optional feedback form, one-time customer support request.

If ANY criterion is not met, disclosure is REQUIRED.

## 4. Common App Patterns

| If your app... | Declare these data types |
|---|---|
| Has user accounts | Contact Info (Email/Name), Identifiers (User ID) |
| Uses Firebase Analytics | Usage Data, Diagnostics, Identifiers (Device ID) |
| Uses Sentry/Crashlytics | Diagnostics (Crash Data, Performance) |
| Sends text to OpenAI/Claude | User Content (Other), Usage Data |
| Sends images to AI vision | Photos or Videos, User Content |
| Uses push notifications | Identifiers (Device ID) |
| Has location features | Location (Precise or Coarse) |
| Has in-app purchases | Purchases, Financial Info (if payment outside Apple) |
| Uses AdMob/ads SDK | Identifiers, Usage Data, Advertising Data |
| Collects health data | Health & Fitness |
| Has social/sharing features | Contacts, User Content |
| Uses RevenueCat | Purchases, Identifiers |
| Uses Supabase with auth | Contact Info, Identifiers, User Content |
