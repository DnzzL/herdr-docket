Status: ready-for-agent

# The repo's own tracker is the queue

## Problem Statement

A card lives in the fleet dir's Backlog.md project (or one of four adapters)
while its spec lives in the repo's `.scratch/`. The owner reads two stores,
the plugin maintains four adapters, and a phone or `backlog browser` shows the
card but not the spec.

## Solution

[ADR 0023](../../docs/adr/0023-the-repos-own-tracker-is-the-queue.md). The
queue is the project repo's tracker, Backlog.md or GitHub. The spec is a
parent task assigned to an agent (the card); its tickets are subtasks. The
pick skips any task with a parent. `herdr-docket init <repo>` prepares a repo.
Basecamp and Notara are removed.

## Done when

A spec published with `to-spec` + `to-tickets` into a repo prepared by
`init <repo>` yields exactly one run, and its tickets yield none.

## Verify by

- `go test ./...` green, including a pick test where a parent and two
  subtasks, all To Do and assigned, yield one candidate: the parent.
- `herdr-docket init <tmp-repo>` writes `backlog/config.yml` with the fleet's
  statuses, `docs/agents/issue-tracker.md`, and `source.dir` in `fleet.yaml`.
- `grep -rni "basecamp\|notara" --include='*.go' .` finds nothing.

## Out of scope

- The triage run and the `question` status (spec 3).
- Migrating an existing `~/fleet/backlog`: the changelog says how.
