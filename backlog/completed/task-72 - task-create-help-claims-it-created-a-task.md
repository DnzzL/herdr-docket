---
id: TASK-72
title: task create --help claims it created a task
status: Done
assignee: []
created_date: '2026-10-13 12:06'
updated_date: '2026-10-28 15:31'
labels: []
dependencies: []
ordinal: 72000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
herdr-docket task create --help prints 'created docket/ (in To Do)': the flag is taken as the title, backlog prints its own help, and the empty id is reported as a creation.
Done when: --help/-h prints the usage and creates nothing; an empty id from the queue is an error, never 'created'.
Verify by: a test that task create --help exits 0 with the usage and calls no Create; a test that an empty id fails.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 --help and -h print the usage and create nothing
- [ ] #2 An empty id from the queue is an error, not a creation
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Checked the card against the branch: both requirements are already implemented on main by c29498f 'fix(cli): task create --help prints usage; an empty id is an error (#44)', which includes changelog.d/TASK-72.md. main.go:478 handles -h/--help by printing usage and returning before any queue call; main.go:517 rejects an empty id before writing criteria or reporting 'created'. Tests TestTaskCreateHelpPrintsUsageAndCreatesNothing (both flags, no Create) and TestTaskCreateEmptyIDIsAnError exist and pass. Full CI green: go build ./..., go vet ./..., go test ./..., gofmt -l . empty. No new code needed — this card duplicates the landed fix, so no PR; closing done with the evidence.

Verdict: already fixed on main, no new code or PR needed. AC#1 met: main.go handles -h/--help by printing 'usage: herdr-docket task create ...' and returning before reaching any queue; TestTaskCreateHelpPrintsUsageAndCreatesNothing exercises both flags, asserts exit 0, asserts the usage text, and asserts no Create. AC#2 met: main.go rejects an empty id from create with an error before putCriteria or any 'created' output (also multi.CreateIn no longer reports empty ids); TestTaskCreateEmptyIDIsAnError asserts an error and that stdout contains no 'created'. Both tests pass; full CI green (go build, go vet, go test ./..., gofmt -l . empty). Evidence: commit c29498f (PR #44) carries the fix and changelog.d/TASK-72.md; this card duplicates it.
<!-- SECTION:NOTES:END -->
