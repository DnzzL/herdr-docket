package host

import (
	"fmt"
	"log"
	"time"

	"github.com/DnzzL/herdr-fleet/internal/herdr"
)

// agentWork starts an interactive agent, hands it the prompt, and waits for it
// to settle. Every step verifies what actually happened rather than trusting
// the previous call's verdict: a freshly started agent is reported ready before
// it can accept input, and herdr's answers are correspondingly hedged.
type agentWork struct {
	ops   ops
	knobs knobs
	a     Spec
}

func (w agentWork) do(s Session, timeout time.Duration) error {
	if err := w.start(s); err != nil {
		return fmt.Errorf("start %s agent: %w", w.a.Agent, err)
	}
	if err := w.submit(s); err != nil {
		return err
	}
	if err := w.await(s, timeout); err != nil {
		return fmt.Errorf("waiting for the agent: %w", err)
	}
	return nil
}

// args assembles what gets forwarded to the agent executable. Model and MCP
// config are sugar over agent_args, and go in front so an explicit agent_args
// entry can still override them.
func (w agentWork) args() []string {
	args := w.a.AgentArgs
	if w.a.Model != "" {
		args = append([]string{"--model", w.a.Model}, args...)
	}
	if w.a.MCPConfig != "" {
		args = append([]string{"--mcp-config", w.a.MCPConfig}, args...)
	}
	return args
}

// start launches the agent, retrying while herdr says the pane is not a shell
// yet. An agent_not_ready answer means the start itself worked — the agent is
// in the pane but still initialising (trust dialog, MCP servers), so start
// waits for it to become idle rather than reporting a failure. Any other
// error is final — a bad agent kind or a missing binary will not fix itself,
// and retrying only delays the report.
func (w agentWork) start(s Session) error {
	deadline := time.Now().Add(w.knobs.paneReady)
	for {
		err := w.ops.AgentStart(agentName(w.a.Name, w.a.RunTag), w.a.Agent, s.PaneID, w.args())
		switch {
		case err == nil:
			return nil
		case w.ops.HasCode(err, herdr.CodeAgentNotReady):
			return w.awaitReady(s, deadline, err)
		case !w.ops.HasCode(err, herdr.CodePaneBusy):
			return err
		}
		if !time.Now().Before(deadline) {
			return err
		}
		log.Printf("pane has no shell yet, retrying agent start in %s", w.knobs.paneReadyPoll)
		time.Sleep(w.knobs.paneReadyPoll)
	}
}

// awaitReady polls the agent until it can take a prompt. startErr is what
// gets reported if the deadline passes first — the not-ready answer is more
// useful than "timed out".
func (w agentWork) awaitReady(s Session, deadline time.Time, startErr error) error {
	for {
		status, err := w.ops.AgentStatus(s.PaneID)
		if err == nil && status == "idle" {
			return nil
		}
		if !time.Now().Before(deadline) {
			return startErr
		}
		log.Printf("agent still starting (status %q), waiting %s", status, w.knobs.paneReadyPoll)
		time.Sleep(w.knobs.paneReadyPoll)
	}
}

// submit gets the prompt in front of the agent and confirms it started working.
//
// A freshly started agent is reported ready before it can actually accept
// input — it may still be connecting MCP servers, especially right after the
// machine wakes. herdr then reports agent_prompt_stalled even though the text
// reached the composer, so each step here checks the status rather than
// trusting the previous call's verdict.
func (w agentWork) submit(s Session) error {
	err := w.ops.AgentSubmit(s.PaneID, w.a.Prompt)
	if err == nil {
		return nil
	}
	if !w.ops.HasCode(err, herdr.CodeStalled) {
		return fmt.Errorf("prompt: %w", err)
	}

	// The text is probably in the composer, just not submitted. Give the agent
	// a moment, then press Enter for it, then fall back to typing it again.
	if w.working(s, w.knobs.promptSettle) {
		return nil
	}
	log.Printf("prompt stalled on %s, submitting the pending composer", s.PaneID)
	if err := w.ops.AgentSubmitPending(s.PaneID); err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	if w.working(s, w.knobs.promptSettle) {
		return nil
	}

	log.Printf("still idle on %s, retyping the prompt", s.PaneID)
	if err := w.ops.AgentSubmit(s.PaneID, w.a.Prompt); err != nil && !w.ops.HasCode(err, herdr.CodeStalled) {
		return fmt.Errorf("prompt: %w", err)
	}
	if w.working(s, w.knobs.promptSettleLast) {
		return nil
	}
	return fmt.Errorf("prompt: the agent never started working; it may still be initialising")
}

// working polls until the agent leaves idle, or the window closes.
func (w agentWork) working(s Session, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for {
		status, err := w.ops.AgentStatus(s.PaneID)
		if err == nil && status != "idle" && status != "unknown" {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(w.knobs.statusPoll)
	}
}

// await blocks until the agent settles, the run is cancelled, or the timeout
// runs out.
//
// It waits in slices rather than handing herdr the whole timeout at once. A run
// whose workspace was closed thirteen seconds in used to sit here for the full
// forty-five minutes before anyone was told, holding the automation's in-flight
// slot the whole time and skipping the next occurrence.
func (w agentWork) await(s Session, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if last != nil {
				return last
			}
			return fmt.Errorf("the agent was still working after %s", timeout)
		}
		slice := min(w.knobs.waitSlice, remaining)

		err := w.ops.AgentWait(s.PaneID, slice)
		if err == nil {
			return nil
		}
		if w.ops.HasCode(err, herdr.CodeAgentGone) {
			return ErrCancelled
		}
		// The slice expired with the agent still working, which is the normal
		// case. Keep it: if the deadline passes it is the most accurate thing
		// we have to report.
		last = err
	}
}
