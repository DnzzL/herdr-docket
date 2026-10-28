---
id: TASK-77
title: 'Sandbox credential: a wizard mints the dishnow-scoped fine-grained PAT'
status: Blocked
assignee:
  - plugin-dev
created_date: '2026-10-18 16:56'
updated_date: '2026-10-18 17:55'
labels: []
dependencies: []
ordinal: 77000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Spike TASK-76 proved the gap: the gh oauth token (gho_) is refused by git-over-https (401 'Invalid username or token') though valid for REST, and GitHub's API cannot create a fine-grained PAT (404), so this is a human step wrapped in a wizard. The token becomes the sandbox's only git credential: repo DnzzL/DishNow-v2 only, Contents read/write, Pull requests read/write, Metadata read, 90-day expiry, no ssh anywhere. It reaches the sandbox as ordinary environment (flake declares GITHUB_TOKEN/GH_TOKEN as "$GITHUB_TOKEN", expanded at launch, never in the flake), exported in the shell herdr starts panes from.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A wizard script walks the human through minting the fine-grained PAT in the GitHub UI with the scopes above, and stores nothing itself.
- [ ] #2 With the token exported, git push over https from inside a sandboxed pane succeeds on a throwaway branch and gh pr create works there (the 401 of TASK-76 no longer reproduces).
- [ ] #3 docs/agents.md names the token as the sandbox git credential and states that ssh is absent inside.
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
start — read TASK-76 spike notes: gho_ refused by git-over-https (401), fine-grained PAT must be minted in the GitHub UI (API 404s), so wizard + docs. Findings so far: PR #49 (sandbox: seam, docs/agents.md sandboxing section) is still OPEN, not merged — my worktree is cut from main, so my docs edit must stand alone there and I will flag the overlap in the PR. No fine-grained PAT exists anywhere on this host (herdr server env has no GITHUB_TOKEN/GH_TOKEN), so AC#2's replay needs the human to run the wizard first — the wizard will perform the push+pr-create verification itself when run. Building: scripts/sandbox-git-token.sh (mint walkthrough + verify over https: ls-remote probe, throwaway-branch push, gh pr create, cleanup; token read from stdin, env-only, stores nothing) + docs/agents.md section + changelog.d fragment.

WHAT: PR https://github.com/DnzzL/herdr-docket/pull/50 (CI green: build pass 38s, MERGEABLE) adds scripts/sandbox-git-token.sh — a wizard that walks a human through minting the dishnow-scoped fine-grained PAT in the GitHub UI (DnzzL/DishNow-v2 only, Contents read/write, Pull requests read/write, Metadata read, 90 days), reads it at a TTY only (piped input refused), enforces a github_pat_ prefix (gho_/ghp_ refused), then proves it over https exactly as AC#2 asks (ls-remote with credential helpers cleared, commit-tree commit pushed as a throwaway branch, gh pr create, then close + branch delete in an exit trap) — and stores nothing (token lives only in the process env; the one written file is an askpass helper that echoes that env and is removed on exit; /tmp leftover count unchanged across all test runs). docs/agents.md gains 'The sandbox's git credential' naming the token as the sandbox's only git credential, stating ssh is absent inside (no ~/.ssh, no agent), token as env "$GITHUB_TOKEN" never in the flake; changelog.d/TASK-77.md written. VERDICTS. AC#1 MET: wizard exists, red paths replayed on the real surface (no-TTY refused; host gho_ token refused at the prefix gate; dummy github_pat_ -> GitHub 401 'Invalid username or token', wizard exits 1 with the permissions checklist). AC#2 NOT VERIFIABLE BY THIS RUN: the fine-grained PAT does not exist yet and only a human can mint one (API 404s, TASK-76), and the replay must run inside a sandboxed pane. What I did prove: the wizard's green-path mechanics replayed by hand with the host token on the real repo — push throwaway branch, gh pr create (DnzzL/DishNow-v2#75), close, delete, all succeeded; and EVIDENCE SURPRISE: TASK-76's premise that gho_ is refused by git-over-https (401) does NOT reproduce on the host today (ls-remote AND push both succeed with the current gho_ token — bogus-password control fails, so the askpass path is genuinely exercised). The fine-grained PAT remains the right credential on authority grounds (one repo, three permissions, 90 days vs the gho_'s repo+org+admin:public_key scope), so the wizard stands; the 401 may have been sandbox-specific. AC#3 MET: docs/agents.md section quotes the token as the sandbox git credential and states ssh is absent inside. Bar: go build/vet/test ./... clean (0 fails), gofmt -l . empty. OVERLAP FLAGGED: open PR #49 (TASK-76's sandbox: seam) edits docs/agents.md too — adjacent sections, one section-level conflict for whoever merges second, no shared code. THE SINGLE QUESTION FOR A HUMAN: run scripts/sandbox-git-token.sh from inside a sandboxed DishNow pane (export GITHUB_TOKEN/GH_TOKEN in the shell that starts herdr's panes first), mint the PAT at the prompt, and paste the wizard's final output here — that output is AC#2's only admissible evidence, and it closes the loop the wizard itself verifies (push + pr create with the fine-grained token, cleaned up after).

fleet: the run's workspace wJ2 (pane wJ2:p1) is left open — jump in to resume.

Frozen 2026-10-18 with TASK-76: good idea, not now. Thaw: a decision from the human, nothing else.
<!-- SECTION:NOTES:END -->
