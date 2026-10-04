package runner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/gate"
	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/host"
	"github.com/DnzzL/herdr-docket/internal/work"
)

const (
	// maxRounds is how many verdicts a PR gets before a human does: past a
	// second FAIL the problem is the ticket, not the code.
	maxRounds = 2
	// checksWait bounds how long the gate waits on pending CI, polled every
	// checksPoll.
	checksWait = 15 * time.Minute
	checksPoll = 30 * time.Second
	// slotPoll is how often a stage retries an agent another run holds.
	slotPoll = 10 * time.Second
)

// pipeline takes a delivered PR from verdict to merge or to a human (ADR
// 0013): the verifier judges, a FAIL sends the author back with the notes,
// a PASS goes to the gate. It runs under the task lock Run holds.
func (r *Runner) pipeline(src work.Source, t work.Task, author fleet.Agent, pr string) error {
	pipe := r.settings.PipelineFor(t.ID)
	if pipe.Verifier == "" {
		return r.hold(src, t, pr, "the daemon read fleet.yaml before this queue named a verifier — restart it")
	}
	verifier, ok := r.agents()[pipe.Verifier]
	if !ok {
		return r.hold(src, t, pr, fmt.Sprintf("verifier %q is not a fleet agent", pipe.Verifier))
	}
	for round := 1; ; round++ {
		v, err := r.stage(src, t, verifier, history.TriggerVerify, pr)
		if err != nil || !v.delivered {
			return err // the attempt has already said why on the task
		}
		if v.verdict == gate.Pass {
			return r.gate(src, t, author, pr, v.patch, pipe.AutoMerge)
		}
		if round >= maxRounds {
			return r.hold(src, t, pr, fmt.Sprintf("the verifier failed it %d times", round))
		}
		w, err := r.stage(src, t, author, history.TriggerRework, pr)
		if err != nil || !w.delivered {
			return err
		}
		pr = w.pr
	}
}

// stage runs one attempt in the agent's own slot, waiting for it while
// another run holds it, up to the agent's timeout.
func (r *Runner) stage(src work.Source, t work.Task, a fleet.Agent, trigger history.Trigger, pr string) (outcome, error) {
	key := LockKey(a)
	deadline := time.Now().Add(time.Duration(a.TimeoutMinutes) * time.Minute)
	for {
		if _, ok := r.acquire(key); ok {
			break
		}
		if time.Now().After(deadline) {
			return outcome{}, r.hold(src, t, pr, fmt.Sprintf("%s stayed busy with another run", a.Name))
		}
		r.sleep(slotPoll)
	}
	defer r.release(key)
	return r.attempt(src, t, a, trigger, pr)
}

// gate waits out pending CI, then merges or holds on gate.Decide's answer.
func (r *Runner) gate(src work.Source, t work.Task, author fleet.Agent, url, patch string, auto bool) error {
	var pr gate.PR
	for waited := time.Duration(0); ; waited += checksPoll {
		var err error
		if pr, err = r.forge.PR(url); err != nil {
			return r.hold(src, t, url, "the pull request could not be read: "+err.Error())
		}
		if pr.Checks != gate.ChecksPending || waited >= checksWait {
			break
		}
		r.sleep(checksPoll)
	}
	task, err := src.Get(t.ID)
	if err != nil {
		return r.hold(src, t, url, "the task could not be re-read for its labels: "+err.Error())
	}
	merge, reason := gate.Decide(gate.Input{
		PR: pr, Verdict: gate.Pass, VerdictPatch: patch,
		Labels: task.Labels, Owners: r.owners(author.Workdir), Auto: auto,
	})
	if !merge {
		return r.hold(src, t, url, reason)
	}
	if err := r.forge.Merge(url, pr.HeadSHA); err != nil {
		return r.hold(src, t, url, "the merge was refused: "+err.Error())
	}
	// The label is a debt; this merge pays it. Whatever an earlier hold left
	// on the PR, it must not read as owed once the PR is in.
	if err := r.forge.RemoveLabel(url, mergeNeededLabel); err != nil {
		log.Printf("%s: unmark %s: %v", t.ID, url, err)
	}
	r.note(src, t.ID, fmt.Sprintf("fleet: merged %s — verified PASS on patch %s, CI green.", url, patch))
	if err := src.Close(t.ID, work.Done); err != nil {
		log.Printf("%s: close as done: %v", t.ID, err)
	}
	if err := r.host.Notify("fleet: merged — "+t.ID, url, host.SoundNone); err != nil {
		log.Printf("%s: notify: %v", t.ID, err)
	}
	return nil
}

// hold parks the task on a human with the one reason the fleet stopped. A
// hold is an outcome, not a failure of the run: nothing broke — and what it
// parks on is a PR only a human can move, so it is mergeNeeded's announcement
// that raises, not a failure's.
func (r *Runner) hold(src work.Source, t work.Task, pr, reason string) error {
	r.note(src, t.ID, fmt.Sprintf("fleet: %s is held for a human — %s.", pr, reason))
	if err := src.Close(t.ID, work.Blocked); err != nil {
		log.Printf("%s: close as blocked: %v", t.ID, err)
	}
	r.mergeNeeded(t, pr, reason)
	return nil
}

func (r *Runner) note(src work.Source, id, text string) {
	if err := src.Comment(id, text); err != nil {
		log.Printf("%s: append note: %v", id, err)
	}
}

// codeowners reads the CODEOWNERS patterns of the repo the author works, from
// the three places GitHub looks (the roster has already expanded ~). No
// file is no protected path.
func codeowners(workdir string) []string {
	for _, rel := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		if raw, err := os.ReadFile(filepath.Join(workdir, rel)); err == nil {
			return gate.ParseCodeowners(string(raw))
		}
	}
	return nil
}
