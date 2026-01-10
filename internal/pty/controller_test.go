//go:build unix

package pty

import (
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewController(t *testing.T) {
	t.Logf("[TEST] Platform: %s/%s", runtime.GOOS, runtime.GOARCH)

	t.Run("nil command returns error", func(t *testing.T) {
		_, err := NewController(nil, nil)
		if err == nil {
			t.Error("expected error for nil command")
		}
		t.Logf("[TEST] nil command error: %v", err)
	})

	t.Run("valid command with default options", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		ctrl, err := NewController(cmd, nil)
		if err != nil {
			t.Fatalf("NewController failed: %v", err)
		}
		defer ctrl.Close()
		t.Log("[TEST] Controller created with default options")
	})

	t.Run("valid command with custom options", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		opts := &Options{
			Rows: 40,
			Cols: 120,
		}
		ctrl, err := NewController(cmd, opts)
		if err != nil {
			t.Fatalf("NewController failed: %v", err)
		}
		defer ctrl.Close()
		t.Logf("[TEST] Controller created with custom options: rows=%d, cols=%d", opts.Rows, opts.Cols)
	})
}

func TestNewControllerFromArgs(t *testing.T) {
	t.Run("creates controller from args", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("echo", []string{"hello"}, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()
		t.Log("[TEST] Controller created from args")
	})
}

func TestControllerStart(t *testing.T) {
	t.Run("starts command successfully", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		err = ctrl.Start()
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		t.Log("[TEST] Command started successfully")

		// Verify we got a valid file descriptor
		fd := ctrl.Fd()
		if fd < 0 {
			t.Errorf("expected valid fd, got %d", fd)
		}
		t.Logf("[TEST] PTY fd: %d", fd)
	})

	t.Run("double start returns error", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("First Start failed: %v", err)
		}

		err = ctrl.Start()
		if err == nil {
			t.Error("expected error on double start")
		}
		t.Logf("[TEST] Double start error: %v", err)
	})
}

func TestControllerInjectCommand(t *testing.T) {
	t.Run("injects command into cat", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		t.Log("[TEST] Injecting 'hello world'")
		err = ctrl.InjectCommand("hello world")
		if err != nil {
			t.Fatalf("InjectCommand failed: %v", err)
		}

		// Give cat time to echo back
		time.Sleep(50 * time.Millisecond)

		output, err := ctrl.ReadOutput()
		if err != nil {
			t.Fatalf("ReadOutput failed: %v", err)
		}
		t.Logf("[TEST] Output: %q", output)

		if !strings.Contains(output, "hello world") {
			t.Errorf("expected output to contain 'hello world', got %q", output)
		}
	})

	t.Run("inject before start returns error", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		err = ctrl.InjectCommand("hello")
		if err == nil {
			t.Error("expected error when injecting before start")
		}
		t.Logf("[TEST] Inject before start error: %v", err)
	})
}

func TestControllerInjectRaw(t *testing.T) {
	t.Run("injects raw bytes", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		// Inject without newline
		t.Log("[TEST] Injecting raw bytes 'test'")
		err = ctrl.InjectRaw([]byte("test"))
		if err != nil {
			t.Fatalf("InjectRaw failed: %v", err)
		}

		// Now send newline
		err = ctrl.InjectRaw([]byte("\n"))
		if err != nil {
			t.Fatalf("InjectRaw newline failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		output, err := ctrl.ReadOutput()
		if err != nil {
			t.Fatalf("ReadOutput failed: %v", err)
		}
		t.Logf("[TEST] Output: %q", output)

		if !strings.Contains(output, "test") {
			t.Errorf("expected output to contain 'test', got %q", output)
		}
	})
}

func TestControllerWaitForPattern(t *testing.T) {
	t.Run("finds pattern in output", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("sh", []string{"-c", "echo 'START'; sleep 0.1; echo 'PATTERN_FOUND'; sleep 0.1; echo 'END'"}, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		pattern := regexp.MustCompile("PATTERN_FOUND")
		t.Logf("[TEST] Waiting for pattern: %s", pattern)

		ctx := context.Background()
		output, err := ctrl.WaitForPattern(ctx, pattern, 5*time.Second)
		if err != nil {
			t.Fatalf("WaitForPattern failed: %v", err)
		}
		t.Logf("[TEST] Matched output: %q", output)

		if !pattern.MatchString(output) {
			t.Errorf("pattern not found in output: %q", output)
		}
	})

	t.Run("timeout when pattern not found", func(t *testing.T) {
		// Use a command that outputs but doesn't match, and keeps running
		ctrl, err := NewControllerFromArgs("sh", []string{"-c", "echo 'NO_MATCH'; sleep 10"}, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		pattern := regexp.MustCompile("NEVER_EXISTS")
		t.Logf("[TEST] Waiting for pattern that won't match: %s", pattern)

		ctx := context.Background()
		_, err = ctrl.WaitForPattern(ctx, pattern, 200*time.Millisecond)
		// Should timeout since pattern doesn't match and process keeps running
		if err != ErrTimeout {
			t.Errorf("expected ErrTimeout, got %v", err)
		}
		t.Logf("[TEST] Got expected timeout error: %v", err)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("sh", []string{"-c", "sleep 10"}, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		pattern := regexp.MustCompile("NEVER")

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		t.Log("[TEST] Waiting with context that will be cancelled")
		start := time.Now()
		_, err = ctrl.WaitForPattern(ctx, pattern, 10*time.Second)
		elapsed := time.Since(start)

		// The function should return quickly after context cancellation
		// (within ~200ms, not the full 10s timeout)
		if elapsed > 500*time.Millisecond {
			t.Errorf("WaitForPattern took too long after cancel: %v", elapsed)
		}

		// Accept context.Canceled as the error
		if err != context.Canceled {
			t.Logf("[TEST] Got error %v instead of context.Canceled (acceptable)", err)
		} else {
			t.Logf("[TEST] Got expected cancellation error: %v", err)
		}
	})
}

func TestControllerWait(t *testing.T) {
	t.Run("returns exit code 0 on success", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("true", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		exitCode, err := ctrl.Wait()
		if err != nil {
			t.Fatalf("Wait failed: %v", err)
		}
		t.Logf("[TEST] Exit code: %d", exitCode)

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d", exitCode)
		}
	})

	t.Run("returns non-zero exit code on failure", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("false", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		exitCode, err := ctrl.Wait()
		if err != nil {
			t.Fatalf("Wait failed: %v", err)
		}
		t.Logf("[TEST] Exit code: %d", exitCode)

		if exitCode == 0 {
			t.Errorf("expected non-zero exit code, got %d", exitCode)
		}
	})
}

func TestControllerSignal(t *testing.T) {
	t.Run("sends SIGTERM to running process", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("sleep", []string{"10"}, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		t.Log("[TEST] Sending SIGTERM")
		err = ctrl.Signal(SIGTERM)
		if err != nil {
			t.Fatalf("Signal failed: %v", err)
		}

		// Process should exit due to signal
		exitCode, err := ctrl.Wait()
		t.Logf("[TEST] Exit code after signal: %d, err: %v", exitCode, err)
	})
}

func TestControllerClose(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		t.Log("[TEST] Closing controller")
		err = ctrl.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Double close should be safe
		err = ctrl.Close()
		if err != nil {
			t.Errorf("Double close failed: %v", err)
		}
		t.Log("[TEST] Double close succeeded")
	})

	t.Run("operations fail after close", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		ctrl.Close()

		err = ctrl.InjectCommand("test")
		if err != ErrClosed {
			t.Errorf("expected ErrClosed, got %v", err)
		}
		t.Logf("[TEST] InjectCommand after close error: %v", err)
	})
}

func TestControllerFd(t *testing.T) {
	t.Run("returns -1 before start", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		fd := ctrl.Fd()
		if fd != -1 {
			t.Errorf("expected fd -1 before start, got %d", fd)
		}
		t.Logf("[TEST] Fd before start: %d", fd)
	})

	t.Run("returns valid fd after start", func(t *testing.T) {
		ctrl, err := NewControllerFromArgs("cat", nil, nil)
		if err != nil {
			t.Fatalf("NewControllerFromArgs failed: %v", err)
		}
		defer ctrl.Close()

		if err := ctrl.Start(); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		fd := ctrl.Fd()
		if fd < 0 {
			t.Errorf("expected valid fd after start, got %d", fd)
		}
		t.Logf("[TEST] Fd after start: %d", fd)
	})
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.Rows != 24 {
		t.Errorf("expected Rows=24, got %d", opts.Rows)
	}
	if opts.Cols != 80 {
		t.Errorf("expected Cols=80, got %d", opts.Cols)
	}
	t.Logf("[TEST] Default options: rows=%d, cols=%d", opts.Rows, opts.Cols)
}
