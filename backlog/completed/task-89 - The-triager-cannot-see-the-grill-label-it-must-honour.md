---
id: TASK-89
title: The triager cannot see the grill label it must honour
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-28 14:03'
updated_date: '2026-10-28 15:40'
labels: []
dependencies: []
ordinal: 98000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
TASK-86 ships a rule that a card carrying the `grill` label is always uncertain, but the triager's run never sees labels: internal/prompt/prompt.go head() renders id/title/body/criteria/notes only, and text.TaskDetail (task view) omits work.Task.Labels too. The shipped role and grill prompt therefore ask the agent to honour a fact it cannot read.

Fix: render labels in the prompt's task head (and task view) when present — e.g. a `## Labels` block — and add a prompt test that a triaged card with Labels [grill] carries the label. Non-blocking for TASK-86: its Verify-by only requires the triager role to name the label, which it does.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A triaged card's prompt contains its labels, including grill
- [ ] #2 task view shows labels when the card has them
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Criterion 1 is met on main (593307f): internal/prompt/prompt.go:199 prints Labels in the task head. Only #2 (task view shows labels) is left.

Diff: internal/text/task.go TaskDetail now prints a 'Labels: grill, mobile' line right after the state line when work.Task.Labels is non-empty; same wording/format as the prompt head so the two views cannot disagree. It is the shared renderer, so task view and the pane's detail both get it. Tests: internal/text/task_test.go asserts the line appears with labels and is absent when there are none; main_task_test.go TestTaskViewShowsBodyNotesAndCriteria now drives 'task view' on a card labelled grill and asserts 'Labels: grill'; docs/commands.md row updated; changelog.d/TASK-89.md added.

Verdicts. #1 A triaged card's prompt contains its labels, including grill: MET, and it was already on main (593307f) — internal/prompt/prompt.go:199 writes 'Labels: <joined>' in head(), covered by internal/prompt/prompt_test.go TestThePromptNamesTheCardsLabels (passes). No change was needed for this box. #2 task view shows labels when the card has them: MET by this PR — internal/text/task.go:29 plus the CLI-level test above.

CI in this worktree: go build ./..., go vet ./..., go test ./... all pass, gofmt -l . empty. GitHub Actions 'build' on PR #68 pass.

PR #68: task view names the card's labels. Criterion #1 (prompt carries labels incl. grill) verified already met on main: prompt.go:199 + TestThePromptNamesTheCardsLabels passes. Criterion #2 (task view shows labels when present): internal/text/task.go renders 'Labels: …' in TaskDetail when non-empty, asserted at renderer level and through 'task view' (main_task_test.go); no empty line for unlabelled cards. go build/vet/test green, gofmt clean, CI build pass. Changelog fragment TASK-89, docs/commands.md updated.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/68: #2 MET — live, not just the fake. Ran this worktree's binary at head a1921e1: `go run . task view docket/TASK-62` (a card whose backlog file carries `labels: [bug]`) prints `Labels: bug` right under the state line; unlabelled cards get nothing (`internal/text/task.go:29-31`, len guard). Renderer test `internal/text/task_test.go` asserts both the present and absent cases, and `main_task_test.go TestTaskViewShowsBodyNotesAndCriteria` drives `task view` on a grill card. Backend path is real, not fake-only: `backlogmd.Source.Get` → `asTask` copies `Labels` (backlogmd.go:203), and `backlog task view --json` indeed emits `labels` for the card — so the fakeSource test agrees with the world. #1 MET, already on main 593307f: `internal/prompt/prompt.go:199` writes `Labels: <joined>` in head(), `TestThePromptNamesTheCardsLabels` passes (Triage path, empty-label case too); no prompt change in this diff, as the task allows. Bar green in this worktree: go build ./..., go vet ./... clean, go test ./... all packages ok, gofmt -l . empty. Docs row (docs/commands.md) and changelog.d/TASK-89.md updated. Nothing out of scope: the diff is 3 lines of code plus tests/docs.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/68 — verified PASS on patch abc72c8440d2d28559394dac18408e9ed1b8ab81, CI green.
<!-- SECTION:NOTES:END -->
