# The unit of work is a spec, run by one implementer

The failure this answers: one run per task meant one worktree, one branch and
one PR per task. The runner owned their lifecycle, and that state was where the
bugs lived: a verify run sat on the human's checkout (TASK-65), an author's
branch and registration outlived the run (TASK-68), a sweep had to retire what
earlier runs left (TASK-69). An idea worth a dozen tasks became a dozen PRs to
verify and gate one by one.

**A card is a spec.** An idea is grilled with the human, then written with
Pocock's `to-spec` and `to-tickets`, unmodified, in their local mode: spec
and ticket files (blocking edges as text) committed on the queue's base
branch. The card points to the spec and carries the spec's own `Done when` /
`Verify by`. `work.Task` does not change: the ticket graph is the run's
business, not the queue's, so no adapter learns parents or dependencies.

**One implementer per repo runs it.** The author's run becomes
`/implement-spec` on the spec: an integration branch, one subagent per ready
ticket in its own worktree, a final `code-review`, one PR. Worktrees are the
agent runtime's, not the runner's. Docket stays runtime-agnostic: the prompt
asks for subagents, and a runtime without them works the graph alone. In the
software-factory language (mattpocock/skills#1316) the daemon is the
orchestrator; this run is an implementer task, not an orchestrator.

**`.scratch` is a plan, not a tracker.** The card is the only ticket that
triggers a run; the ticket files are the implementer's plan and trigger
nothing.

**The rest of ADR 0013 holds, at spec size.** A separate verifier run checks
every ticket's `Verify by` and the spec's against the one PR. A FAIL starts a
new implementer run on the same integration branch, twice at most, then
Blocked. The gate stays code and reads `CODEOWNERS` at the PR's base ref; the
run lock stays a flock (herdr workspace labels are not unique, so a workspace
cannot be the lock).

**Rejected.** *Tickets as cards, routed one by one*: keeps the per-task
worktree state this removes, and the ticket graph would need parent and
dependency fields on `work.Task` and in all four frozen adapters. *The graph
in the card body, written by a docket skill*: forks what Pocock's skills
already publish locally. *GitHub native auto-merge instead of the gate*: puts
the merge policy in one forge and needs a paid plan for branch protection on
private repos (ADR 0013). *A planner agent splitting the spec after the
grill*: re-derives context the grill session already holds.

**Status:** accepted
