---
id: TASK-36
title: >-
  docs/factory.md wires the loop end to end
status: To Do
assignee: []
created_date: '2026-09-21 19:12'
labels: []
dependencies:
  - TASK-31
  - TASK-33
  - TASK-34
ordinal: 36000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The loop is real once its pieces exist, and the record of how they compose is what makes it a product rather than personas per repo. docs/factory.md is that record: the whole cycle — intake filing, the dev carrying a PR to green, the review sweep, the stall sweep, the lookback — with the queue as the only thing any stage shares. It names the dials that decide how much autonomy is in the room (statuses gates, budgets, budgeted verbs) and the schedules that drive each persona, so a user can copy the factory onto their own fleet in one read.

It also states the refusals the loop inherits — the ones that keep it a queue and not a platform: **fleet code never merges — agents do.** The merge decision is a persona rule the owner edits in `AGENT.md`, the same slot the video calls an approval policy; ADR 0008 stands untouched because the board never closes a task with a verdict, and it never merges either. No event triggers, per the automations plugin's own ADR; no second-model oversight persona except where a schedule's `model:` line is the honest version of one. Where the video said monorepos, beta/stable couplings, or strong telemetry: those are the user's app's concerns, not the plugin's, and the doc says so once.

The personas are in docs/examples.md and stay there — this document links to them and never pastes a second copy: one copy, one home. TASK-37 (the one-step init) builds on what this doc fixes, so it is blocked by this one on the parts that rewrite what the doc fixed.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 docs/factory.md exists: the cycle end to end, one diagram or table, each stage naming its persona and its schedule
- [ ] #2 The refusals have their own section: fleet code never merges — who merges is whose persona it is (ADR 0008 intact); event triggers out, telemetry and repo-layout guidance pointed at the app rather than bought here
- [ ] #3 Every persona the doc describes lives in docs/examples.md and the doc links rather than repeats
- [ ] #4 The README links the doc from its How-much-autonomy section, as the worked version of the levers it already lists
<!-- AC:END -->
