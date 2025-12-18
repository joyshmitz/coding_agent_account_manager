// Package exec handles running AI CLI tools with profile isolation.
package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/profile"
	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/provider"
)

// Runner executes AI CLI tools with profile isolation.
type Runner struct {
	registry *provider.Registry
}

// NewRunner creates a new runner with the given provider registry.
func NewRunner(registry *provider.Registry) *Runner {
	return &Runner{registry: registry}
}

// RunOptions configures the exec behavior.
type RunOptions struct {
	// Profile is the profile to use.
	Profile *profile.Profile

	// Provider is the provider to use.
	Provider provider.Provider

	// Args are the arguments to pass to the CLI.
	Args []string

	// WorkDir is the working directory.
	WorkDir string

	// NoLock disables profile locking.
	NoLock bool

	// Env are additional environment variables.
	Env map[string]string
}

// Run executes the AI CLI tool with profile isolation.
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	// Lock profile if not disabled
	if !opts.NoLock {
		// Use LockWithCleanup to handle stale locks from dead processes
		if err := opts.Profile.LockWithCleanup(); err != nil {
			return fmt.Errorf("lock profile: %w", err)
		}
		defer opts.Profile.Unlock()
	}

	// Get provider environment
	providerEnv, err := opts.Provider.Env(ctx, opts.Profile)
	if err != nil {
		return fmt.Errorf("get provider env: %w", err)
	}

	// Build command
	bin := opts.Provider.DefaultBin()
	cmd := exec.CommandContext(ctx, bin, opts.Args...)

	// Set up environment with deduplication (last one wins in our map logic)
	envMap := make(map[string]string)
	
	// 1. Start with inherited environment
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// 2. Apply provider environment (overrides inherited)
	for k, v := range providerEnv {
		envMap[k] = v
	}

	// 3. Apply custom environment options (overrides provider)
	for k, v := range opts.Env {
		envMap[k] = v
	}

	// Reassemble into slice
	cmd.Env = make([]string, 0, len(envMap))
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Set working directory
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	var capture *codexSessionCapture
	var stdoutObserver, stderrObserver *lineObserverWriter
	if opts.Provider.ID() == "codex" {
		capture = &codexSessionCapture{}
		stdoutObserver = newLineObserverWriter(os.Stdout, capture.ObserveLine)
		stderrObserver = newLineObserverWriter(os.Stderr, capture.ObserveLine)
	}

	// Connect stdio
	cmd.Stdin = os.Stdin
	if stdoutObserver != nil {
		cmd.Stdout = stdoutObserver
	} else {
		cmd.Stdout = os.Stdout
	}
	if stderrObserver != nil {
		cmd.Stderr = stderrObserver
	} else {
		cmd.Stderr = os.Stderr
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigChan:
				if cmd.Process != nil {
					cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()
	defer close(done)
	defer signal.Stop(sigChan)

	// Run command
	runErr := cmd.Run()
	if stdoutObserver != nil {
		stdoutObserver.Flush()
	}
	if stderrObserver != nil {
		stderrObserver.Flush()
	}

	now := time.Now()
	opts.Profile.LastUsedAt = now
	if capture != nil {
		if sessionID := capture.ID(); sessionID != "" {
			opts.Profile.LastSessionID = sessionID
			opts.Profile.LastSessionTS = now.UTC()
		}
	}
	if err := opts.Profile.Save(); err != nil {
		// Don't hide the original process exit code, but do surface metadata issues
		// when the command otherwise succeeded.
		if runErr == nil {
			return fmt.Errorf("save profile metadata: %w", err)
		}
		fmt.Fprintf(os.Stderr, "warning: failed to save profile metadata: %v\n", err)
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			// Propagate the actual exit code, but first ensure we release the lock.
			if !opts.NoLock {
				if err := opts.Profile.Unlock(); err != nil && !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "warning: failed to unlock profile: %v\n", err)
				}
			}
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("run command: %w", runErr)
	}

	return nil
}

// RunInteractive runs an interactive session with the AI CLI.
func (r *Runner) RunInteractive(ctx context.Context, opts RunOptions) error {
	return r.Run(ctx, opts)
}

// LoginFlow runs the login flow for a profile.
func (r *Runner) LoginFlow(ctx context.Context, prov provider.Provider, prof *profile.Profile) error {
	return prov.Login(ctx, prof)
}

// Status checks the authentication status of a profile.
func (r *Runner) Status(ctx context.Context, prov provider.Provider, prof *profile.Profile) (*provider.ProfileStatus, error) {
	return prov.Status(ctx, prof)
}
