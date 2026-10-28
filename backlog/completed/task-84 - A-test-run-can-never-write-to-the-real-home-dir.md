---
id: TASK-84
title: A test run can never write to the real home dir
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-22 19:12'
updated_date: '2026-10-22 19:32'
labels: []
dependencies: []
ordinal: 84000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
On 2026-10-22 a test in the root package ran `init --factory` against the owner's real config and inserted a merge-digest entry into ~/.config/herdr/plugins/config/dnzzl.automations/automations.yaml. No test in package main isolates the environment.

Add a TestMain to package main's tests that, before m.Run(), points HOME, XDG_CONFIG_HOME and HERDR_PLUGIN_STATE_DIR at a fresh temp dir, and removes it after. Grep the internal packages for code that reaches the real home or config dir under test; add the same guard to any package whose tests do.

Done when: no test in the repo can read or write the owner's real home, config or plugin state dir.

Verify by: go test ./... green; with HOME pointed at a read-only empty dir (chmod 500), go test ./... still passes, so no test depends on writing the real HOME; grep shows a TestMain setting the three variables in every package whose code under test resolves the home or config dir.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Slice done, gates green. Added internal/testenv (Main points HOME, XDG_CONFIG_HOME, HERDR_PLUGIN_CONFIG_DIR and HERDR_PLUGIN_STATE_DIR at one fresh temp dir, removes it after m.Run) and a testmain_test.go in every package whose code can resolve the config/state/skill dir: root, credentials, daemon, fleet, herdr, history, hostpath, runner, skill, work/{github,basecamp,notara}. HERDR_PLUGIN_CONFIG_DIR is set too because it beats HOME inside hostpath.ConfigDir — without it a test on a machine with herdr installed still reads the owner's real fleet.yaml. Regression test TestATestRunNeverResolvesTheRealHome (main_test.go) checks HOME is a temp dir, every resolved dir sits under it, and the config dir is actually writable there — so it fails if the guard is ever dropped.

Met every criterion, evidence in PR #56. (1) go build ./..., go vet ./..., gofmt -l . empty, go test ./... green — 21 packages ok, two consecutive -count=1 runs. (2) With HOME at a read-only empty dir (chmod 500; GOCACHE/GOMODCACHE/GOPATH redirected so Go itself could run), go test ./... exits 0 and that dir is still empty afterwards. (3) grep 'func TestMain' shows one in every package importing hostpath or credentials — root, credentials, daemon, fleet, herdr, history, hostpath, runner, skill, work/{github,basecamp,notara} — all calling internal/testenv.Main, which sets HOME, XDG_CONFIG_HOME, HERDR_PLUGIN_CONFIG_DIR and HERDR_PLUGIN_STATE_DIR to one fresh temp dir and removes it after m.Run; HERDR_PLUGIN_CONFIG_DIR is the addition beyond the ticket's three because it beats HOME inside hostpath.ConfigDir. Regression test TestATestRunNeverResolvesTheRealHome goes red if the guard is dropped. No changelog fragment: nothing user-visible. CONTRIBUTING's 'Working on it' now states the invariant for new packages. Follow-up filed as docket/TASK-85 (live herdr contract test uses a fixed agent name, flaked once on back-to-back runs).

verdict FAIL on https://github.com/DnzzL/herdr-docket/pull/56: Guard itself verified: internal/testenv/testenv.go:31-45 sets HOME, XDG_CONFIG_HOME, HERDR_PLUGIN_CONFIG_DIR and HERDR_PLUGIN_STATE_DIR to one MkdirTemp dir and removes it after m.Run; 13 TestMains (root + credentials, daemon, fleet, herdr, history, hostpath, runner, skill, work/{github,basecamp,notara}) cover every package importing hostpath or credentials. prompt/pick import fleet for value types only (prompt.go:28; pick.go:37,96), so no guard needed. go build ./..., go vet ./..., gofmt -l . empty, go test ./... green twice (-count=1). With HOME=<chmod 500 empty dir> and GOCACHE/GOMODCACHE/GOPATH redirected, go test ./... exits 0 and that HOME is still empty afterwards. Regression test is real: removing root testmain_test.go in a throwaway copy makes main_test.go:327 fail ("HOME = /home/thomas, want the temp dir testenv made"); real automations.yaml untouched (md5 ab8bc2a50cbf64f84b3a9bcaad504b09, mtime 16:25, runs at 21:2x). BLOCKER: this diff makes the repo's only real-queue check of the runner unrunnable and leaves its comment lying. internal/runner/settle_live_test.go:6-9 still documents === RUN   TestLiveSettlePass
    settle_live_test.go:37: list: backlog task list: Invalid directory from BACKLOG_CWD: /tmp/herdr-docket-test2725615753/fleet
--- FAIL: TestLiveSettlePass (0.52s)
FAIL
FAIL	github.com/DnzzL/herdr-docket/internal/runner	0.531s
FAIL, but :15 calls fleet.LoadSettings(), which now resolves hostpath.ConfigDir() to the temp dir because testenv overwrites HERDR_PLUGIN_CONFIG_DIR unconditionally (testenv.go:37). Verified in a throwaway copy with a probe test in package runner: ConfigDir=/tmp/herdr-docket-test.../plugins/config, LoadSettings dir=/tmp/herdr-docket-test.../fleet sources=0, then List err="backlog task list: Invalid directory from BACKLOG_CWD: /tmp/herdr-docket-test.../fleet". So a human following the documented command gets an obscure BACKLOG_CWD error, not a skip, and the settle pass can never be verified against the real queue again. Fix: give testenv an explicit opt-out (e.g. skip the overrides when HERDR_DOCKET_TESTENV=real is set), so the default still keeps every test off the real home, and update settle_live_test.go:6-9 to name the opt-in command. ~4 lines. Non-blocking: unguarded host/gate/backlogmd tests exec git/gh, which read the owner's real ~/.gitconfig and gh config; outside this ticket's plugin-config scope but worth one line in CONTRIBUTING.

Verifier's blocker fixed on the PR branch: testenv gains an explicit opt-out — HERDR_DOCKET_TESTENV=real skips all four overrides, so the live probes see the real fleet.yaml again. settle_live_test.go's doc now names 'HERDR_DOCKET_TESTENV=real LIVE_SETTLE=1 go test ...', and CONTRIBUTING states the opt-in plus one line on exec-git/gh tests reading ~/.gitconfig. Probed both ways with a throwaway test in package runner: default gives HOME=/tmp/herdr-docket-test… and the BACKLOG_CWD error the verifier saw; with the opt-in HOME=/home/thomas, real config dir, src.List() returned 416 tasks read-only. Gates rerun next.

Verifier blocker fixed (commit 58ad9d4). internal/testenv gains HERDR_DOCKET_TESTENV=real: when set, Main skips all four overrides, so a live probe sees the owner's real fleet.yaml and queue; the default keeps every test off the real home. settle_live_test.go's doc now names 'HERDR_DOCKET_TESTENV=real LIVE_SETTLE=1 go test ./internal/runner/ -run TestLiveSettlePass -v' and says why; CONTRIBUTING states the opt-in and one line that exec-git/gh tests still read the real ~/.gitconfig (verifier's non-blocking point). Evidence: throwaway probe in package runner showed default HOME=/tmp/herdr-docket-test… + BACKLOG_CWD /tmp/…/fleet error, opt-in HOME=/home/thomas + src.List() 416 tasks read-only; probe deleted. Criteria: (1) go build ./..., go vet ./..., gofmt -l . empty, go test ./... green (21 packages). (2) HOME at a chmod 500 empty dir with GOCACHE/GOMODCACHE/GOPATH redirected: go test ./... exits 0, dir still empty. (3) grep 'func TestMain' shows one in every package importing hostpath or credentials — all calling testenv.Main. TestATestRunNeverResolvesTheRealHome still goes red if the guard is dropped. CI build job green. No changelog fragment: nothing user-visible.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/56: Verified head 58ad9d4. Guard: internal/testenv/testenv.go:31-60 sets HOME, XDG_CONFIG_HOME, HERDR_PLUGIN_CONFIG_DIR, HERDR_PLUGIN_STATE_DIR to one MkdirTemp dir and removes it after m.Run; 13 TestMains call it (root + credentials, daemon, fleet, herdr, history, hostpath, runner, skill, work/{github,basecamp,notara}) — every package importing hostpath or credentials; prompt/pick import fleet for value types only. Bar: go build ./..., go vet ./..., gofmt -l . empty, go test ./... green (21 pkgs). Read-only HOME: HOME=<chmod 500 empty dir>, GOCACHE/GOMODCACHE/GOPATH redirected → go test ./... exit 0, HOME still empty. Blocker fixed and re-probed with a throwaway test in package runner (deleted): default HOME=/tmp/herdr-docket-test… ConfigDir=…/plugins/config fleetDir=…/fleet sources=0; HERDR_DOCKET_TESTENV=real → HOME=/home/thomas, ConfigDir=~/.config/herdr/plugins/config/dnzzl.herdr-docket, fleetDir=~/fleet sources=4. settle_live_test.go:6-12 now names the opt-in command. Regression test real: hiding testmain_test.go makes main_test.go:327 fail (HOME=/home/thomas); default passes; opt-out makes it fail as expected. Owner's automations.yaml untouched (md5 ab8bc2a50cbf64f84b3a9bcaad504b09, mtime 16:25). No unrelated work in the commits. Non-blocking, documented in CONTRIBUTING:39-44: host/gate/backlogmd tests exec git and read the real ~/.gitconfig; outside this ticket's home/config/state-dir scope, read-only, follow-up to plugin-dev.

fleet: https://github.com/DnzzL/herdr-docket/pull/56 is held for a human — internal/daemon/testmain_test.go is a CODEOWNERS path (/internal/daemon/).

fleet: https://github.com/DnzzL/herdr-docket/pull/56 is merged — the pull request settles this task done.
<!-- SECTION:NOTES:END -->
