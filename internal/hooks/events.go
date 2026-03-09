package hooks

// Hook event constants. These are the lifecycle events that can trigger hooks.
const (
	// Build events (swiftship)
	EventBuildStart          = "build.start"
	EventBuildAnalyzeDone    = "build.analyze.done"
	EventBuildPlanDone       = "build.plan.done"
	EventBuildGenerateDone   = "build.generate.done"
	EventBuildCompileStart   = "build.compile.start"
	EventBuildCompileSuccess = "build.compile.success"
	EventBuildCompileFailure = "build.compile.failure"
	EventBuildFixStart       = "build.fix.start"
	EventBuildFixDone        = "build.fix.done"
	EventBuildRunStart       = "build.run.start"
	EventBuildRunSuccess     = "build.run.success"
	EventBuildDone           = "build.done"

	// Legacy aliases (kept for backward compat with existing configs)
	EventBuildFail = "build.fail"

	// Store events (appstore)
	EventStoreUploadStart     = "store.upload.start"
	EventStoreUploadDone      = "store.upload.done"
	EventStoreUploadFailure   = "store.upload.failure"
	EventStoreProcessingStart = "store.processing.start"
	EventStoreProcessingDone  = "store.processing.done"
	EventStoreTestflightDist  = "store.testflight.distribute"
	EventStoreSubmitStart     = "store.submit.start"
	EventStoreSubmitDone      = "store.submit.done"
	EventStoreSubmitFailure   = "store.submit.failure"
	EventStoreReviewStatus    = "store.review.status"
	EventStoreReviewApproved  = "store.review.approved"
	EventStoreReviewRejected  = "store.review.rejected"
	EventStoreReleaseDone     = "store.release.done"
	EventStoreValidatePass    = "store.validate.pass"
	EventStoreValidateFail    = "store.validate.fail"

	// Legacy aliases
	EventStoreUploadFail     = "store.upload.fail"
	EventStoreSubmitFail     = "store.submit.fail"
	EventStoreRelease        = "store.release"
	EventStoreReviewStart    = "store.review.start"
	EventStoreReviewDone     = "store.review.done"
	EventStoreTestflightDone = "store.testflight.done"

	// Pipeline events
	EventPipelineStart    = "pipeline.start"
	EventPipelineStepDone = "pipeline.step.done"
	EventPipelineDone     = "pipeline.done"
	EventPipelineFailure  = "pipeline.failure"

	// Legacy aliases
	EventWorkflowStart = "workflow.start"
	EventWorkflowDone  = "workflow.done"
	EventWorkflowFail  = "workflow.fail"
	EventWorkflowStep  = "workflow.step"
)

// HookContext carries the event name and key-value variables for template substitution.
type HookContext struct {
	Event string
	Vars  map[string]string
}

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	// dryRunKey is the context key for dry-run mode.
	dryRunKey contextKey = iota
)
