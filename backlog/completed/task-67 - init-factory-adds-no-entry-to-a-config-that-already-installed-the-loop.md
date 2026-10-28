---
id: TASK-67
title: init --factory adds no entry to a config that already installed the loop
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-04 17:33'
updated_date: '2026-10-28 15:25'
labels: []
dependencies: []
ordinal: 67000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Install writes the loop's entries only when the config has no '- name: intake' yet, so a fleet that installed before an entry existed — today merge-digest (docket/TASK-64) — never receives it from a re-run, and every future entry ships the same way: shipped bytes nobody's config gets. Appending only the entries missing by name (check each '- name: X') keeps the idempotency test's contract and lands the new entry for existing fleets. Then delete the 'fragile' known-gap line docs/factory.md gained for this.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Picked up. Reading CONTRIBUTING and the installer: install.go gates the whole append on strings.Contains(raw, "- name: intake"), so a config with the loop minus merge-digest gets nothing from a re-run. Writing the failing test first (append-only-missing-by-name, idempotency kept), then the fix, then deleting the fragile known-gap line in docs/factory.md.

Fix done, PR open: https://github.com/DnzzL/herdr-docket/pull/28 (CI green). Red test committed first (ee907f5), fix in 6e87cb2. install.go now splits the shipped block per entry and appends only the '- name: X' lines the config lacks; a complete loop takes the old 'already drives the loop' branch and is never written. Deleted the fragile known-gap line in docs/factory.md, added a CHANGELOG entry and corrected the merge-digest bullet's stale 'pastes it in by hand' sentence. Verified on the real surface: built binary in /tmp, fresh init --factory, stripped merge-digest to simulate a pre-TASK-64 fleet, re-run printed 'appended merge-digest' with 4 unique entries parsing under yaml.v3, third run printed 'already drives the loop' with unchanged md5. go build/vet/test ./... and gofmt -l . all clean.

Red test first (commit ee907f5 fails on old code: a config with the loop minus merge-digest gets nothing from a re-run), fix in 6e87cb2: install.go splits the shipped block per entry and appends only the entries the config does not name ('- name: X' checked line-exact); a config naming them all is not written, so the idempotency contract holds. Also deleted the fragile known-gap line in docs/factory.md, added a CHANGELOG entry, corrected the merge-digest bullet's stale hand-paste sentence. Met: re-run lands merge-digest, idempotency kept, gap line gone. Verified myself: go build ./... , go vet ./... , go test ./... , gofmt -l . all clean; real surface with the binary built into /tmp — fresh install, merge-digest stripped to simulate a pre-TASK-64 fleet, re-run printed 'factory: appended merge-digest', config parses with yaml.v3 as 4 unique entries pointing at the fleet dir, third run printed 'already drives the loop' with unchanged md5. CI green on the PR.

fleet: run failed: waiting for the agent: agent wait: timeout: timed out waiting for agent status

fleet: the run's workspace w1 (pane w1:pV) is left open — jump in to resume.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/28: Verified PR head 6e87cb2 myself. Bar on head: go build ./..., go vet ./..., go test ./..., gofmt -l . all clean (same four as ci.yml). Red-first proven: at ee907f5 (parent 9299099, old installer) go test ./internal/factory/ -run TestInstallAppendsOnlyTheEntriesAConfigIsMissing FAILs at install_test.go:204 — a config with the loop minus merge-digest gets nothing from a re-run; green at 6e87cb2. Real surface (real herdr 0.9.1, HOME and XDG_CONFIG_HOME sandboxed in /tmp, owner config untouched): fresh init --factory created automations.yaml with 4 unique entries parsing under yaml.v3, repos = fleet dir; stripped merge-digest to simulate a pre-TASK-64 fleet and re-ran: "factory: appended merge-digest", diff shows only the merge-digest chunk inserted, 4 unique; third run: "already drives the loop", md5 unchanged (f632aad3... = f632aad3...). Old binary replayed the same scenario: "already drives the loop", still 3 entries — bug live before, fixed after. Probes: middle entry stripped -> "appended lookback", 4 unique; config holding only "- name: intake-followup" -> all 4 shipped entries appended, owner entry kept (substring trap handled at install.go:177 hasEntry). Criteria: append-missing-by-name at install.go:149 entryName, :177 hasEntry, :243-249 loop; idempotency test untouched (diff +49/-0) and green; fragile known-gap line (base docs/factory.md:357) deleted, other fragile lines (347/349/352) kept. Scope: exactly 2 commits, 4 files, all the ticket own; no CODEOWNERS path touched, so no human hold. Non-blocking observation for whoever merges: the PR edits CHANGELOG.md directly, which CONTRIBUTING post-#26/ADR 0016 forbids (changelog.d/TASK-67.md is the current form); its base 9299099 predates that rule and then required the CHANGELOG.md entry, no other open PR touches CHANGELOG.md (PR 27 is daemon_test only), merge is CLEAN, and changelog.go folds the legacy Unreleased section at release — lands correctly, not a block.

fleet: https://github.com/DnzzL/herdr-docket/pull/28 is merged — the pull request settles this task done.
<!-- SECTION:NOTES:END -->
