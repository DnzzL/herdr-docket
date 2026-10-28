---
id: TASK-56
title: history shows a verifier run's verdict
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-03 17:24'
updated_date: '2026-10-03 17:46'
labels: []
dependencies: []
ordinal: 56000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Since ADR 0013 a verifier run stamps its verdict (PASS or FAIL) and the patch-id it judged on its history record (history.Record.Verification, PatchID). herdr-docket history does not show it, so a human supervising the pipeline cannot tell a PASS from a FAIL without reading history.jsonl.

Done when: a history line for a run whose record carries a Verification shows it, e.g. 'verified PASS p=<first 8 chars of patch-id>', and a run without one prints exactly what it prints today.

Verify by: a test next to TestHistoryLineShowsTheDelivery in main_test.go that is red before the change and green after; then run ./bin-built herdr-docket history against a state dir holding one record with Verification set and paste the line in the PR.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Test written first (TestHistoryLineShowsTheVerification, red before, green after), then formatHistory shows 'verified PASS  p=<8 chars>' only when a record carries a Verification; CHANGELOG entry. go test ./... (20 pkgs), go vet, gofmt clean; replayed on ./bin-built herdr-docket history against a seeded state dir — line pasted in PR #16. PR: https://github.com/DnzzL/herdr-docket/pull/16 CI green.

Verified myself: TestHistoryLineShowsTheVerification commits red then green (runs without a Verification print exactly the old line); go test ./... 20 pkgs ok, go vet + gofmt clean; replayed on the real binary: seeded a state dir with two records via history.SetVerification, 'verified PASS  p=9ad5c1e2' shows on the verified run only — line pasted in PR #16. CI green (build check pass). Criteria met; task verdict criterion replayed as stated.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/16: Test next to TestHistoryLineShowsTheDelivery (main_test.go:138) is red on pr-16~1 (4101520f: 'missing "verified PASS"') and green at head. Repo bar on the branch, throwaway worktree: go build, go vet, gofmt -l all clean; go test ./... 20 pkgs ok. Diff is the ticket's own work only: 4101520f touches main_test.go; 6c7f41f touches main.go (+9, gated block at main.go:740-747, shown only when r.Verification != "") and CHANGELOG. Replay on the real binary: built ./bin-built at pr-16 head, seeded a fresh state dir with two records via internal/history.Append — one with Verification=PASS + PatchID 9ad5c1e2..., one without. 'herdr-docket history' shows 'verified PASS  p=9ad5c1e2' on the verified run only; the unverified run's line is identical to what today's binary (main, 848ec37) prints for the same record — no new tokens. PASS criterion met as stated in the ticket.

fleet: https://github.com/DnzzL/herdr-docket/pull/16 is held for a human — this queue does not merge on its own (merge: never).

fleet: PR #16 merged by hand after merge: auto was enabled — verified PASS on patch 21ec56c8, CI green.
<!-- SECTION:NOTES:END -->
