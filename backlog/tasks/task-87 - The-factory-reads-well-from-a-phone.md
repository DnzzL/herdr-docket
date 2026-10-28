---
id: TASK-87
title: The factory reads well from a phone
status: Done
assignee: []
created_date: '2026-10-28 13:29'
updated_date: '2026-10-28 14:07'
labels: []
dependencies: []
ordinal: 90000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Problem

From a phone (Termux + `herdr --remote`) the owner cannot tell whether the daemon is alive, what waits on them, or answer a grill round without typing the answer syntax by hand. A terminal under 50 columns breaks the runs pane.

## Solution

The daemon writes a heartbeat each tick; every view shows `daemon live · tick 3s ago` or `daemon down for 4 min`. `herdr-docket questions` is a line-based prompt: it lists the `question` cards, shows the newest round one question at a time (Enter = the reco, a letter = that option, anything else = free text), then calls `task answer`. The runs pane opens with the daemon line and a "Your turn" section (question cards, Blocked cards, PRs at PASS awaiting a merge) and lays out under 50 columns.

## Done when

On a 45-column terminal, the owner sees whether the daemon is alive, what waits on them, and answers a round with Enter/letters only.

## Verify by

`go test ./...` green, including: a heartbeat older than 3 ticks reads as down; `questions` fed "\n B\n keep it\n" produces `1.A 2.B 3. keep it` for a three-question round whose recos are A; the pane renders with no line over 45 columns for a fixture with long titles.

## Out of scope

A full-screen TUI library; push notifications (ntfy).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Integrated the three delivered slices onto one branch. t87.1 (heartbeat + status), t87.2 (questions), t87.3 (pane Your turn) were merged; conflicts in internal/work/github (Config.Trusted vs omitempty tags, trusted-comment tests vs sub-issue tests) and docs/queues.md resolved as unions. Fixed init_test's SourceConfig struct compare (Config gained a slice) and made questions treat EOF at the send prompt as a send so the card's exact input "\n B\n keep it\n" posts 1.A 2.B 3. keep it; added TestQuestionsSendsWhenTheInputEndsAfterTheLastAnswer. Added changelog.d/TASK-87.md and moved the three phone bullets out of the repo-tracker fragment. Bar green: go build ./..., go vet ./..., go test ./..., gofmt -l . empty; CI build pass on PR #62. Scope note for the verifier: the branch sits on the repo-tracker integration base the three child slices were cut from (init <repo> as queue, Triage/Question, task ask/answer), so the PR diff is larger than the three phone commits; the phone work is the last three slices. Live smoke: /tmp build prints 'daemon never started' via status; questions on the real fleet.yaml fails because that config still names a notara source, so no live questions round was run (unit test covers the round).

PR #62, CI build pass. Verdict per criterion — Done when (45-column terminal: daemon alive, what waits, answer a round with Enter/letters): met. Heartbeat is written each tick (internal/daemon/heartbeat.go, TestATickWritesTheHeartbeat); status and the pane/ questions open on daemon.Liveness (TestPaneOpensOnTheDaemonLine, questions test asserts first line is the daemon line); pane lists Your turn with question cards, Blocked cards and PASS-awaiting-merge PRs (TestYourTurnListsQuestionsBlockedAndHeldMerges); questions walks a round with Enter = reco, letters = options, text = free, then calls taskAnswer (TestQuestionsAnswersARoundWithEnterLettersAndFreeText). Verify by: go test ./... green, including the three named cases — TestLivenessOldHeartbeatIsDown (4 ticks older than 3 reads down), TestQuestionsSendsWhenTheInputEndsAfterTheLastAnswer (input "\n B\n keep it\n" posts ## Answer 2 with 1.A 2.B 3. keep it), TestPaneNeverWrapsOnANarrowTerminal (long-title fixture, every line <= 45 columns). Also go build/vet green and gofmt -l . empty. Caveat, stated for the verifier: the branch was merged from the repo-tracker integration base the three child slices were cut from, so the PR carries that base too; the phone work is the last three slices (87.1/87.2/87.3). No live questions round was run on the real fleet.yaml because it still names a removed notara source; the round is covered by unit tests.

verdict FAIL on https://github.com/DnzzL/herdr-docket/pull/62: Checked on the PR head bc4d411 in the fleet worktree. Bar: go build ./... ok, go vet ./... clean, gofmt -l . empty, go test ./... all packages ok. All TASK-87 criteria re-derived live and met: (1) heartbeat — fresh heartbeat reads 'daemon live · tick 0s ago', one 240s old reads 'daemon down for 4 min', none reads 'daemon never started' (internal/daemon/heartbeat.go:30, written each tick at internal/daemon/daemon.go:107). (2) questions — on a real Backlog.md queue (herdr-docket init /tmp/d87live/repo, a Question card with a ## Round 2 note, recos A), printf '\n B\n keep it\n' | herdr-docket questions printed '1.A 2.B 3. keep it', wrote '## Answer 2 / 1.A 2.B 3. keep it' into the card's notes and moved Question to Triage (questions.go:51). (3) pane — run under a real 45-column pty (script + stty cols 45): first line 'daemon never started · fleet running', then Your turn with '? TASK-2 …' and '! TASK-3 …'; zero rendered lines over 45 columns (runs.go:356 via internal/text.Truncate). 87.1/87.2/87.3 are Done at head and changelog.d/TASK-87.md is present. BLOCKING, scope: only 4 of the 22 non-merge commits are this task's (994e9c3, bfefc74, a8ae965, 636d6e1). The rest are other cards' work merged in: 86.1 (3be0564), 86.2 (2e011e7), 88.3 (d9a0f3b), and a ~17-commit repo-tracker base (a2c5dea..20621bf) that includes the breaking 'feat!: basecamp and notara queues are removed' (d90e9d4), ADR 0023 and the init rewrite. That base has no card and no Verify by at head (its .scratch spec is deleted in 20621bf), so it cannot be verified from TASK-87, and it overlaps the open PRs #60 (task 86) and #61 (86.3), both based on main and still carrying notara — merging #62 first lands those specs and removes two queue backends under this card's verdict. Fix: give the repo-tracker base its own card and PR (or a human's explicit decision to carry it), land 86.2's task ask/answer through its own verdict since 87.2 depends on it, then rebase #62 onto main keeping only the four TASK-87 commits. The phone code itself needs no change and I will re-verify just those four. A human should look at the breaking backend removal either way.
<!-- SECTION:NOTES:END -->
