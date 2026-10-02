// Package prompt assembles what the agent is told: the fleet's shared brief,
// the agent's persona, the task in full, and the closing protocol — how to
// report back into the queue. Its core is pure text in, text out, so the one
// contract the whole system depends on is pinned by a test.
package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DnzzL/herdr-docket/internal/fleet"
	"github.com/DnzzL/herdr-docket/internal/work"
)

// Assemble builds the run prompt for one task. fleetDir is where the fleet
// lives — roster, wiring, backend config, and the optional shared brief
// (FLEET.md). The agent works in its own workdir, so the prompt names the
// fleet dir for context only: every call it makes goes through the fleet CLI,
// which finds the queue on its own.
//
// FLEET.md, when present, is the company-wide brief: its body is prepended to
// the persona, so what is true of every agent lives in one file instead of
// being repeated in each AGENT.md. No file, or an empty one, and the prompt is
// exactly what it was before the file existed.
func Assemble(a fleet.Agent, v work.Task, fleetDir string, words fleet.Words) string {
	return assemble(a, v, readBrief(fleetDir), fleetDir, words)
}

// Runnable rewrites a prompt's command lines to an invocation this machine can
// actually run, and is applied by whoever hands the prompt to an agent rather
// than by Assemble — asking where the binary lives is a question about the
// world, and Assemble answers only questions about the fleet.
func Runnable(s string) string {
	exe, _ := os.Executable()
	return runnable(s, cliFor(exe))
}

// readBrief is FLEET.md's body, or "" when there is none worth reading. A
// missing, empty or all-comment brief is no brief: it is optional, and never a
// reason a run cannot be assembled.
func readBrief(fleetDir string) string {
	raw, err := os.ReadFile(filepath.Join(fleetDir, "FLEET.md"))
	if err != nil {
		return ""
	}
	brief := strings.TrimSpace(string(raw))
	if brief == "" || allComments(brief) {
		return ""
	}
	return brief
}

// allComments reports whether every line is a comment. The file `init`
// scaffolds is nothing but comments, and an untouched scaffold is meant to be
// invisible: the brief starts speaking only once a human writes real prose
// into it.
func allComments(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return false
	}
	return true
}

// assemble is the pure core: the brief in, the prompt out. Reading the file
// outside it is what keeps the one contract the whole system depends on
// pinned by a test.
func assemble(a fleet.Agent, v work.Task, brief, fleetDir string, words fleet.Words) string {
	return head(a, v, brief) + protocol(a, v, fleetDir, words, false)
}

// Deliver is Assemble for a queue with a verifier (ADR 0013): an author's pull
// request is its delivery, and the task stays open for the verifier.
// pr names the author's own earlier delivery when a verifier sent it back.
func Deliver(a fleet.Agent, v work.Task, pr, fleetDir string, words fleet.Words) string {
	return deliver(a, v, pr, readBrief(fleetDir), fleetDir, words)
}

func deliver(a fleet.Agent, v work.Task, pr, brief, fleetDir string, words fleet.Words) string {
	rework := ""
	if pr != "" {
		rework = fmt.Sprintf(`## Your pull request came back

%s is your delivery for this task, and the verifier failed it — its notes
are the newest above. Work on that PR's branch (`+"`gh pr checkout %s`"+`),
fix what the verifier found, push, and deliver the same PR again.

`, pr, pr)
	}
	return head(a, v, brief) + rework + protocol(a, v, fleetDir, words, true)
}

// Verify is the verifier's prompt: the same layers and task, the pull request
// that claims to deliver it, and the one command that records a verdict. The
// verifier neither merges nor closes — the runner reads the verdict and the
// merge gate decides.
func Verify(a fleet.Agent, v work.Task, pr, fleetDir string) string {
	return verify(a, v, pr, readBrief(fleetDir), fleetDir)
}

func verify(a fleet.Agent, v work.Task, pr, brief, fleetDir string) string {
	return head(a, v, brief) + fmt.Sprintf(`## The pull request to verify

%s claims to deliver this task. You did not write it, and your verdict is the
only thing standing between it and main. Re-derive every acceptance criterion
yourself, on the real surface: replay the verification the task states, run
it, read the result. The author's description, its checkboxes and green CI
are claims, not evidence. A criterion you could not check is not met.

## When you are done — required

Record exactly one verdict on that pull request, with the evidence — what
you ran, what you saw, file and line for every finding:

  herdr-docket task verdict %s PASS --pr "%s" --note "<what you checked and how>"
  herdr-docket task verdict %s FAIL --pr "%s" --note "<what fails, and the fix you would make>"

A FAIL goes back to the author with your note as its brief, so make it
actionable. You never merge, never push to the branch, and never close or
re-route the task: the fleet does that from your verdict. (The fleet dir,
for context, is %s.)
`, pr, v.ID, pr, v.ID, pr, fleetDir)
}

// head is everything a run is told before how to report: the layers, the
// persona and the task in full.
func head(a fleet.Agent, v work.Task, brief string) string {
	var b strings.Builder
	// Three layers, widest first: what is true of the fleet, then of the role,
	// then of this post. Each one may be absent, and an absent layer changes
	// nothing else.
	for _, layer := range []string{brief, a.RoleBrief} {
		if layer != "" {
			b.WriteString(layer)
			b.WriteString("\n\n")
		}
	}
	b.WriteString(a.Persona)
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "# Your task: %s — %s\n\n", v.ID, v.Title)
	if v.Body != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", v.Body)
	}
	if len(v.Criteria) > 0 {
		b.WriteString("## Acceptance criteria\n\n")
		for _, c := range v.Criteria {
			box := "[ ]"
			if c.Checked {
				box = "[x]"
			}
			fmt.Fprintf(&b, "- %s #%d %s\n", box, c.Index, c.Text)
		}
		b.WriteString("\nThese boxes are the queue's record, not yours to edit. Your verdict on\neach one — met or not, and the evidence — belongs in the note that closes the\ntask.\n\n")
	}
	if v.Notes != "" {
		fmt.Fprintf(&b, "## Notes from previous runs\n\n%s\n\n", v.Notes)
	}
	return b.String()
}

// protocol is how an agent reports back. delivered is a queue with a
// verifier: a PR leaves the task open, and handing work on is not the
// agent's move.
func protocol(a fleet.Agent, v work.Task, fleetDir string, words fleet.Words, delivered bool) string {
	var b strings.Builder

	// A prefixed id (myapp/TASK-12) means the fleet works several queues, and
	// a follow-up has to land in the queue this task came from. The prefix is
	// the source, so the prompt writes it in and the agent never has to know a
	// second queue exists.
	createSource := ""
	if i := strings.IndexByte(v.ID, '/'); i > 0 {
		createSource = " -s " + v.ID[:i]
	}

	fmt.Fprintf(&b, `## Verify, then end the run — required, exactly once

Your own task answers to the herdr-docket CLI and nothing else — never touch it
by hand, whichever board it happens to live on. (The fleet dir, for context, is
%s: agents and the shared brief, not the queue.)

Say what happened as you go, so the next reader has the thread:

  herdr-docket task note %s "<what you did and why>"

Read the task and the acceptance criteria once more before you end: the note
carries your verdict on each one — met or not, and the evidence — and every
conclusion you draw belongs in a task note, not only in chat. What you say in
this conversation the queue never receives; the queue hears the note.

## Ending the run — required, exactly once

%s

The ending command is the last tool call of the run: **execute it, then
summarise**. If your run has already said its conclusion, and stops there,
the queue receives nothing: a run that ends with a message and no command is the
failure mode this section exists to prevent.

**End with a verdict on your own task** — done says the work landed, fail
says it did not, block hands the decision back to a human:

  herdr-docket task done %s --note "<what you did, which criteria you met, and how you know>"
  herdr-docket task fail %s --note "<why you could not do it>"
  herdr-docket task block %s --note "<what a human must decide or unblock>"

%s%s

Always execute one, and the same ending, and only one — your task, once,
however the run ends and however the budget went, and nothing standing before
it. No ending is optional-with-nothing: do not end by leaving it open
and unassigned. That is not a choice among these; it is the failed run the
queue files by default (task open, agent gone, verdict unreported). When your
run's budget is nearly spent, this is the command you cut everything else to
run.

Do not touch other agents' tasks.

If you edit this project's board directly — triage, a status another task
should sit in — use the word this project accepts, not the fleet's idea of it.
%s
This run has a time budget. If the task is too big to finish well within it,
do one coherent slice, record exactly where you stopped in the closing note,
then create the follow-up task for the rest and close this one done — a
finished slice with a good handoff beats a timed-out marathon.

If you find follow-up work, create a task for it instead of expanding this one:

  herdr-docket task create "<title>" -d "<what and why>" -a %s%s
`, fleetDir, v.ID, endingsIntro(delivered), v.ID, v.ID, v.ID,
		prParagraph(v.ID, delivered), handOn(v.ID, delivered), wordList(words), a.Name, createSource)
	return b.String()
}

// endingsIntro says how many ways a run can end where the queue has no
// verifier: two — a verdict, or a handoff — with the choice deferred to the
// task and the persona. A verified queue's worker delivers rather than hands
// on, so it is told its one ending instead of a second it must not use.
func endingsIntro(delivered bool) string {
	if delivered {
		return `This queue verifies before it merges: this run ends with a verdict on
your own task — done delivers your pull request, and the verifier judges it.`
	}
	return `A run ends one of two ways: close the task with a verdict, or hand it on to
another agent. Decide from the task and your own persona which one fits.`
}

// prParagraph says what a pull request on the closing command means here.
func prParagraph(id string, delivered bool) string {
	if !delivered {
		return `If your work went out as a pull request, put it on the closing command:

  herdr-docket task done <id> --pr "<the PR url>" --note "..."

The PR is how the run history records where the work landed — a url in prose
alone is one the fleet cannot read.
`
	}
	return fmt.Sprintf(`This queue verifies before it merges. When your work is a pull request, end
with it on the done command — CI green, the task's verification replayed,
the evidence in the PR description:

  herdr-docket task done %s --pr "<the PR url>" --note "<what you verified and how>"

The task stays open: a verifier that did not write the code judges the PR,
and the fleet either merges it or brings it back to you with the verifier's
notes. Never merge it yourself.
`, id)
}

// handOn is the second ending, standing beside the verdict rather than as an
// aside: the run hands the task to the agent that finishes it. Except where
// the runner sequences the work — a PR leaves the task open, and handing work
// on is not the agent's move.
func handOn(id string, delivered bool) string {
	if delivered {
		return ""
	}
	return fmt.Sprintf(`
**Hand it on to another agent** — the work was specced, somebody else
builds it, or the task's own brief names a persona whose correct ending is a
handoff (a dev whose pull request awaits review, a PM who specs work the dev
builds):

  herdr-docket task assign %s <agent>

The task stays open with its whole history in one place, and the fleet routes
it to that agent on the next tick.`, id)
}

// wordList is the queue's own status words, as the prompt states them. Only
// the ones the fleet writes: a board's human columns are not the fleet's to
// teach, and a backend with no statuses at all is simply not told about any.
func wordList(w fleet.Words) string {
	if !w.Known() {
		return "(This queue has no status words: a task is open or it is closed.)"
	}
	var b strings.Builder
	for _, l := range []struct{ what, word string }{
		{"work this fleet may pick up", w.Todo},
		{"work a run has in hand", w.InProgress},
		{"a task that ended done", w.Done},
		{"a run that failed", w.Failed},
		{"a task blocked on a human", w.Blocked},
	} {
		if l.word != "" {
			fmt.Fprintf(&b, "  %-28s %s\n", l.what+":", l.word)
		}
	}
	return b.String()
}

// cliFor is the invocation the prompt should tell an agent to use: the bare
// name when a pane's PATH already resolves it, and otherwise the absolute
// path of the binary writing the prompt.
//
// This exists because the plugin is not on a pane's PATH. Every run was told
// to close itself with `herdr-docket task done`, every run that reached the
// end found no such command, and the fleet recorded the silence that followed
// as the agent's failure. The binary knows where it lives; saying so is the
// difference between a protocol and a suggestion.
func cliFor(exe string) string {
	if _, err := exec.LookPath("herdr-docket"); err == nil {
		return "herdr-docket"
	}
	if exe == "" {
		return "herdr-docket"
	}
	if abs, err := filepath.Abs(exe); err == nil {
		return abs
	}
	return "herdr-docket"
}

// runnable rewrites the prompt's command lines to an invocation that works,
// and leaves its prose alone: an agent copies the indented lines, and a
// sentence naming an absolute path reads worse without helping anybody.
func runnable(s, cli string) string {
	if cli == "herdr-docket" {
		return s
	}
	return strings.ReplaceAll(s, "\n  herdr-docket ", "\n  "+cli+" ")
}
