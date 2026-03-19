package validation

import (
	"fmt"
	"strings"
)

// Subscription represents an auto-renewable subscription for review-readiness validation.
type Subscription struct {
	ID                   string
	Name                 string
	ProductID            string
	State                string
	GroupID              string
	HasImage             bool
	ImageCheckSkipped    bool
	ImageCheckSkipReason string
}

// SubscriptionsInput collects subscription validation inputs.
type SubscriptionsInput struct {
	AppID         string
	Subscriptions []Subscription
}

// SubscriptionsReport is the top-level validate subscriptions output.
type SubscriptionsReport struct {
	AppID             string        `json:"appId"`
	SubscriptionCount int           `json:"subscriptionCount,omitempty"`
	Summary           Summary       `json:"summary"`
	Checks            []CheckResult `json:"checks"`
	Strict            bool          `json:"strict,omitempty"`
}

// ValidateSubscriptions validates subscription review readiness and returns a report.
func ValidateSubscriptions(input SubscriptionsInput, strict bool) SubscriptionsReport {
	checks := make([]CheckResult, 0)
	checks = append(checks, subscriptionImageChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionReviewReadinessChecks(input.Subscriptions)...)
	summary := summarize(checks, strict)

	return SubscriptionsReport{
		AppID:             strings.TrimSpace(input.AppID),
		SubscriptionCount: len(input.Subscriptions),
		Summary:           summary,
		Checks:            checks,
		Strict:            strict,
	}
}

func subscriptionFetchChecks(reason string) []CheckResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}

	return []CheckResult{{
		ID:          "subscriptions.readiness.unverified",
		Severity:    SeverityInfo,
		Field:       "subscriptions",
		Message:     "Could not verify subscription readiness for this app",
		Remediation: reason,
	}}
}

func subscriptionImageChecks(subs []Subscription) []CheckResult {
	var checks []CheckResult
	for _, sub := range subs {
		state := strings.ToUpper(strings.TrimSpace(sub.State))
		if state == "REMOVED_FROM_SALE" || state == "DEVELOPER_REMOVED_FROM_SALE" {
			continue
		}
		label := formatSubscriptionLabel(sub)
		if sub.ImageCheckSkipped {
			remediation := strings.TrimSpace(sub.ImageCheckSkipReason)
			if remediation == "" {
				remediation = "Review this subscription's promotional image in App Store Connect; validation could not verify image presence automatically"
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.images.unverified",
				Severity:     SeverityInfo,
				Field:        "images",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("Could not verify whether %s has a subscription promotional image", label),
				Remediation:  remediation,
			})
			continue
		}
		if sub.HasImage {
			continue
		}

		checks = append(checks, CheckResult{
			ID:           "subscriptions.images.recommended",
			Severity:     SeverityWarning,
			Field:        "images",
			ResourceType: "subscription",
			ResourceID:   strings.TrimSpace(sub.ID),
			Message:      fmt.Sprintf("%s has no subscription promotional image", label),
			Remediation:  "Upload a unique promotional image if you plan to promote this subscription on the App Store, support offer-code redemption pages, or run win-back offers; the App Review screenshot is separate and review-only",
		})
	}

	return checks
}

func subscriptionReviewReadinessChecks(subs []Subscription) []CheckResult {
	// These checks are warnings by default. Many apps have subscriptions that
	// aren't relevant to a given release. Use --strict to gate in CI.
	okStates := map[string]struct{}{
		"APPROVED":                {},
		"WAITING_FOR_REVIEW":      {},
		"IN_REVIEW":               {},
		"PENDING_BINARY_APPROVAL": {},
	}
	ignoreStates := map[string]struct{}{
		"DEVELOPER_REMOVED_FROM_SALE": {},
		"REMOVED_FROM_SALE":           {},
	}

	var checks []CheckResult
	for _, sub := range subs {
		state := strings.ToUpper(strings.TrimSpace(sub.State))
		if state == "" {
			continue
		}
		if _, ok := okStates[state]; ok {
			continue
		}
		if _, ok := ignoreStates[state]; ok {
			continue
		}

		label := formatSubscriptionLabel(sub)
		message := fmt.Sprintf("%s is %s", label, state)
		remediation := remediationForSubscriptionState(state)

		checks = append(checks, CheckResult{
			ID:           "subscriptions.review_readiness.needs_attention",
			Severity:     SeverityWarning,
			Field:        "state",
			ResourceType: "subscription",
			ResourceID:   strings.TrimSpace(sub.ID),
			Message:      message,
			Remediation:  remediation,
		})
	}

	return checks
}

func formatSubscriptionLabel(sub Subscription) string {
	name := strings.TrimSpace(sub.Name)
	productID := strings.TrimSpace(sub.ProductID)

	switch {
	case name != "" && productID != "":
		return fmt.Sprintf("Subscription %q (%s)", name, productID)
	case name != "":
		return fmt.Sprintf("Subscription %q", name)
	case productID != "":
		return fmt.Sprintf("Subscription %s", productID)
	default:
		return "Subscription"
	}
}

func remediationForSubscriptionState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "MISSING_METADATA":
		return "Complete required metadata for this subscription, including its image, in App Store Connect"
	case "READY_TO_SUBMIT":
		return "Submit this subscription for review in App Store Connect so it is attached to the next app review submission"
	case "DEVELOPER_ACTION_NEEDED":
		return "Resolve developer action required issues for this subscription in App Store Connect"
	case "REJECTED":
		return "Address the rejection feedback for this subscription and resubmit in App Store Connect"
	default:
		return "Review this subscription in App Store Connect and submit it for review if needed"
	}
}
