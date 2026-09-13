package herdr

import (
	"errors"
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
