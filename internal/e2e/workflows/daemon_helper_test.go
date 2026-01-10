package workflows

import (
	"os"
	"testing"

	"github.com/Dicklesworthstone/coding_agent_account_manager/cmd/caam/cmd"
)

// TestDaemonHelper is used to run the daemon in a subprocess.
// It is invoked by other tests with -test.run=TestDaemonHelper.
func TestDaemonHelper(t *testing.T) {
	if os.Getenv("GO_WANT_DAEMON_HELPER") != "1" {
		return
	}

	// Set up args to simulate "caam daemon start --fg"
	// We need to use cmd.Execute() but we can't easily pass args to it directly if it uses os.Args?
	// cobra uses os.Args[1:] by default.
	// But we can set args on the root command if we had access to it.
	// cmd.Execute() uses rootCmd.
	
	// Since we can't modify rootCmd args easily from here without exposing it,
	// we will rely on os.Args being set by the caller?
	// No, the caller calls the test binary.
	
	// We can set os.Args manually.
	// But we must preserve the 0th arg (program name) and overwrite the rest.
	// IMPORTANT: The test runner adds flags like -test.run which might confuse cobra if not cleared.
	os.Args = []string{"caam", "daemon", "start", "--fg", "--verbose"}
	
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
