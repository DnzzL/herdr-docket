---
id: TASK-75
title: >-
  docs and roles teach task verdict without --note — the one route to a bare
  verdict comment
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-13 17:23'
updated_date: '2026-10-13 18:35'
labels: []
dependencies: []
ordinal: 75000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Every command example outside prompt.go omits --note: internal/factory/roles/reviewer.md:36, docs/commands.md:20, docs/factory.md:177 and docs/examples.md:251 all show `task verdict <id> PASS|FAIL --pr <url>`, while prompt.go:125-126 (what a verify run actually executes) shows `--note "<what you checked and how>"`. An agent that follows the role/docs literally, plus FLEET.md's 'every conclusion belongs in a task note', produces exactly TASK-71's reported symptom: a bare docket-verdict comment on the PR with the evidence stranded in the task notes. The comment builder already appends the note when one is passed (main.go:641-644), so the remaining hole is the instruction, not the code. The fleet's own copy at /home/thomas/fleet/roles/reviewer.md:36 has the same omission — update it with the repo change.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 grep -rn 'task verdict' docs internal/factory/roles shows every command example carrying --note
- [ ] #2 the shipped reviewer role text states the verdict command with --note <evidence>
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Run for TASK-75: the docs and roles taught a bare 'task verdict … --pr' while the verify prompt runs it with --note, so agents following the docs left evidence in the task notes instead of on the PR comment. Fixed every example: internal/factory/roles/reviewer.md now states --note "<evidence>" (and why), docs/examples.md carries the same text verbatim (drift test pins the role inside it), docs/commands.md, docs/factory.md and the ADR 0013 mention all carry --note. Added internal/factory/drift_test.go TestEveryVerdictExampleCarriesItsNote so a bare example fails the build, plus changelog.d/TASK-75.md. The fleet's own copy /home/thomas/fleet/roles/reviewer.md was updated to match the shipped role (diffed identical). PR: https://github.com/DnzzL/herdr-docket/pull/47. AC#1 MET: 'grep -rn task verdict docs internal/factory/roles' returns 5 hits, every one carrying --note (docs/adr/0013:17, docs/commands.md:20, docs/examples.md:252, docs/factory.md:177, internal/factory/roles/reviewer.md:37). AC#2 MET: shipped roles/reviewer.md:36-37 states 'herdr-docket task verdict <id> PASS|FAIL --pr <url> --note "<evidence>"'. Verified: go build/vet/test all pass, gofmt -l empty; red-before-green shown — the new test's regexp run against HEAD's old reviewer.md catches the bare example, green on the new text; TestWhatWeShipIsWhatTheDocsShow passes so examples.md and the role stay verbatim-identical.

Both criteria met. AC#1: grep -rn 'task verdict' docs internal/factory/roles returns 5 hits, every one carrying --note (docs/adr/0013:17, docs/commands.md:20, docs/examples.md:252, docs/factory.md:177, internal/factory/roles/reviewer.md:37). AC#2: the shipped reviewer role (internal/factory/roles/reviewer.md:36-37) states 'herdr-docket task verdict <id> PASS|FAIL --pr <url> --note "<evidence>"' and explains the note is what lands on the PR comment. Verified myself: go build ./..., go vet ./..., go test ./... all pass, gofmt -l . empty; new drift test TestEveryVerdictExampleCarriesItsNote is red on HEAD's old reviewer.md (caught the bare example) and green on the new text; TestWhatWeShipIsWhatTheDocsShow passes so docs/examples.md and the role stay verbatim-identical. The fleet's own copy /home/thomas/fleet/roles/reviewer.md was updated to match the shipped role byte-for-byte (diff clean). Changelog fragment changelog.d/TASK-75.md written.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/47: AC#1 MET: grep -rn 'task verdict' docs internal/factory/roles returns 5 hits, every one carrying --note (docs/adr/0013:17, docs/commands.md:20, docs/examples.md:252, docs/factory.md:177, internal/factory/roles/reviewer.md:37) — no bare example left in scope. AC#2 MET: shipped internal/factory/roles/reviewer.md:37 states 'herdr-docket task verdict <id> PASS|FAIL --pr <url> --note "<evidence>"' plus the why (the note is what lands on the PR comment). I ran the repo's bar myself: go build ./..., go vet ./..., go test ./... (21 pkgs) all pass, gofmt -l . empty. Red-before-green re-derived, not taken on trust: the new TestEveryVerdictExampleCarriesItsNote regexp run against HEAD~1's five files flags 5 bare examples (commands.md 'task verdict ID PASS|FAIL --pr URL', examples.md, factory.md, ADR 0013, reviewer.md) — red before, green after. TestWhatWeShipIsWhatTheDocsShow passes so docs/examples.md and the role stay verbatim-identical. Scope is the ticket's own: single commit, only docs/roles/drift test/changelog fragment — no daemon, runner, gate or CLI code touched, so the execute risk area is unchanged; the drift test reads real files from disk, not an agreeing fake. Fleet copy /home/thomas/fleet/roles/reviewer.md is byte-identical to the shipped role and carries --note at line 37.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/47 — verified PASS on patch a01b7a9e9838d951369c6b88eed4adaa65cb48cb, CI green.
<!-- SECTION:NOTES:END -->
