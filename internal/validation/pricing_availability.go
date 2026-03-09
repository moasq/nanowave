package validation

import (
	"fmt"
	"strings"
)

func pricingChecks(appID string, priceScheduleID string) []CheckResult {
	if strings.TrimSpace(priceScheduleID) != "" {
		return nil
	}
	return []CheckResult{
		{
			ID:           "pricing.schedule.missing",
			Severity:     SeverityError,
			Field:        "appPriceSchedule",
			ResourceType: "app",
			ResourceID:   strings.TrimSpace(appID),
			Message:      "app price schedule is missing",
			Remediation:  "Set pricing for the app in App Store Connect (Pricing and Availability)",
		},
	}
}

func availabilityChecks(appID string, availabilityID string, availableTerritories int) []CheckResult {
	if strings.TrimSpace(availabilityID) == "" {
		return []CheckResult{
			{
				ID:           "availability.missing",
				Severity:     SeverityError,
				Field:        "appAvailabilityV2",
				ResourceType: "app",
				ResourceID:   strings.TrimSpace(appID),
				Message:      "app availability is missing",
				Remediation:  "Configure availability for the app in App Store Connect (Pricing and Availability)",
			},
		}
	}

	if availableTerritories > 0 {
		return nil
	}

	return []CheckResult{
		{
			ID:           "availability.territories.none",
			Severity:     SeverityError,
			Field:        "territoryAvailabilities",
			ResourceType: "appAvailabilityV2",
			ResourceID:   strings.TrimSpace(availabilityID),
			Message:      fmt.Sprintf("no available territories configured (available=%d)", availableTerritories),
			Remediation:  "Enable at least one territory in App Store Connect (Pricing and Availability)",
		},
	}
}
