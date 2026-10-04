# The factory is an author, a verifier and a gate in code

The fleet had grown five personas handing tasks to each other through
`task assign`, a board TUI, per-agent budgets and a recovery path per way a
run could vanish. Each handoff was a state the runner had to reconcile, and
most open bugs (TASK-48, 49, 53) lived in those states. Merging was the
reviewer persona's own judgment, written as prose in its prompt.

**One author, one verifier, a fixed pipeline.** A queue names an author
(`default_agent`) and, optionally, a `verifier`. The runner — not an agent —
sequences them: the author delivers a PR (`task done --pr` leaves the task
open), the runner starts the verifier on the same task, a FAIL sends the
author back with the notes at most twice, then the task is Blocked. Agents no
longer hand work on; `task assign` stays a human verb.

**The merge decision is code.** The verifier records a verdict through the
CLI (`task verdict PASS|FAIL`), pinned to the PR's `git patch-id`. The gate
merges only when the verdict is PASS on the current patch-id, checks pass,
the task carries no `critical` label and no changed file matches the repo's
`CODEOWNERS`. Anything else holds the task Blocked and notifies. CI green
alone is not a verdict, and the author never approves its own work (after
pstack's `shipping.md`). A queue opts in with `merge: auto`; the default is
`never`.

**Critical paths live in the target repo, read by the daemon.** Branch
protection would make GitHub enforce CODEOWNERS natively, but it is absent
on private repos on the free plan and `reviewDecision` says nothing without
it — the gate would silently pass critical work. Reading `CODEOWNERS`
directly works on every plan, and the rules that hold are the base
branch's: the gate fetches the file from the forge at the ref the PR
targets, not from any checkout on disk, whose copy can be stale or missing
(TASK-57).

**Rejected:** the verifier merging after its own PASS — the gated party would
hold the gate. Agent-driven handoffs kept as they are — every handoff is a
state the runner must reconcile, which is where the recovery bugs came from.
No daemon at all (skills plus herdr-automations crons, as builder.io ships
it) — supervision and the run lock need a long-lived process, and a guarantee
that guards a merge cannot live in prose.

**Cut with it:** the board TUI (herdr's sidebar and `agent list` show live
state; `history`/`logs` show the past), per-agent budgets (one
live agent capped its runs; `agent pause` is the brake now), the pm persona (a ticket without a verification statement goes to a
human), and the vanished-agent recovery (an abnormal end is Blocked plus a
notification). `agent pause` stays: with auto-merge on, a kill switch per
agent is supervision, not machinery.

**Status:** accepted
