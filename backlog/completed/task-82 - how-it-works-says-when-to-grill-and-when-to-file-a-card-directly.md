---
id: TASK-82
title: how-it-works says when to grill and when to file a card directly
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-22 17:32'
updated_date: '2026-10-22 17:36'
labels: []
dependencies: []
ordinal: 82000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
docs/how-it-works.md, section "From idea to card", only describes the grill path. The owner's rule: a path that is unclear goes through a grill (grill → to-spec → to-tickets → a card with a Spec: line); a clear task goes straight to a card with its own Done when / Verify by, no spec, worked with /tdd. Example of the clear path: TASK-81.

Add that rule to the section, in the doc's voice; mirror it in README's How it works only if README describes the intake. Docs state what is, not history.

Done when: a reader of how-it-works knows which of the two paths an idea takes.

Verify by: go test ./... green; read the section: both paths named, with the condition that picks one.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Edited docs/how-it-works.md section 'From idea to card': opens with the condition (is the path to the result clear), the grill path is labelled 'Unclear → grill it into a spec', and a new paragraph names 'Clear → file the card itself' — its own Done when / Verify by, no Spec: line, worked as one ticket with /tdd, nothing under .scratch. Mirrored in README 'How it works' items 1-2 (that section does describe the intake). Fragment: changelog.d/TASK-82.md. Criteria: (1) reader knows which path an idea takes — met, both paths and the selecting condition are in the section; (2) go test ./... green — met, plus go build ./..., go vet ./..., gofmt -l . empty; the docs/examples.md role-drift test still passes since no shipped bytes changed. PR #54.

docs/how-it-works.md 'From idea to card' now names both paths and the condition that picks one: unclear → grill → to-spec → to-tickets → a card with a Spec: line; clear → a card with its own Done when / Verify by, no Spec:, worked with /tdd. Mirrored in README's How it works (items 1-2) and fragment changelog.d/TASK-82.md. Verified: go build ./..., go vet ./..., go test ./... all green, gofmt -l . empty, role-drift test keeps passing.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/54: Verified at head c9946b8 (= PR #54 headRefOid), worktree clean, 3 files, +25/-7.

Ran the repo bar: go build ./... ok, go vet ./... ok, gofmt -l . empty, go test ./... all packages ok (includes internal/fleet init_test.go role-drift check against docs/examples.md, untouched bytes).

Criterion re-derived by reading docs/how-it-works.md:63-89: the section now opens with the selecting condition ('whether the path to the result is clear'), then '**Unclear → grill it into a spec.**' (grill → to-spec/to-tickets → .scratch tree → one card with Spec: line) and '**Clear → file the card itself.**' (own Done when / Verify by, no Spec: line, worked as one ticket with /tdd, nothing under .scratch). Both paths named with the condition that picks one — met. Wording matches the shipped dev role (internal/fleet/roles/dev.md:8-12: Spec: line → implement-spec, else one ticket with /tdd), so the doc does not promise behaviour the role lacks. README How it works items 1-2 mirrored, as the ticket allows (that section describes the intake); fragment changelog.d/TASK-82.md follows changelog.d/README.md shape.

Note (non-blocking, filed as docket/TASK-83): README's tagline/diagram (lines 3-18), 'A card is one spec' (line 99), README item 4 (line 59) and how-it-works step 5 (line 108, 'checks the spec's and every ticket's Verify by') still read as spec-only. The ticket scoped README to How it works, so this is a follow-up, not a FAIL.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/54 — verified PASS on patch 02caee2040c0bfdf93b9ef807e9dabcc196a36a5, CI green.
<!-- SECTION:NOTES:END -->
