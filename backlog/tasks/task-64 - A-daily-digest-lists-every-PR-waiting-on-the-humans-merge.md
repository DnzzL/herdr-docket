---
id: TASK-64
title: A daily digest lists every PR waiting on the human's merge
status: Done
assignee: []
created_date: '2026-10-04 15:52'
updated_date: '2026-10-04 17:41'
labels:
  - enhancement
dependencies:
  - TASK-63
priority: medium
ordinal: 64000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The merge-needed popup is transient and desktop-only: a human away from the terminal misses it, and on 2026-10-04 eleven DishNow PRs piled up unnoticed. Wanted: once a day, one place that lists every open PR labelled merge-needed across the fleet's queues' repos, with why each one waits and since when. Delivered as an automations.yaml entry plus its documented example (same shape as the stall sweep); one GitHub issue per repo titled 'Merges waiting', updated in place, closed when empty.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A scheduled entry runs once a day and reads every queue's repo for open PRs labelled merge-needed
- [ ] #2 Each repo with waiting PRs has exactly one open 'Merges waiting' issue, rewritten each run: PR link, task id, reason, waiting since
- [ ] #3 A repo with none has no open digest issue (the previous one is closed)
- [ ] #4 The example in docs states the entry and that the digest decides nothing — it only lists
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Delivered as PR https://github.com/DnzzL/herdr-docket/pull/25: a fourth factory schedule merge-digest (cron 30 7 * * *, repo ~/fleet, workspace root, model haiku) in internal/factory/install.go entries + its documented example in docs/factory.md (drift-pinned together), plus loop-table row, the carve-out under 'the schedule never does the work', the licensing paragraph ('the digest decides nothing — it only lists'), CHANGELOG, and a red-first test TestTheMergeDigestIsShippedDailyAndLicensedInDocs. Design: the entry does its own listing from the cron run (criterion #1 makes the entry the reader); facts come from sources of record only — task id from the queue file/history.jsonl, reason from the queue's 'is held for a human —' note, waiting since from the PR's merge-needed label event on the GitHub timeline; unknowns print 'not recorded'/'unknown', never guessed. Replayed on the real surface: enumeration of all 3 queues -> 2 GitHub repos (fleet skipped, no remote), label query [] on both, no digest issue open; joints verified on real held PR 55 (dishnow/TASK-120 + CODEOWNERS reason); full issue lifecycle (create -> rewrite in place -> supersede duplicate -> close when empty) proven on a throwaway probe repo. Live automations.yaml untouched — activation is a human paste (installer gap filed as a follow-up).

PR https://github.com/DnzzL/herdr-docket/pull/25 — CI green (build pass), local bar green (go build ./..., go vet ./..., go test ./..., gofmt -l . empty). Criterion verdicts, evidence my own: #1 MET — shipped entry merge-digest, cron "30 7 * * *" (daily), prompt reads every queue's repo for open PRs labelled merge-needed; replayed on the real config: 3 sources -> DishNow-v2 + herdr-docket resolved from git remotes (fleet skipped: no remote), gh pr list --label merge-needed -> [] on both. #2 MET — the entry keeps exactly one open 'Merges waiting' issue per repo, rewritten each run with PR link, task id, reason, waiting since; proven end-to-end on a throwaway probe repo (create -> gh issue edit rewrite in place, exactly one open, a late duplicate closed as superseded) and the three joins proven on real data: held DishNow PR 55 -> dishnow/TASK-120 + CODEOWNERS reason from the queue file, history.jsonl fallback agrees, waiting-since from the label event on the PR timeline. Caveat, plainly: no fleet PR carried the label during this run — the labeler is docket/TASK-63's unmerged PR — so the non-empty path was proven on the probe repo, not on DishNow. #3 MET — probe run with no labelled PR closed the open digest issue and left none open; both real repos verified to have no open 'Merges waiting' issue. #4 MET — docs/factory.md shows the entry verbatim (drift-pinned to install.go) and states 'the digest decides nothing — it only lists', pinned by the new test (red before the entry, green after). Known gap recorded in docs: init --factory will not append this entry to an already-installed automations.yaml — this fleet activates by pasting it from docs/factory.md; follow-up docket/TASK-67 filed. Residue: probe repo DnzzL/merge-digest-probe (private, PR and issues closed) needs manual delete — token lacks delete_repo scope. Never merged it myself.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/25: My own evidence, worktree at head b9d34b6 (base origin/main 570ded5). Bar green: go build ./..., go vet ./..., go test ./... exit 0, gofmt -l . empty. Scope: 4 files (internal/factory/install.go, docs/factory.md, internal/factory/drift_test.go, CHANGELOG.md), 2 commits, the ticket's own — no stray work. Red-first proven: worktree at test commit 4b6b1a4 -> TestTheMergeDigestIsShippedDailyAndLicensedInDocs fails 'init writes no merge-digest entry' (drift_test.go:41), green at head. #1 MET: entry install.go:95, cron '30 7 * * *' (daily, pinned by the test), prompt enumerates every fleet.yaml source dir -> git remote -> 'gh pr list --label merge-needed'; replayed on the real config: 3 sources -> DishNow-v2 + herdr-docket resolved from remotes, ~/fleet skipped (no remote), label query ran -> [] on both. #2 MET: prompt prescribes create / gh issue edit in place / close-duplicate-superseded, body one bullet per PR with link, task id, reason, waiting since; replayed live on throwaway probe repo: issue #4 created (exactly one open), edit kept number #4 with 2 bullets, late duplicate #5 closed 'superseded' -> one open. Joins on real data: DishNow PR 55 -> grep -rl -> task-120 file line 62 'is held for a human - CODEOWNERS path' -> TASK-120 + reason; history.jsonl carries pull/55 (fallback); the entry's exact timeline jq returned 2026-10-04T17:39:14Z on a real labeled event and empty -> 'unknown' on unlabeled PRs (read, never guessed). Caveat, plainly: no fleet PR carries merge-needed today (labeler is docket PR #24, still open), so the non-empty path was proven on the probe repo, not on DishNow. #3 MET: probe run with nothing waiting closed the digest issue with the 'nothing waiting' comment -> 0 open; both real repos have no open 'Merges waiting'. #4 MET: docs/factory.md:81 carries the entry verbatim (TestWhatWeShipIsWhatTheDocsShow pins install.go <-> docs), licensing sentence at docs/factory.md:140 'the digest decides nothing - it only lists', pinned red-first by the new test. Not blocking, recorded: live automations.yaml has no merge-digest entry -- activation is a human paste, known gap at docs/factory.md:354 with follow-up docket/TASK-67; probe repo DnzzL/merge-digest-probe needs manual delete (all its issues closed, token lacks delete_repo).

fleet: merged https://github.com/DnzzL/herdr-docket/pull/25 — verified PASS on patch 7170f27a5184386ca185f182cdcbac288f2c85cd, CI green.
<!-- SECTION:NOTES:END -->
