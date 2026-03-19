package validation

import (
	"fmt"
	"unicode/utf8"
)

// MetadataLengthIssue describes one over-limit metadata field.
type MetadataLengthIssue struct {
	Field  string
	Length int
	Limit  int
}

// VersionLocalizationLengthIssues returns over-limit fields for one version localization.
func VersionLocalizationLengthIssues(loc VersionLocalization) []MetadataLengthIssue {
	return metadataLengthIssues([]metadataLengthField{
		{field: "description", value: loc.Description, limit: LimitDescription},
		{field: "keywords", value: loc.Keywords, limit: LimitKeywords},
		{field: "whatsNew", value: loc.WhatsNew, limit: LimitWhatsNew},
		{field: "promotionalText", value: loc.PromotionalText, limit: LimitPromotionalText},
	})
}

// AppInfoLocalizationLengthIssues returns over-limit fields for one app-info localization.
func AppInfoLocalizationLengthIssues(loc AppInfoLocalization) []MetadataLengthIssue {
	return metadataLengthIssues([]metadataLengthField{
		{field: "name", value: loc.Name, limit: LimitName},
		{field: "subtitle", value: loc.Subtitle, limit: LimitSubtitle},
	})
}

type metadataLengthField struct {
	field string
	value string
	limit int
}

func metadataLengthIssues(fields []metadataLengthField) []MetadataLengthIssue {
	issues := make([]MetadataLengthIssue, 0, len(fields))
	for _, field := range fields {
		length := utf8.RuneCountInString(field.value)
		if length <= field.limit {
			continue
		}
		issues = append(issues, MetadataLengthIssue{
			Field:  field.field,
			Length: length,
			Limit:  field.limit,
		})
	}
	return issues
}

func metadataLengthChecks(versionLocs []VersionLocalization, appInfoLocs []AppInfoLocalization) []CheckResult {
	var checks []CheckResult

	for _, loc := range versionLocs {
		for _, issue := range VersionLocalizationLengthIssues(loc) {
			switch issue.Field {
			case "description":
				checks = append(checks, CheckResult{
					ID:           "metadata.length.description",
					Severity:     SeverityError,
					Locale:       loc.Locale,
					Field:        "description",
					ResourceType: "appStoreVersionLocalization",
					ResourceID:   loc.ID,
					Message:      fmt.Sprintf("description exceeds %d characters", issue.Limit),
					Remediation:  fmt.Sprintf("Shorten description to %d characters or fewer", issue.Limit),
				})
			case "keywords":
				checks = append(checks, CheckResult{
					ID:           "metadata.length.keywords",
					Severity:     SeverityError,
					Locale:       loc.Locale,
					Field:        "keywords",
					ResourceType: "appStoreVersionLocalization",
					ResourceID:   loc.ID,
					Message:      fmt.Sprintf("keywords exceed %d characters", issue.Limit),
					Remediation:  fmt.Sprintf("Shorten keywords to %d characters or fewer", issue.Limit),
				})
			case "whatsNew":
				checks = append(checks, CheckResult{
					ID:           "metadata.length.whats_new",
					Severity:     SeverityError,
					Locale:       loc.Locale,
					Field:        "whatsNew",
					ResourceType: "appStoreVersionLocalization",
					ResourceID:   loc.ID,
					Message:      fmt.Sprintf("what's new exceeds %d characters", issue.Limit),
					Remediation:  fmt.Sprintf("Shorten what's new to %d characters or fewer", issue.Limit),
				})
			case "promotionalText":
				checks = append(checks, CheckResult{
					ID:           "metadata.length.promotional_text",
					Severity:     SeverityError,
					Locale:       loc.Locale,
					Field:        "promotionalText",
					ResourceType: "appStoreVersionLocalization",
					ResourceID:   loc.ID,
					Message:      fmt.Sprintf("promotional text exceeds %d characters", issue.Limit),
					Remediation:  fmt.Sprintf("Shorten promotional text to %d characters or fewer", issue.Limit),
				})
			}
		}
	}

	for _, loc := range appInfoLocs {
		for _, issue := range AppInfoLocalizationLengthIssues(loc) {
			switch issue.Field {
			case "name":
				checks = append(checks, CheckResult{
					ID:           "metadata.length.name",
					Severity:     SeverityError,
					Locale:       loc.Locale,
					Field:        "name",
					ResourceType: "appInfoLocalization",
					ResourceID:   loc.ID,
					Message:      fmt.Sprintf("name exceeds %d characters", issue.Limit),
					Remediation:  fmt.Sprintf("Shorten name to %d characters or fewer", issue.Limit),
				})
			case "subtitle":
				checks = append(checks, CheckResult{
					ID:           "metadata.length.subtitle",
					Severity:     SeverityError,
					Locale:       loc.Locale,
					Field:        "subtitle",
					ResourceType: "appInfoLocalization",
					ResourceID:   loc.ID,
					Message:      fmt.Sprintf("subtitle exceeds %d characters", issue.Limit),
					Remediation:  fmt.Sprintf("Shorten subtitle to %d characters or fewer", issue.Limit),
				})
			}
		}
	}

	return checks
}
