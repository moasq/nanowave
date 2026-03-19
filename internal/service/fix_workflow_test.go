package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildOutputExcerptKeepsRecentRawOutput(t *testing.T) {
	long := strings.Repeat("header\n", 4000) + strings.Repeat("tail\n", 3500) + "recent failure context\n"
	got := buildOutputExcerpt([]byte(long))

	if !strings.Contains(got, "[xcodebuild output truncated to the last 32000 characters]") {
		t.Fatalf("expected truncation marker, got %q", got)
	}
	if !strings.Contains(got, "recent failure context") {
		t.Fatalf("expected recent raw output to be preserved, got %q", got)
	}
}

func TestBuildFixPromptIncludesFailureContext(t *testing.T) {
	specs := []xcodeBuildSpec{
		{
			label:    "Demo (iOS)",
			platform: "ios",
			args:     []string{"-project", "Demo.xcodeproj", "-scheme", "Demo", "-destination", "generic/platform=iOS", "-quiet", "build"},
		},
	}
	failure := buildFailure{
		spec:   specs[0],
		err:    errors.New("exit status 65"),
		output: []byte("/tmp/Demo.swift:42:10: error: broken"),
	}

	appendPrompt, userMsg := buildFixPrompt("Demo", specs, failure)

	if !strings.Contains(appendPrompt, "CLI Fix Mode") {
		t.Fatalf("expected append prompt to include CLI fix instructions, got %q", appendPrompt)
	}
	if !strings.Contains(userMsg, "Failing validation target: Demo (iOS)") {
		t.Fatalf("expected failing target in prompt, got %q", userMsg)
	}
	if !strings.Contains(userMsg, "xcodebuild -project Demo.xcodeproj") {
		t.Fatalf("expected exact xcodebuild command in prompt, got %q", userMsg)
	}
	if !strings.Contains(userMsg, "error: broken") {
		t.Fatalf("expected build output in prompt, got %q", userMsg)
	}
}

func TestRunBuildFixLoopStopsAfterSuccessfulRetry(t *testing.T) {
	spec := xcodeBuildSpec{label: "Demo", platform: "ios", args: []string{"build"}}
	initialFailure := &buildFailure{spec: spec, output: []byte("error"), err: errors.New("exit status 65")}

	verifyCalls := 0
	failure, attempts, err := runBuildFixLoop(context.Background(), initialFailure, []xcodeBuildSpec{spec}, func(context.Context, xcodeBuildSpec) ([]byte, error) {
		verifyCalls++
		if verifyCalls == 1 {
			return []byte("still broken"), errors.New("exit status 65")
		}
		return nil, nil
	}, 3, func(context.Context, buildFailure, int) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failure != nil {
		t.Fatalf("expected success after retry, got failure: %+v", failure)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRunBuildFixLoopReturnsApplyFixError(t *testing.T) {
	spec := xcodeBuildSpec{label: "Demo", platform: "ios", args: []string{"build"}}
	initialFailure := &buildFailure{spec: spec, output: []byte("error"), err: errors.New("exit status 65")}
	wantErr := errors.New("claude failed")

	failure, attempts, err := runBuildFixLoop(context.Background(), initialFailure, []xcodeBuildSpec{spec}, func(context.Context, xcodeBuildSpec) ([]byte, error) {
		t.Fatal("verify runner should not be called when applyFix fails")
		return nil, nil
	}, 3, func(context.Context, buildFailure, int) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if failure == nil || failure.spec.label != spec.label {
		t.Fatalf("expected current failure to be returned, got %+v", failure)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
