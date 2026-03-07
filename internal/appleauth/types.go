package appleauth

// TrustedPhone represents a phone number for SMS 2FA.
type TrustedPhone struct {
	ID                 int    `json:"id"`
	NumberWithDialCode string `json:"numberWithDialCode"`
}

// AuthState holds the 2FA state after SRP authentication.
type AuthState struct {
	TrustedPhones     []TrustedPhone
	CodeLength        int
	HasTrustedDevices bool
}

// OnboardingResult holds the outcome of the full onboarding flow.
type OnboardingResult struct {
	KeyID    string
	IssuerID string
	TeamName string
}
