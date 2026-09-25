---
id: TASK-44
title: >-
  UsageSince counts closing records, not runs — a PR-attach copy doubles the spend
status: To Do
assignee: []
created_date: '2026-09-25 16:25'
labels: []
dependencies: []
ordinal: 44000
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Observed on a live fleet as `dev 5/4 runs today` against a cap of 4 — while
the agent had run exactly four tasks. `UsageSince` totals every record whose
status `closes()`, but history is append-only with one *latest* record per
run: when an agent closes late with `--pr`, `SetPullRequest` copies the
task's newest record — which may already be a closing record — and the copy
is a second closing record for the same `RunID`. The spend counts twice.

The display lie is the small half. The damage is at the boundary: `OverBudget`
reads `runs >= cap`, so a phantom run makes the fleet treat a half-spent
agent as spent — with a cap of 4 and one duplication, the agent effectively
gets three. The budget, which exists so a background loop keeps running,
starts starving the loop instead.

`Runs()` already collapses to the latest record per RunID; `UsageSince` does
not. The budget display also says "today" while the window is a rolling 24
hours — the two disagree about what is being counted.

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A run whose PR-attach appended a copy of an already-closing record counts once against the budget — a test pins two closing records on one RunID as one spent run
- [ ] #2 The rollover window and the roster/board wording agree: the line says what the window is, not "today", or the window becomes the day it names — either way one meaning
- [ ] #3 A real fifth run under a cap of four is still refused — the fix must not widen the cap while narrowing the count
<!-- AC:END -->
