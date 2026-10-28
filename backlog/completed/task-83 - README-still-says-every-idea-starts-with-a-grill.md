---
id: TASK-83
title: README still says every idea starts with a grill
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-22 17:36'
updated_date: '2026-10-22 17:41'
labels: []
dependencies: []
ordinal: 83000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
TASK-82 taught docs/how-it-works.md and README's How it works that a clear task skips the grill, but four lines still describe the spec path as the only one: the tagline and ASCII diagram (README lines 3-18: 'Grill an idea into a spec', no clear-path branch), 'Not a workflow engine. A card is one spec' (README line 99), README step 4 'The author works the spec's tickets' (line 59), and docs/how-it-works.md anatomy step 5 'checks the spec's and every ticket's Verify by' (line 108) — a card with no Spec: line has neither. Fix: make each read true for both paths in one sentence, no history.

Done when: no line in README.md or docs/how-it-works.md claims a card necessarily points at a spec, or that verification always reads a spec's Verify by.
Verify by: read every 'spec' hit in the two files; go test ./... green.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 No line in README.md or docs/how-it-works.md implies every card has a Spec: line.
- [ ] #2 go test ./... green.
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
AC #1 met: reworded README tagline, diagram, steps 2/4/5 and 'Not a workflow engine', plus docs/how-it-works.md's model and anatomy, so every spec reference is conditional on the unclear path or scoped to a card that carries one. Re-read every 'spec' hit in both files: no line claims a card necessarily has a Spec: line, or that verification always reads a spec's Verify by. AC #2 met: go build ./..., go vet ./..., go test ./... and gofmt -l . all green locally; PR CI run 39824525937 green. PR #55 was rebased onto main after TASK-82 (#54) merged and is mergeable. Added changelog.d/TASK-83.md.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/55: AC #1 met: re-read every 'spec' hit in both files at head 8934180 — README:3 (tagline: grill only 'when the path is unclear'), README:16-18 (diagram now has both idea (unclear) and idea (clear) branches, card labelled 'Spec: line, or its own Done when / Verify by'), README:53 ('unclear idea'), README:56-58 (card 'either points at a spec ... or — when the task is clear — carries its own'), README:61/65 (author and verifier both cover 'whatever the card carries'), README:103 ('A card is one unit of work, spec or not'), docs/how-it-works.md:24-29 ('usually a pointer' + 'A card with no Spec: line is a single ticket'), 66-88 (two explicit paths), 108-110 (verifier checks 'every Verify by the card carries ... or the clear card's own'). grep -rn 'card is a spec' README.md docs/how-it-works.md finds nothing. AC #2 met: ran the repo bar myself in this worktree — go build ./... OK, go vet ./... OK, gofmt -l . empty, go test ./... ok in all 21 packages. Diff vs origin/main is docs only (README.md, docs/how-it-works.md, changelog.d/TASK-83.md) — no daemon/runner/gate/CLI code, so no risk-area test needed. Not blocking: README:80 autonomy table still says 'grill the spec' as the human's step-1 action; pre-existing from before TASK-82, does not claim a card has a Spec: line.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/55 — verified PASS on patch 99c3c4e8384b2567ce6bd870aeecd6c73d262206, CI green.
<!-- SECTION:NOTES:END -->
