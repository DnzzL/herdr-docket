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

// realHome is HOME as this binary found it, captured before Main replaces it.
var realHome = os.Getenv("HOME")

// RealHome is the home the test binary started with, which Main has since
// pointed elsewhere. A TestMain hook that starts a child needing the owner's
// real home — the contract suite's herdr server, whose panes run the agent
// the owner actually has installed — restores just this one variable;
// everything the plugin reads stays isolated.
func RealHome() string { return realHome }

// Main points HOME, XDG_CONFIG_HOME and Herdr's plugin config and state dirs at
// a fresh temp dir, runs m, and removes the dir. Call it from TestMain, which
// is the only place it can cover every test in the binary:
//
//	func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }
//
// When HERDR_DOCKET_TESTENV=real is set the overrides are skipped, so the
// binary sees the real environment.
func Main(m *testing.M) int { return MainWith(m, nil) }

// MainWith is Main plus a hook that runs after the environment is settled and
// before any test does, returning the func that undoes it. The hook exists
// because a resource started before the overrides would answer on a different
// path than the tests then look at: the herdr contract suite starts its
// throwaway server from that hook, so the server and its clients agree on the
// temp config dir (TASK-94).
func MainWith(m *testing.M, setUp func() (tearDown func())) int {
	if os.Getenv(RealEnv) == "real" {
		return runIsolated(m, setUp)
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
	return runIsolated(m, setUp)
}

// runIsolated runs the hook, then the tests, then the teardown — in that order,
// so a tearDown that removes what setUp created still has the environment.
func runIsolated(m *testing.M, setUp func() (tearDown func())) int {
	if setUp != nil {
		if tearDown := setUp(); tearDown != nil {
			defer tearDown()
		}
	}
	return m.Run()
}
