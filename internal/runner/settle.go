package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/DnzzL/herdr-docket/internal/history"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// settleEvery is the cadence of the settle pass. The queue is polled every
// tick (15s), but a pull request moves on a human's clock and every read of
// one is a trip through the forge — per tick that is four subprocesses per
// held task, a few times a minute, to learn nothing new.
const settleEvery = 3 * time.Minute

// Settle lets a held task's pull request end it. Two endings outlive the run
// while a PR is still out — a hold (Blocked) and a task closed failed — and
// both are read here through the forge, and settled by the PR's own state:
// MERGED closes the task done (a hold whose PR the human merged, and a task
// failed whose PR shipped anyway); CLOSED unmerged says in its note that a
// human closed it and settles the hold failed; OPEN leaves the task standing.
// The fleet never merges here — settling reads what a human already decided.
//
// The daemon calls this every tick with the queue as that tick read it; the
// cadence is this method's own — first call due (a daemon settling what is
// already true), then one per settleEvery however often the tick asks. A PR
// that cannot be read is logged and left alone: an unreadable state is
// evidence that does not settle, and that stays a human's.
func (r *Runner) Settle(src work.Source, tasks []work.Task) {
	if !r.settleDue() {
		return
	}
	for _, t := range tasks {
		if !settles(t) {
			continue
		}
		pr, err := history.TaskPullRequest(t.ID)
		if err != nil {
			log.Printf("%s: read the task's pull request: %v", t.ID, err)
			continue
		}
		if pr == "" {
			continue // nothing out on a forge: no state to settle with
		}
		p, err := r.forge.PR(pr)
		if err != nil {
			log.Printf("%s: the pull request could not be read: %v", t.ID, err)
			continue
		}
		switch p.State {
		case "MERGED":
			r.note(src, t.ID, fmt.Sprintf("fleet: %s is merged — the pull request settles this task done.", pr))
			if err := src.Close(t.ID, work.Done); err != nil {
				log.Printf("%s: close as done: %v", t.ID, err)
			}
		case "CLOSED":
			// A task already failed says so, and has nothing to wait for
			// any more; a hold is the ending that waits, so the note about
			// the human closing it is the hold's to carry.
			if !t.Blocked {
				continue
			}
			r.note(src, t.ID, fmt.Sprintf("fleet: %s was closed unmerged by a human — the pull request settles this task failed.", pr))
			if err := src.Close(t.ID, work.Failed); err != nil {
				log.Printf("%s: close as failed: %v", t.ID, err)
			}
		}
	}
}

// settles reports the tasks a pull request can still end: one parked on a
// human (Blocked) and one closed failed. Every other task owes nothing to a
// forge — an open task is a run's to close, a done one is over.
func settles(t work.Task) bool { return t.Blocked || t.Verdict == work.Failed }

// settleDue reports whether this call is the settle pass, and marks it taken:
// the first one, then one per settleEvery — the tick can ask as often as it
// likes and the forge is read every few minutes, not every tick.
func (r *Runner) settleDue() bool {
	r.settleMu.Lock()
	defer r.settleMu.Unlock()
	now := r.now()
	if !r.lastSettle.IsZero() && now.Sub(r.lastSettle) < settleEvery {
		return false
	}
	r.lastSettle = now
	return true
}
