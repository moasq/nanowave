package service

import (
	"testing"
	"time"
)

func TestRunLogWatchDuration(t *testing.T) {
	t.Run("default when unset", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "")
		got := runLogWatchDuration()
		want := time.Duration(defaultRunLogWatchSeconds) * time.Second
		if got != want {
			t.Fatalf("runLogWatchDuration() = %v, want %v", got, want)
		}
	})

	t.Run("zero disables log watch", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "0")
		got := runLogWatchDuration()
		if got != 0 {
			t.Fatalf("runLogWatchDuration() = %v, want 0", got)
		}
	})

	t.Run("positive value", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "12")
		got := runLogWatchDuration()
		want := 12 * time.Second
		if got != want {
			t.Fatalf("runLogWatchDuration() = %v, want %v", got, want)
		}
	})

	t.Run("follow keyword enables indefinite watch", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "follow")
		got := runLogWatchDuration()
		if got != -1 {
			t.Fatalf("runLogWatchDuration() = %v, want -1", got)
		}
	})

	t.Run("minus one enables indefinite watch", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "-1")
		got := runLogWatchDuration()
		if got != -1 {
			t.Fatalf("runLogWatchDuration() = %v, want -1", got)
		}
	})

	t.Run("invalid value falls back to default", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "invalid")
		got := runLogWatchDuration()
		want := time.Duration(defaultRunLogWatchSeconds) * time.Second
		if got != want {
			t.Fatalf("runLogWatchDuration() = %v, want %v", got, want)
		}
	})

	t.Run("negative value less than -1 falls back to default", func(t *testing.T) {
		t.Setenv("NANOWAVE_RUN_LOG_WATCH_SECONDS", "-2")
		got := runLogWatchDuration()
		want := time.Duration(defaultRunLogWatchSeconds) * time.Second
		if got != want {
			t.Fatalf("runLogWatchDuration() = %v, want %v", got, want)
		}
	})
}
