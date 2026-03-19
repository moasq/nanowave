---
name: "biometrics"
description: "Biometric authentication: Face ID, Touch ID, LAContext, fallback to passcode. Use when implementing app features related to biometrics."
---
# Biometrics

BIOMETRIC AUTHENTICATION (Face ID / Touch ID):
- import LocalAuthentication; LAContext().evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics)
- Requires NSFaceIDUsageDescription permission (add CONFIG_CHANGES)
- Check canEvaluatePolicy first; fall back to passcode if biometrics unavailable
- LAContext().biometryType to detect .faceID vs .touchID vs .none
- Always provide manual unlock alternative (PIN/password)
