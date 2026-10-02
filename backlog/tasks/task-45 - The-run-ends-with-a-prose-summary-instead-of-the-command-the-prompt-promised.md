---
id: TASK-45
title: >-
  The run ends with a prose summary instead of the command the prompt promised
status: To Do
assignee: []
created_date: '2026-10-02 11:55'
labels: []
dependencies: []
ordinal: 45000
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The fleet's most frequent failure, observed across every model used so far:
the run does its work, states its conclusion in the closing notes or the PR
thread — "Now closing per merge policy" — and then ends the turn with prose
instead of executing the command. pi stops, `reconcile` records *settled
without reporting a verdict*, and a clean run is filed `failed`. On 2026-09-25
alone this hit five-plus runs across dev and reviewer; on 2026-10-02 the
review of PR 42 concluded correctly and was still filed as a failure.

Two halves, both in the prompt (internal/prompt/prompt.go), both prose-level
and testable:

1. **The last tool call must be the protocol.** The required section lists the
   commands but never says when they must happen relative to the agent's final
   message. Models treat the summary as the end. The prompt should state the
   ordering: execute the closing or handoff command first, then summarise — a
   prose ending with no command is a failed run, by construction.

2. **Handoff is a first-class ending.** The section is headed "close the task
   exactly once, reporting exactly one verdict", which reads as mandatory even
   though `task assign` is documented as "the one way to end a run without a
   verdict". Personas that hand off (dev → reviewer) lose to the imperative.
   The required section should name both endings and defer the choice to the
   task and the persona: close with a verdict, or hand on — never neither.

The queue absorbed every instance so far (auto-failed into the human column,
notes intact), which is why this is a hardening task and not a data-loss one —
but each failure misattributes a good run as bad and burns budget against the
cap (and, until TASK-44, double-counted).

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The prompt states the ordering: the closing or handoff command is the run's final tool call, and a prose summary that ends a run without it is a failed run — a prompt test pins the wording
- [ ] #2 The required section presents `done|fail|block` and `assign` as the two endings, deferring which to the task and persona, and a prompt test pins that both appear as endings rather than one as an aside
- [ ] #3 The prompt test also pins that neither ending is presented as optional-with-nothing — the "leave it open and unassigned" outcome stays explicitly named as the failure it is
<!-- AC:END -->
