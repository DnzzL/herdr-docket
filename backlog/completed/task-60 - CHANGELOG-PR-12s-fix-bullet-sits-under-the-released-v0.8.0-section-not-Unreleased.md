---
id: TASK-60
title: >-
  CHANGELOG: PR 12's fix bullet sits under the released v0.8.0 section, not
  Unreleased
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-03 22:48'
updated_date: '2026-10-03 23:09'
labels: []
dependencies: []
ordinal: 60000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Non-blocking finding from docket/TASK-48's verification (PR 12). CHANGELOG.md:47 the 'A run whose workspace was closed mid-flight reports cancelled again' bullet was added under '## v0.8.0 — 2026-10-02', but that section is a released, tagged version (tag v0.8.0, release commit 196657d 2026-10-02; release convention renames '## Unreleased' into the version section, see that commit). The fix ships in a future release, so the bullet belongs under '## Unreleased' (CHANGELOG.md:5). Fix: move the bullet; no code change.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Moved PR 12's 'A run whose workspace was closed mid-flight reports cancelled again' bullet out of the released '## v0.8.0 — 2026-10-02' section (tag v0.8.0, release commit 196657d) to the top of '## Unreleased'. The bullet exists only on PR 12's branch — main never got it (verified: grep over origin/main finds it only in a1b4405) — so I re-cut this run's branch onto PR 12's head (a1b4405) and opened PR 22 with base = PR 12's branch: its diff is exactly the move, CHANGELOG.md only, one commit of my own (90e39c1), and merging it lands the fix into PR 12, which is already held for a human merge on the CODEOWNERS path. If PR 12 merges first, retarget PR 22 to main — the diff is identical either way. Bar green in my worktree before pushing: go build/vet/test ./... (exit 0, 21 packages), gofmt -l . empty; CI on PR 22 green (build SUCCESS). PR 22: https://github.com/DnzzL/herdr-docket/pull/22

Bullet moved: PR 22's diff is CHANGELOG.md only (10+/10-, one commit 90e39c1) — the 'workspace was closed mid-flight reports cancelled again' entry now sits under '## Unreleased' (line 7, heading line 5) and is gone from the released '## v0.8.0 — 2026-10-02' section (grep confirms: single hit, under Unreleased). No code change, as the ticket asks. Verified myself in my worktree: go build ./..., go vet ./..., go test ./... exit 0 across 21 packages, gofmt -l . empty; CI on PR 22 green (build SUCCESS, run completed). The bullet existed only on PR 12's open branch, never on main, so PR 22's base is PR 12's branch and merging it corrects PR 12's CHANGELOG in place — if PR 12 merges first, retarget PR 22 to main, same diff. Not merged by me.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/22: Re-derived every ticket fact myself. Pre-state: on PR 12's head (a1b4405) the bullet sat under '## v0.8.0 — 2026-10-02' (git show a1b4405:CHANGELOG.md); bullet was added only by a1b4405 — git grep finds it absent on origin/main and absent in tag v0.8.0, so it never shipped and must land in a future release. Release convention confirmed: tag v0.8.0 = commit 196657d (2026-10-02), whose diff renames '## Unreleased' into the version section. Post-state (90e39c1): sole occurrence at CHANGELOG.md:7 under '## Unreleased' (heading :5), none in the v0.8.0 section (:55); moved 10-line bullet byte-identical to pre-state (cmp clean). Scope: git diff a1b4405...90e39c1 = CHANGELOG.md only, 10+/10-, one commit 90e39c1 whose message is the ticket's own work — no code change, as asked. Safety fact: nothing executable changed; proof is the single-file diffstat. Bar run by me in disposable worktree at 90e39c1: go build ./..., go vet ./..., go test ./... all exit 0 (21 packages), gofmt -l . empty; CI on PR 22 build pass, mergeable CLEAN. Base = PR 12's branch head, so merging lands the move into PR 12 (held for human merge); if PR 12 merges first, retarget PR 22 to main — diff identical. Not merged.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/22 — verified PASS on patch e80a7e54a19c70c783145d2dcb8b854074ded062, CI green.
<!-- SECTION:NOTES:END -->
