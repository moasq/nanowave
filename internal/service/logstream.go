package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moasq/nanowave/internal/terminal"
)

const defaultRunLogWatchSeconds = 30

func runLogWatchDuration() time.Duration {
	raw := strings.TrimSpace(os.Getenv("NANOWAVE_RUN_LOG_WATCH_SECONDS"))
	if raw == "" {
		return time.Duration(defaultRunLogWatchSeconds) * time.Second
	}

	if strings.EqualFold(raw, "follow") {
		return -1
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < -1 {
		return time.Duration(defaultRunLogWatchSeconds) * time.Second
	}

	if seconds == -1 {
		return -1
	}

	return time.Duration(seconds) * time.Second
}

func streamSimulatorLogs(ctx context.Context, processName, bundleID string, duration time.Duration) error {
	if duration <= 0 {
		if duration == 0 {
			return nil
		}
	}

	watchCtx := ctx
	cancel := func() {}
	if duration > 0 {
		watchCtx, cancel = context.WithTimeout(ctx, duration)
	}
	defer cancel()

	predicate := fmt.Sprintf(`process == "%s"`, processName)
	if strings.TrimSpace(bundleID) != "" {
		predicate = fmt.Sprintf(`process == "%s" OR subsystem CONTAINS[c] "%s"`, processName, bundleID)
	}

	cmd := exec.CommandContext(watchCtx, "xcrun", "simctl", "spawn", "booted", "log", "stream",
		"--style", "compact",
		"--level", "debug",
		"--predicate", predicate,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to read log stream stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to read log stream stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start simulator log stream: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamLogReader(stdout, false, &wg)
	go streamLogReader(stderr, true, &wg)

	waitErr := cmd.Wait()
	wg.Wait()

	if watchCtx.Err() == context.DeadlineExceeded {
		terminal.Info("Stopped log streaming.")
		return nil
	}
	if watchCtx.Err() == context.Canceled {
		return nil
	}
	if waitErr != nil {
		return fmt.Errorf("simulator log stream failed: %w", waitErr)
	}

	return nil
}

// streamMacOSLogs streams native macOS logs using `log stream` directly (no simctl).
func streamMacOSLogs(ctx context.Context, processName, bundleID string, duration time.Duration) error {
	if duration <= 0 {
		if duration == 0 {
			return nil
		}
	}

	watchCtx := ctx
	cancel := func() {}
	if duration > 0 {
		watchCtx, cancel = context.WithTimeout(ctx, duration)
	}
	defer cancel()

	predicate := fmt.Sprintf(`process == "%s"`, processName)
	if strings.TrimSpace(bundleID) != "" {
		predicate = fmt.Sprintf(`process == "%s" OR subsystem CONTAINS[c] "%s"`, processName, bundleID)
	}

	cmd := exec.CommandContext(watchCtx, "log", "stream",
		"--style", "compact",
		"--level", "debug",
		"--predicate", predicate,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to read log stream stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to read log stream stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start macOS log stream: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamLogReader(stdout, false, &wg)
	go streamLogReader(stderr, true, &wg)

	waitErr := cmd.Wait()
	wg.Wait()

	if watchCtx.Err() == context.DeadlineExceeded {
		terminal.Info("Stopped log streaming.")
		return nil
	}
	if watchCtx.Err() == context.Canceled {
		return nil
	}
	if waitErr != nil {
		return fmt.Errorf("macOS log stream failed: %w", waitErr)
	}

	return nil
}

func streamLogReader(r io.Reader, isErr bool, wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if isErr {
			terminal.OutputLine(fmt.Sprintf("  %s[sim-log]%s %s", terminal.Dim, terminal.Reset, line))
			continue
		}
		terminal.OutputLine(fmt.Sprintf("  %s[sim-log]%s %s", terminal.Dim, terminal.Reset, line))
	}
}

func (s *Service) stopBackgroundLogStreaming() {
	s.logWatchMu.Lock()
	cancel := s.logWatchCancel
	s.logWatchCancel = nil
	s.logWatchMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *Service) startBackgroundLogStreaming(stream func(context.Context, string, string, time.Duration) error, processName, bundleID string, duration time.Duration) {
	s.stopBackgroundLogStreaming()

	if duration == 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.logWatchMu.Lock()
	s.logWatchSeq++
	seq := s.logWatchSeq
	s.logWatchCancel = cancel
	s.logWatchMu.Unlock()

	go func() {
		err := stream(ctx, processName, bundleID, duration)

		s.logWatchMu.Lock()
		if s.logWatchSeq == seq {
			s.logWatchCancel = nil
		}
		s.logWatchMu.Unlock()

		if err != nil && ctx.Err() == nil {
			terminal.Warning(fmt.Sprintf("Log streaming unavailable: %v", err))
		}
	}()
}
