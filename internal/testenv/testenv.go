// Package testenv keeps a test binary away from the user's real home.
//
// The plugin resolves its config, state and skill directories from HOME and
// from Herdr's environment variables. A test that forgets to point those at a
// temp dir writes into the owner's real config — which is how a test run once
// inserted a merge-digest entry into automations.yaml (TASK-84). Answering it
// from TestMain is what makes the isolation hold for the whole binary,
// including the next test someone adds without thinking about directories.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// RealEnv, set to "real", skips the overrides so a test can reach the owner's
// real home on purpose — the live probes that need the real fleet.yaml and
// queue (LIVE_SETTLE=1). It is why the guard does not make a real-queue check
// unrunnable; the default stays isolated.
const RealEnv = "HERDR_DOCKET_TESTENV"

// Main points HOME, XDG_CONFIG_HOME and Herdr's plugin config and state dirs at
// a fresh temp dir, runs m, and removes the dir. Call it from TestMain, which
// is the only place it can cover every test in the binary:
//
//	func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
//
// When HERDR_DOCKET_TESTENV=real is set the overrides are skipped, so the
// binary sees the real environment.
func Main(m *testing.M) int {
	if os.Getenv(RealEnv) == "real" {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "herdr-docket-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)

	// HERDR_PLUGIN_CONFIG_DIR is not one of the three the ticket names, but it
	// wins over HOME inside hostpath.ConfigDir: without it a test on a machine
	// with herdr installed still reads the owner's real fleet.yaml.
	for _, kv := range [][2]string{
		{"HOME", dir},
		{"XDG_CONFIG_HOME", filepath.Join(dir, ".config")},
		{"HERDR_PLUGIN_CONFIG_DIR", filepath.Join(dir, "plugins", "config")},
		{"HERDR_PLUGIN_STATE_DIR", filepath.Join(dir, "plugins", "state")},
	} {
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			fmt.Fprintf(os.Stderr, "testenv: %v\n", err)
			return 1
		}
	}
	return m.Run()
}
