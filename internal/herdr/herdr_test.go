package herdr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAPIErrorPrefersHerdrsOwnCode(t *testing.T) {
	body := []byte(`{"error":{"code":"agent_pane_busy","message":"pane w1T:p1 is not an available shell"},"id":"cli:agent:start"}`)
	err := newAPIError([]string{"agent", "start", "triage"}, body, "", errors.New("exit status 1"))

	if !HasCode(err, CodePaneBusy) {
		t.Fatalf("code not recovered from %v", err)
	}
	want := "agent start: agent_pane_busy: pane w1T:p1 is not an available shell"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestNewAPIErrorFallsBackToTheExecError(t *testing.T) {
	// The failure that made a run undiagnosable: herdr exits non-zero and
	// prints nothing, so the only thing left to report is why the process died.
	err := newAPIError([]string{"worktree", "create"}, nil, "", errors.New("exit status 1"))

	want := "worktree create: exit status 1"
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestNewAPIErrorKeepsStderrOverTheExecError(t *testing.T) {
	// "exit status 1" says less than whatever herdr wrote, so it loses.
	err := newAPIError([]string{"worktree", "create"}, nil,
		"  fatal: not a git repository\n", errors.New("exit status 128"))

	want := "worktree create: fatal: not a git repository"
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestNewAPIErrorSurvivesANilExecError(t *testing.T) {
	if err := newAPIError([]string{"pane", "read"}, nil, "", nil); err == nil {
		t.Fatal("want an error even with nothing to say")
	}
}

// herdr prints its error envelope on stderr, not stdout. Reading only stdout
// left Code empty on every error the fleet ever saw, which turned every
// HasCode branch into dead code — the prompt-stall recovery gave up on its
// first attempt while its error text still read correctly, so nothing showed.
func TestAnErrorCodeIsReadFromWhicheverStreamCarriesIt(t *testing.T) {
	envelope := `{"error":{"code":"agent_prompt_stalled","message":"no observed working state"},"id":"cli:agent:prompt"}`
	for _, tc := range []struct {
		name           string
		stdout, stderr string
	}{
		{"on stderr, which is where herdr puts it", "", envelope},
		{"on stdout, in case that ever changes", envelope, ""},
	} {
		err := newAPIError([]string{"agent", "prompt"}, []byte(tc.stdout), tc.stderr, nil)
		if !HasCode(err, CodeStalled) {
			t.Errorf("%s: HasCode = false, got %v", tc.name, err)
		}
		if strings.Contains(err.Error(), "{") {
			t.Errorf("%s: the raw envelope leaked into the message: %v", tc.name, err)
		}
	}
}

// fakeHerdr stands in for the herdr binary at the process boundary the fleet
// actually crosses. hostpath.Bin reads HERDR_BIN_PATH per call, so the client
// under test is the one that ships, exec and all: what the script prints is
// what herdr prints, on the stream herdr prints it on.
func fakeHerdr(t *testing.T, stderr string, code int) string {
	t.Helper()
	script := "#!/bin/sh\n"
	if stderr != "" {
		script += "cat >&2 <<'JSON'\n" + stderr + "\nJSON\n"
	}
	script += fmt.Sprintf("exit %d\n", code)
	path := filepath.Join(t.TempDir(), "herdr")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// Closing a workspace that is already gone is not a failure, but it is a fact
// — and the two calls that meet the same herdr code have to report it the same
// way. Focus reported it and WorkspaceClose swallowed it, which left the pane
// unable to tell "I closed it" from "there was nothing left to close".
func TestClosingAWorkspaceThatIsGoneSaysSoTheWayFocusDoes(t *testing.T) {
	gone := `{"error":{"code":"workspace_not_found","message":"no workspace wR:p9"},"id":"cli:workspace:close"}`
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, gone, 1))

	var c Client
	if err := c.WorkspaceClose("wR:p9"); !errors.Is(err, ErrGone) {
		t.Fatalf("got %v, want ErrGone", err)
	}
}

func TestClosingAWorkspaceThatIsThereReportsNothing(t *testing.T) {
	t.Setenv("HERDR_BIN_PATH", fakeHerdr(t, "", 0))

	var c Client
	if err := c.WorkspaceClose("wR:p9"); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
}

// Anything that is not an envelope still has to produce a readable line rather
// than an empty one — that is what this function existed for first.
func TestANonEnvelopeErrorStillReads(t *testing.T) {
	err := newAPIError([]string{"worktree", "create"}, nil, "boom: no space left", nil)
	if HasCode(err, CodeStalled) {
		t.Error("a message with no code must not match a code")
	}
	if !strings.Contains(err.Error(), "no space left") {
		t.Errorf("the message must survive, got %v", err)
	}
}
