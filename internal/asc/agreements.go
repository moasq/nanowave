package asc

import "encoding/json"

// ParseAgreements parses the JSON output from `asc agreements list`.
// Returns (allGood, agreements).
func ParseAgreements(data []byte) (bool, []Agreement) {
	// ASC CLI returns an envelope: {"data": [{"attributes": {"status": "..."}}]}
	var env Envelope
	if json.Unmarshal(data, &env) != nil {
		return true, nil // unparseable = skip gracefully
	}
	agreements := make([]Agreement, 0, len(env.Data))
	for _, d := range env.Data {
		var a Agreement
		if json.Unmarshal(d.Attributes, &a) == nil {
			agreements = append(agreements, a)
		}
	}
	for _, a := range agreements {
		switch a.Status {
		case "ACTIVE":
			// OK
		default:
			// Any non-ACTIVE status (EXPIRED, PENDING, etc.) means action needed
			return false, agreements
		}
	}
	return true, agreements
}
