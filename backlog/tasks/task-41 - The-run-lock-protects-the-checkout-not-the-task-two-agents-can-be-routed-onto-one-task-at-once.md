---
id: TASK-41
title: >-
  The run lock protects the checkout, not the task: two agents can be routed
  onto one task at once
status: To Do
assignee: []
created_date: '2026-09-25 12:05'
updated_date: '2026-10-01 09:32'
labels: []
dependencies: []
priority: high
ordinal: 41000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
<!-- SECTION:DESCRIPTION:BEGIN -->
Observed on a live fleet: a task unassigned and mid-run under `default_agent:
dev` was re-picked by `pm` within one tick of the human changing
`default_agent: dev` to `pm` in `fleet.yaml`. Two workspaces, two agents, one
task — both runs live at once.

The pick logic is deliberate about the first half: it picks any *open* task,
because a task left `In Progress` by a dead process owes work and re-picking
is the self-heal. Its comment claims "the run lock is what actually keeps two
runs off the same task" — but the lock is keyed per `LockKey`, which is the
agent (worktree mode) or the shared checkout (root mode). Two *different*
agents on the same task hold two different keys, so the lock keeps the
*checkout* safe and says nothing about the task. The invariant pick assumes —
"one task routes to exactly one agent, so holding that agent's slot is holding
the task" — holds only while routing is stable. Default routing is re-read
every tick, and a human editing `fleet.yaml` breaks the invariant from under
a running task.

The second half is the self-heal path meeting it: `In Progress` is picked on
purpose, so after the config change the same task was both mid-run *and*
freshly routable.

Three shapes to weigh, the first favoured:

- **Lock the task, not just the checkout.** A second flock per task id
  (`run-task-<id>.lock`) beside the existing agent/checkout lock. flock dies
  with the process, which is exactly the property the self-heal path wants:
  a crashed run releases the task, a live one holds it, and no config edit
  can route a second agent onto work in flight. Cross-process too — `run` and
  the daemon race-proof on the same key.
- **Daemon-local in-flight set.** The daemon knows what it is running; pick
  skips those ids. Cheapest, but only one process deep: a manual `run` and
  the daemon still race on differently-routed agents.
- **Phase guard with an active-run oracle.** Skip `In Progress` unless the
  routed agent's run is dead — needs the same in-flight knowledge anyway and
  resurrects stale `In Progress` false-negatives (a record from a killed
  daemon looks alive until checked).

The config-edit-mid-run case is the rare one; the durable claim worth fixing
is the comment's: today two runs of one task are impossible only when the
routing doesn't change under them.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A task with a run in flight cannot be started by a second run, whatever the agents, whatever `fleet.yaml` says at that moment — pinned by a test that flips default_agent between two runs of one task
- [ ] #2 The self-heal survives: a run whose process died releases the task, and the next tick picks it up again — pinned by the existing In Progress re-pick behaviour
- [ ] #3 Cross-process holds too: `herdr-docket run TASK-x` while the daemon is routing the same task to another agent refuses rather than doubles
- [ ] #4 pick.go's comment tells the truth about which lock protects what
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: default_agent "plugin-dev" (fleet.yaml) is not a fleet agent.
<!-- SECTION:NOTES:END -->
