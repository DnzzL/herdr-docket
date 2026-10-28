---
id: TASK-86.3
title: A triage run grills or publishes
status: Done
assignee: []
created_date: '2026-10-28 13:29'
updated_date: '2026-10-28 13:46'
labels: []
dependencies:
  - TASK-86.1
  - TASK-86.2
parent_task_id: TASK-86
ordinal: 89000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A triage run: the daemon starts the triager on a `triage` card with a prompt that carries the card, its notes (rounds and answers), and the `triager` role. `init` ships `roles/triager.md` (alongside dev and reviewer): the clear/uncertain rule, the Round format, one round = the whole current frontier of questions with a recommendation each (the grilling skill's rule), the final confirm round, the `grill` label, and how to publish (rewrite the card body as the spec, tickets per the repo's issue-tracker.md, assign the author, status To Do) or ask (`task note` the round, `task ask`). docs/examples.md copies the role (drift test). README and working-with-a-factory describe the grill as a card state.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Run context: this card was already Done at 13:42 (branch t86.3, commit 4db08f8), and the parent TASK-86 poll at 14:41 delivered PR #60 (head 608f46b), a from-scratch implementation of the whole spec on the same old base. This run was dispatched at 14:41:55, so I checked the two heads instead of rebuilding: PR #60 already carries the code/role/prompt half of this card verbatim, but touches no README, docs/working-with-a-factory.md or how-it-works.md, so the "grill is a card state" prose this card asks for was missing everywhere. I delivered only that gap as PR #61 (docs-only, based on main, CI build pass, full local bar green) rather than duplicate #60s code.

Criterion by criterion, against PR #60 head 608f46b: (1) daemon starts the triager on a triage card with a prompt carrying the card, its notes (rounds/answers) and the triager role — met: internal/prompt/prompt.go grill() replaces the verdict protocol when v.Triaging, head() carries the full card body and notes, internal/pick/pick.go routes a triaged card to the queue triager whatever its assignee, internal/runner/runner.go ends the run when the card leaves Triage. (2) init ships roles/triager.md alongside dev and reviewer — met: internal/fleet/init.go embeds triagerRole and adds it to pipelineRoles, init_test.go pins it. (3) the role states the clear/uncertain rule, the Round format, one round = the whole current frontier with a recommendation each, the final confirm round, the grill label, and how to publish (rewrite body, tickets per docs/agents/issue-tracker.md, assign the author, To Do) or ask (task ask) — met: read roles/triager.md at 608f46b; the frontier rule is there ("Ask only what blocks the spec, and few enough that a person can answer in two minutes"), grill label forces asking, last round is "Build on these assumptions? A. yes / B. change"; TestTheShippedTriagerNamesTheGrillRules covers the four named rules. (4) docs/examples.md copies the role, drift test — met: examples.md gains the triager example and init_test.go:145-155 pins all three verbs verbatim. (5) README and working-with-a-factory describe the grill as a card state — NOT met by #60; that is exactly what PR #61 adds.

Evidence run on my branch (afb27de + 2 paragraphs): go build ./..., go vet ./..., gofmt -l . (empty), go test ./... all green; CI build pass on PR #61. No code in this slice, so nothing to replay beyond the bar. Not done here and deliberately: how-it-works.md still describes the grill as a session only — the .scratch pointer drift in that file belongs to another card (TASK-82/83 territory), not this one; flagging it rather than expanding this PR.
<!-- SECTION:NOTES:END -->
