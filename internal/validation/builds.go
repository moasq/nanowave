package validation

import (
	"fmt"
	"strings"
)

func buildChecks(build *Build) []CheckResult {
	if build == nil {
		return []CheckResult{
			{
				ID:           "build.required.missing",
				Severity:     SeverityError,
				ResourceType: "build",
				Message:      "no build attached to app store version",
				Remediation:  "Select a build for this version in App Store Connect",
			},
		}
	}

	var checks []CheckResult
	buildID := strings.TrimSpace(build.ID)

	if build.Expired {
		checks = append(checks, CheckResult{
			ID:           "build.invalid.expired",
			Severity:     SeverityError,
			ResourceType: "build",
			ResourceID:   buildID,
			Message:      "build is expired",
			Remediation:  "Select a non-expired build for this version in App Store Connect",
		})
	}

	state := strings.TrimSpace(build.ProcessingState)
	if state != "" && !strings.EqualFold(state, "VALID") {
		checks = append(checks, CheckResult{
			ID:           "build.invalid.processing_state",
			Severity:     SeverityError,
			Field:        "processingState",
			ResourceType: "build",
			ResourceID:   buildID,
			Message:      fmt.Sprintf("build processing state is %s", state),
			Remediation:  "Wait for build processing to complete or select a different build",
		})
	}

	return checks
}
