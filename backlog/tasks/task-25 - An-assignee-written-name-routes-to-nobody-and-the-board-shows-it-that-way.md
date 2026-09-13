---
id: TASK-25
title: 'An assignee written @name routes to nobody, and the board shows it that way'
status: To Do
assignee:
  - fleet-dev
created_date: '2026-09-13 20:41'
labels: []
dependencies: []
priority: high
ordinal: 25000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`dishnow-pm` filed four tickets tonight and assigned every one of them to `@dishnow-dev`. The fleet showed all four as `@dishnow-dev (unknown!)` and routed none of them: `pick` looks up `agents["@dishnow-dev"]` against folder names in `agents/`, which have no `@`. Four ready tickets sat unroutable until a human stripped the character by hand.

The agent was not being careless. `backlog task view` *displays* assignees as `@dishnow-dev`, and `@name` is how a person writes an assignee anywhere else. Both spellings mean the same agent, and only one of them works.

`notara/NOT-130` and `NOT-122` have carried `@thomas (unknown!)` since this morning for the same reason — there the name really is unknown, but the fleet cannot even say so correctly.

## Where the fix belongs

In the backlogmd adapter, not in `pick`. ADR 0001 says the core owns the vocabulary and an adapter translates its backend's shape on the way in; `@` is Backlog.md's display convention for a person, and it should stop at the adapter exactly as its status words do. `first()` in `internal/work/backlogmd/backlogmd.go` is the one place every assignee passes through on the way out.

Weigh it against the alternative before you write it: normalising in `pick.AssigneeFor` would cover every backend at once, including one that has not been written yet. The argument against is that it teaches the core a convention it should not know. Say which you picked and why.

Whichever way: `@thomas` must still come out as a name the board reports as unknown, not as something silently dropped. An assignee the fleet cannot match is a real signal and it must survive the normalisation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A task assigned @name is routed to the agent named name, and a test pins both spellings
- [ ] #2 An assignee that matches no agent is still reported as unknown, with the name readable — the @ is normalised, not the signal
- [ ] #3 The choice between normalising in the adapter and normalising in pick is stated with its reason, in the code or an ADR
<!-- AC:END -->
