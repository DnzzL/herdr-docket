---
id: TASK-85
title: herdr contract test uses a fixed agent name and flakes on back-to-back runs
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-22 19:19'
updated_date: '2026-10-22 19:28'
labels: []
dependencies: []
ordinal: 85000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
TestARealBusyPaneRefusesAnAgentStartWithTheCodeTheFleetRetriesOn (internal/herdr/contract_test.go) starts a real herdr agent named docket-contract-first. Under consecutive go test ./... runs it failed once with agent_name_taken: a previous run's workspace close had not yet released the name. Not caused by TASK-84 (observed while re-running the suite), but it makes the suite red at random on hosts with a live herdr.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The contract test derives agent names from time.Now().UnixNano() (or the workspace id), so two runs cannot collide
- [ ] #2 go test ./internal/herdr -count=3 passes on a host with herdr installed
- [ ] #3 go build ./..., go vet ./..., go test ./... and gofmt -l . stay clean
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Fixed the flake in internal/herdr/contract_test.go: the two agent names now carry a base-36 time.Now().UnixNano() suffix (contract-first-<id>, contract-second-<id>), which keeps them under herdr's 32-char limit and distinct across runs. Root cause confirmed live: a first attempt using the old 22-char prefix plus a decimal nanosecond suffix was rejected as invalid_agent_name (over 32 chars); base-36 fits. Verified on the host's herdr 0.9.0: go test ./internal/herdr -count=3 -> ok 15.185s, then go test ./... all packages ok; go build ./..., go vet ./..., gofmt -l . clean. PR #57. No changelog fragment: test-only, not user-visible.

AC1 met: both agent names derive from time.Now().UnixNano() (base-36 suffix) in contract_test.go, so two runs cannot collide. AC2 met: go test ./internal/herdr -count=3 passed against live herdr 0.9.0 (ok, 15.185s). AC3 met: go build ./..., go vet ./..., go test ./... (all packages ok), gofmt -l . empty. Test-only change; no changelog fragment.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/57: Verified at head 7174ec4 (diff bca1594..HEAD: only internal/herdr/contract_test.go, +9/-2, one commit 'test(herdr): unique agent names per contract run' — matches the ticket, nothing extra).

AC1 met — contract_test.go:135-137: runID := strconv.FormatInt(time.Now().UnixNano(), 36); first := "contract-first-"+runID, second := "contract-second-"+runID. Names are unique per run by construction; worst case (math.MaxInt64 base36 = 13 chars) a name is 28 chars, under herdr's 32-char limit. No fixed agent name left anywhere in the repo (grep docket-contract/contract-first over *.go/*.md: only the new derived lines).

AC2 met — on a host with live herdr (test resolves /nix/store/...-herdr-0.9.0/bin/herdr via hostpath.Bin(), same 0.9.0 the author used): 'go test ./internal/herdr -count=3' -> ok 12.354s; and the targeted run '-count=3 -run TestARealBusyPaneRefusesAnAgentStartWithTheCodeTheFleetRetriesOn' -> 3x PASS, ok 15.382s, each run logging 'herdr contract held against .../herdr-0.9.0'.

AC3 met — go build ./... OK, go vet ./... OK, go test ./... all packages ok (internal/herdr 5.390s), gofmt -l . printed nothing.

Honest limit, not a finding: I could not make the old version red here. Replaying the pre-fix file (git show bca1594:internal/herdr/contract_test.go) through 'go test -overlay' against HEAD's tree passed -count=3 — overlay replacement confirmed live by a marker test that failed as expected — so the agent_name_taken window did not open on this host. AC2 stands on HEAD's green back-to-back runs, and the fix still removes the collision by construction rather than by timing.

Test-only change, so no changelog.d fragment is owed (CONTRIBUTING: user-visible changes only).

fleet: merged https://github.com/DnzzL/herdr-docket/pull/57 — verified PASS on patch ba52e88c99c1691c20358bda48166e790934a055, CI green.
<!-- SECTION:NOTES:END -->
