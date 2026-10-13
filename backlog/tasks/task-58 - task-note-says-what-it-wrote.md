---
id: TASK-58
title: task note says what it wrote
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-03 20:09'
updated_date: '2026-10-03 21:13'
labels: []
dependencies: []
priority: high
ordinal: 58000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
herdr-docket task note prints nothing on success, unlike done/fail/block/assign which print '<id> <verb>'. On 2026-10-03 a plugin-dev run (docket/TASK-57) read its own silent, successful note writes as failures — every note had landed — concluded 'the backend filters apostrophes', and spent the rest of its turn chasing that until it settled with nothing delivered. A write the agent cannot see succeed is one it will retry, rephrase and doubt.

Done when: task note prints '<id> noted' on success, exactly as the closing verbs confirm theirs, and still prints nothing extra on failure (the error is the output).

Verify by: a test in main_task_test.go that is red before and green after (the note verb's output contains '<id> noted'); then run the built binary's task note against a throwaway task and paste the output in the PR.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Red first: TestTaskNoteSaysItWrote (main_task_test.go) fails at f50aa79 with 'note must confirm the write, got ""'. Fix at 606faa3: the note verb prints '<id> noted' after Comment returns nil, returns the error untouched otherwise (stdout stays empty). CHANGELOG entry added; built binary against a throwaway Backlog.md queue prints 'TASK-1 noted' (exit 0) and nothing on stdout for a refused note (exit 1). PR: https://github.com/DnzzL/herdr-docket/pull/20

Done when met: task note prints '<id> noted' on success, nothing extra on failure. Test in main_task_test.go (TestTaskNoteSaysItWrote) red at f50aa79 before the fix, green at 606faa3 after; it also asserts a refused note leaves stdout empty. CI green (build pass). Replayed on the real surface: built binary against a throwaway Backlog.md queue printed 'TASK-1 noted' exit 0, and for TASK-99 stdout was empty with the error on stderr exit 1 — output pasted in the PR. Full bar clean in my worktree: go build, go vet, go test ./..., gofmt -l empty.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/20: Re-derived every criterion on a throwaway worktree of pr-20-verify (head 606faa3, merge-base 143b24c = main). (1) Red first: git checkout f50aa79; go test -run TestTaskNoteSaysItWrote . → FAIL main_task_test.go:347 'note must confirm the write, got ""'. Green at 606faa3: same command PASS. (2) Full bar at head: go build ./..., go vet ./..., go test ./... all ok, gofmt -l . empty; CI build job pass. (3) Live surface: built binary, HERDR_PLUGIN_CONFIG_DIR=/tmp/t58-cfg over throwaway Backlog.md queue /tmp/t58-q (backlog 1.53.0): 'task note TASK-1 …' → stdout 'TASK-1 noted', stderr empty, exit 0, and the note landed in the task file's Notes section; 'task note TASK-99 …' → stdout empty, error on stderr, exit 1. (4) Format parity: main.go:306 '%s noted' matches the closing verbs' main.go:501 '%s %s'. (5) Scope: diff touches only the note case in runTaskCmd (main.go:300-307), one new test, one CHANGELOG entry — no smuggled work, commits are the ticket's own (f50aa79 test, 606faa3 fix). The one fact it's safe because of: the print sits after src.Comment returns nil (main.go:301-306), so the error path is byte-identical to before — asserted by the test's second half and replayed live via TASK-99.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/20 — verified PASS on patch 3b2bf53f2c352e508fbf9e70cf9a1345371104f8, CI green.
<!-- SECTION:NOTES:END -->
