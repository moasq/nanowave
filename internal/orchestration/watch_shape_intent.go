package orchestration

// normalizeWatchShapeIntentHints keeps watch/tvOS hints internally consistent.
// It does not infer intent from prompt text; it only normalizes router output.
func normalizeWatchShapeIntentHints(decision *IntentDecision) {
	if decision == nil {
		return
	}

	if decision.PlatformHint == PlatformWatchOS {
		// watchOS does not use iPhone/iPad device_family hints.
		decision.DeviceFamilyHint = ""
		if decision.WatchProjectShapeHint == "" {
			decision.WatchProjectShapeHint = WatchShapeStandalone
		}
		return
	}

	if decision.PlatformHint == PlatformTvOS {
		// tvOS does not use device_family or watch project shape hints.
		decision.DeviceFamilyHint = ""
		decision.WatchProjectShapeHint = ""
		return
	}

	// Non-watch routes must not carry watch-only shape hints.
	decision.WatchProjectShapeHint = ""
}
