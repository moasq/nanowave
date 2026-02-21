# iOS Provisioning & Signing Automation

## What Is Provisioning?

Apple requires every iOS app to be cryptographically signed before it can run on a physical device. This involves:

1. **Certificates** — Prove your identity as a developer
2. **Bundle IDs** — Uniquely identify your app in Apple's ecosystem
3. **Devices** — Register which physical devices can run development builds
4. **Provisioning Profiles** — Bind certificates + bundle IDs + devices together

This process is notoriously painful when done manually through the Apple Developer Portal. Xcode's "Automatic Signing" handles it for individual developers, but breaks down for CI/CD, team workflows, and automation tools like Nanowave.

## The Automation Approach

We automate the entire flow using the **App Store Connect API v1**. This REST API lets us programmatically:

- Generate signing certificates (development + distribution)
- Register bundle IDs and enable capabilities
- Register test devices
- Create and download provisioning profiles

All API calls are authenticated via **JWT tokens** signed with an App Store Connect API key.

## Prerequisites

1. **Apple Developer Program membership** ($99/year)
2. **App Store Connect API key** with Admin role — see [api-setup.md](api-setup.md)
3. **macOS** with Xcode command line tools installed
4. **Go 1.21+** for running the automation code

## Quick Reference

| Document | What It Covers |
|---|---|
| [api-setup.md](api-setup.md) | API key creation, JWT token generation |
| [certificates.md](certificates.md) | CSR generation, certificate creation & installation |
| [bundle-ids.md](bundle-ids.md) | Bundle ID registration, capability management |
| [devices.md](devices.md) | Device registration, UDID discovery |
| [profiles.md](profiles.md) | Provisioning profile creation & installation |
| [keychain.md](keychain.md) | macOS Keychain operations for CI/CD |
| [implementation-plan.md](implementation-plan.md) | CLI integration spec, proposed commands |

## End-to-End Flow

```
1. Create API Key (one-time, manual in App Store Connect)
         │
         ▼
2. Generate JWT Token (api-setup.md)
         │
         ▼
3. Generate CSR + Create Certificate (certificates.md)
         │
         ▼
4. Register Bundle ID + Capabilities (bundle-ids.md)
         │
         ▼
5. Register Test Devices (devices.md)
         │
         ▼
6. Create Provisioning Profile (profiles.md)
         │
         ▼
7. Install Certificate + Profile Locally (keychain.md)
         │
         ▼
8. Build with Manual Signing (implementation-plan.md)
```

## Current State

The Nanowave CLI currently hardcodes `CODE_SIGN_STYLE: Automatic` with no `DEVELOPMENT_TEAM` set. This works for simulator builds but requires manual Xcode configuration for device builds, TestFlight, or App Store submission.

### Where signing is configured today

| File | What It Does |
|---|---|
| `cli/internal/orchestration/xcodegen.go:83,167` | Hardcodes `CODE_SIGN_STYLE: Automatic` for main + extension targets |
| `cli/internal/xcodegenserver/config.go:137,228` | Same hardcoding in the MCP server YAML generator |
| `cli/internal/service/service.go:372` | xcodebuild invocation — no signing flags |
| `cli/internal/storage/project.go` | Project store — no signing fields |
| `cli/internal/orchestration/pipeline.go` | Build prompts — no signing flags |
