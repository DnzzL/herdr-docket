package herdr

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/DnzzL/herdr-docket/internal/hostpath"
)

// This suite holds the boundary the rest of the tests agree to pretend about.
// The bug that made it necessary: herdr prints its error envelope on stderr,
// the client read stdout, and every HasCode branch in the codebase was dead
// for months while a thorough suite stayed green — because every test built
// its APIError by hand (or printed the envelope on the wrong stream) and
// nothing ever crossed the boundary where herdr's actual bytes arrive.
//
// So these tests exec the real herdr binary, like the client does, and pin
// what herdr 0.9.x actually emits for each shape the fleet branches on. That
// makes them the loud counterpart to a question nobody had asked: not
// "does the code handle this error", but "does herdr still send this error".
// When herdr changes an answer, these go red and the constants and the
// branches that read them are renegotiated in the same change — not after.
//
// They are skipped, not failed, where herdr is not installed: the fake-shaped
// unit tests hold the read logic; this suite holds the facts about herdr.
// Everything here owns what it touches — workspaces this suite opens are
// closed by it, and nobody else's pane, agent or workspace is contacted.

func requireHerdr(t *testing.T) {
	t.Helper()
	bin := hostpath.Bin()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("herdr (%s) is not installed: %v — the contract suite holds against the real binary only; the unit tests stand in for it", bin, err)
	}
	t.Logf("herdr contract held against %s", bin)
}

// TestTheClientReadsTheCodeOutOfARealHerdrError is the test that would have
// gone red on 12 September. No bytes of the answer are written by this test:
// herdr provides them. It forces the cheapest real error the binary can emit
// (a workspace id it has never issued) and asserts the code the fleet's
// cleanup and focus paths branch on arrives through the client — in whatever
// stream herdr prints the envelope on.
func TestTheClientReadsTheCodeOutOfARealHerdrError(t *testing.T) {
	requireHerdr(t)

	var c Client
	gone := fmt.Sprintf("contract-gone-%d", time.Now().UnixNano())
	err := run(nil, "workspace", "close", gone)
	if !HasCode(err, CodeWorkspaceGone) {
		t.Fatalf("workspace close of an unknown id: HasCode(CodeWorkspaceGone) = false: %v — the fleet's vanished-workspace recovery cannot fire", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeWorkspaceGone || apiErr.Message == "" {
		t.Fatalf("expected a real herdr envelope with the code and a message, got %#v", apiErr)
	}
	// And the wrapped answer ships intact.
	if err := c.WorkspaceClose(gone); !errors.Is(err, ErrGone) {
		t.Fatalf("WorkspaceClose of the same unknown id: %v, want ErrGone", err)
	}
}

// A vanished agent is what herdr answers agent_not_found to — for every shape
// the fleet's vanished-agent recovery was written for. Probed against 0.9.1,
// one shape per line:
//
//   - a target that never registered,
//   - a registered agent whose workspace was closed under it (the exact
//     shape CodeAgentGone existed for),
//   - a registered agent whose process exited on its own in a live
//     workspace, and
//   - an agent that did not survive its own boot (one-shot, no readiness).
//
// All four answer agent_not_found; agent_not_running — CodeAgentGone's old
// value, still present in the binary — was not reachable through any of
// them, so the recovery branches on nothing herdr emits. The contract was
// renegotiated in TICKET-48's change: CodeAgentGone now pins the code herdr
// actually sends, and this test holds both halves of that agreement — the
// code arrives, and the codes that mean other things do not answer for it.
func TestHerdrAnswersAVanishedAgentWithAgentNotFound(t *testing.T) {
	requireHerdr(t)

	var c Client
	target := fmt.Sprintf("contract-missing-%d", time.Now().UnixNano())
	_, statusErr := c.AgentStatus(target)
	if !HasCode(statusErr, CodeAgentGone) {
		t.Fatalf("agent get of a vanished agent: %v, want the %s envelope read intact", statusErr, CodeAgentGone)
	}
	var apiErr *APIError
	if !errors.As(statusErr, &apiErr) || apiErr.Code != CodeAgentGone || apiErr.Message == "" {
		t.Fatalf("expected a real herdr envelope with the code and a message, got %#v", apiErr)
	}
	// The constants that mean other things must NOT answer for this shape,
	// or the wrong recovery fires for it.
	for _, not := range []string{CodeAgentNotReady, CodeStalled, CodePaneBusy, CodeWorkspaceGone} {
		if HasCode(statusErr, not) {
			t.Errorf("the %s envelope must not read as %s", CodeAgentGone, not)
		}
	}
	// wait and prompt answer the same way: the await slice maps the wait
	// answer to ErrCancelled, and a bare prompt must not be retried as if
	// the agent were still there to receive it.
	if err := c.AgentWait(target, time.Second); !HasCode(err, CodeAgentGone) {
		t.Errorf("agent wait of a vanished agent: %v, want %s", err, CodeAgentGone)
	}
}

// agent_pane_busy is herdr's refusal to start an agent in a pane that is not
// sitting at a shell prompt. Forced for real: a pane of a throwaway workspace
// already holding an agent. The retry behind it (the sleep-delayed workspace
// started while the machine was waking) is the recovery that never ran once
// in production before the fix.
func TestARealBusyPaneRefusesAnAgentStartWithTheCodeTheFleetRetriesOn(t *testing.T) {
	requireHerdr(t)

	var c Client
	workspaceID, paneID, err := c.WorkspaceCreate(t.TempDir(), "docket contract")
	if err != nil {
		t.Fatalf("own throwaway workspace: %v", err)
	}
	t.Cleanup(func() { _ = c.WorkspaceClose(workspaceID) })

	// The shell has to be up first: a pane mid-boot answers agent_not_ready,
	// a different shape, and would make this test race what it means to pin.
	if err := paneInteractive(t, c, paneID); err != nil {
		t.Fatalf("the pane never got interactive: %v", err)
	}
	if err := c.AgentStart("docket-contract-first", "pi", paneID, nil); err != nil {
		t.Fatalf("first agent start should succeed on an up shell: %v", err)
	}
	// The second start is the busy one: the pane is already an agent's.
	err = c.AgentStart("docket-contract-second", "pi", paneID, nil)
	if !HasCode(err, CodePaneBusy) {
		t.Fatalf("agent start into an occupied pane: HasCode(CodePaneBusy) = false: %v — the start retry would not fire", err)
	}
}

// paneInteractive blocks until the pane's shell has spawned — the threshold
// herdr's own agent start waits for (its --timeout default is 30s), so the
// test never races the pane it is pinning.
func paneInteractive(t *testing.T, c Client, paneID string) error {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for {
		_, lastErr = c.PaneRead(paneID, 5)
		if lastErr == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return lastErr
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// The shapes the fleet branches on that this suite does NOT force, and why.
// Each is an agent-lifecycle state, and forcing one honestly means racing a
// live agent's boot: the test would be flakier than the code it pins, and a
// flaky contract suite teaches people to ignore the contract.
//
//   - agent_prompt_stalled: herdr emits it only when an accepted submission
//     observes no working/blocked state within 5s (its own agent prompt
//     help spells the rule out). Probed against 0.9.x, an agent that accepts
//     a prompt it will not act on answers `timeout`, not stalled — stalled
//     is reachable only by racing an agent mid-boot. The unit suite holds the
//     client-side shape (TestAnErrorCodeIsReadFromWhicheverStreamCarriesIt).
//   - agent_not_ready: exists only in the seconds an agent is booting; the
//     honest forcing needs a live agent whose readiness herdr hedges on.
//
