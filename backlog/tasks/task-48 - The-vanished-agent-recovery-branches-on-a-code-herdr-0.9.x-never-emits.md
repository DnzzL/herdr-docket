---
id: TASK-48
title: The vanished-agent recovery branches on a code herdr 0.9.x never emits
status: Blocked
assignee:
  - plugin-dev
created_date: '2026-10-01 15:13'
updated_date: '2026-10-03 22:49'
labels: []
dependencies: []
ordinal: 48000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Found while building the herdr contract suite (TASK-26): herdr 0.9.1 answers {"error":{"code":"agent_not_found"}} to agent get / wait / prompt for a target that never registered — and, probed in a throwaway workspace closed under a live agent, to a REGISTERED agent whose workspace was closed too. The fleet's CodeAgentGone is "agent_not_running" (internal/host/agent.go:169, and the await slice that maps it to ErrCancelled); HasCode matches nothing this herdr emits, so a run whose workspace dies mid-flight never reports ErrCancelled — it waits out its remaining slices and reports the last wait error. Decide the fix: broaden the match (both codes), retarget the constant, or pin that herdr emits agent_not_running somewhere the probes did not look. The contract suite in internal/herdr/contract_test.go pins the actual emission; change goes with contract renegotiated in the same commit.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Run started: reading the vanished-agent path (internal/host/agent.go await/submit), the contract suite, and probing herdr 0.9.1 for what agent_not_running actually attaches to before deciding broaden/retarget/pin.

Probing planned: (1) agents in live workspaces get/wait/prompt after workspace close — task says that answered agent_not_found; (2) agent that exits on its own in a throwaway workspace; (3) wait timeout shape. Throwaway workspaces only, closed by the probe.

Probe 1 confirms task's observation: agent get/wait after workspace close answers agent_not_found exactly. Herdr never emitted agent_not_running in that shape. Now checking whether agent_not_running is reachable at all (agent exits on its own, pane alive).

Probe 2: agent exited on its own in a live workspace — herdr answers agent_not_found to get/wait/prompt there too. agent_not_running (live in the 0.9.1 binary, 'agent is no longer running in the target pane') not reachable. Next: what does the pane registry's loss actually look like mid-run in the fleet (agent rename probes), then decide fix.

Probes complete. NOT_REACHABLE forward: one-shot pi (pi --version) exits within 30s in a live workspace, herdr still answers nothing there; a pane whose agent died also answers agent_not_found only. agent_not_running is genuinely dead in 0.9.x. Verdict: retarget the constant where herdr 0.9.x sends it, tau the contract accordingly. Writing test first.

Correction to previous note, written badly: the plan is to retarget the gone-agent constant to what herdr 0.9.x actually emits for every vanished-agent shape probed — agent_not_found — and renegotiate the contract suite in the same commit. Probes: (1) agent get/wait/prompt after workspace close: agent_not_found; (2) agent exited on its own in live workspace: agent_not_found; (3) one-shot agent (pi --version) that exits during its own boot: agent_not_found; (4) never-registered target: agent_not_found. agent_not_running is present in the 0.9.1 binary but unreachable by all four shapes the fleet lives on. Now test first in the contract suite and unit suite, then the rename.

Notable: the workflow's own env pins HERDR_BIN_PATH to herdr 0.9.0 while the interactive shell has 0.9.1; both behave identically on this question (agent_not_found for every vanished shape probed, agent_not_running unreachable on both — 0.9.0's binary checked too). Note for the daemon's own runs: HERDR_BIN_PATH wins over PATH, so what the contract suite holds against depends on the env it inherits. Test now red against the constant as-pinned (agent_not_running); making the change next.

Fix shipped as PR 12 (https://github.com/DnzzL/herdr-docket/pull/12): CodeAgentGone retargeted from agent_not_running to agent_not_found (the code herdr 0.9.0/0.9.1 answers on all four probed vanished-agent shapes); contract suite renegotiated in the same commit, live-probing get and wait; CHANGELOG entry added. CI running — watching for green before handing on.

Retargeted CodeAgentGone to agent_not_found and renegotiated the contract suite in the same commit. Verified myself: probed herdr 0.9.0 and 0.9.1 live in throwaway workspaces (closed by the probe) — agent get/wait/prompt answer agent_not_found for unregistered, workspace-closed-under-agent, self-exited and boot-failed agents; agent_not_running is unreachable through any call the fleet makes, so retarget over broaden. Contract test was red against the old pin, green after; full local bar green (build/vet/test/gofmt); remote CI green (PR 12). Left with plugin-reviewer; not merged by me.

Stall sweep (fleet/TASK-21), pass 7, stall type 3 — your closing note says 'Left with plugin-reviewer; not merged by me' (PR 12). That promised follow-up has no home: the live open queue (fleet/TASK-1/2/9/11/13/16/20, docket/TASK-49/50) contains no task mentioning PR #12. gh this sweep: PR #12 OPEN at head c28bbb8, zero reviews. Who it waits on: the human — route PR #12's review/merge. Nudge only: I do not reopen, close, assign, or file work of my own.

Re-queued 2026-10-03 (cleanup): PR #12 (https://github.com/DnzzL/herdr-docket/pull/12) is still open from before the pipeline (ADR 0013). Rebase it onto main (the board, budgets and pm are gone; prompt and runner changed in #14/#15), make it green, then deliver it with task done --pr. If main already does what it does, close the PR and say so in a block note instead.

Re-queued slice done: PR 12 rebased onto current main (795153e → head a1b4405, clean rebase, same 4-file diff — main never absorbed the fix, CodeAgentGone still agent_not_running there). Local bar green (build/vet/test/gofmt), contract probe replayed live against herdr 0.9.0 (red→green history kept in the original commits), CI green on a1b4405, PR MERGEABLE/CLEAN. PR body updated with the rebase line.

Rebased PR 12 onto main (head a1b4405) as the re-queue asked; main did not already contain the fix (CodeAgentGone still agent_not_running at origin/main). Verified myself: clean rebase with the same 4-file diff (CHANGELOG, herdr.go constant, contract suite, host test); full local bar green (go build/vet/test, gofmt empty); contract suite replayed live after rebase against the env-pinned herdr 0.9.0 binary — TestHerdrAnswersAVanishedAgentWithAgentNotFound holds (agent_not_found for get and wait); CI green on the new head (run 37156975796), PR MERGEABLE. Not merged by me — left with plugin-reviewer.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/12: Verified in a disposable worktree at head a1b4405 (1 commit, 4 files, all the ticket's own). Bar green: go build/vet/test ./..., gofmt -l empty. Decisive fact re-derived live, not from the PR's claims: (1) throwaway workspace w9P opened by me, pi agent t48-probe registered in it, workspace closed under it — herdr 0.9.0 answers {"error":{"code":"agent_not_found"}} to agent get, wait AND prompt (raw envelopes, exit 1); (2) never-registered target: same agent_not_found on 0.9.0 (HERDR_BIN_PATH, the env the contract suite inherits) and on 0.9.1 (/run/current-system/sw/bin/herdr); (3) agent_not_running exists in the 0.9.0 binary strings ('agent is no longer running in the target pane') but no call reached it; probe workspace closed by the probe (workspace list count 0). Red proof: constant reverted to agent_not_running in my worktree → TestHerdrAnswersAVanishedAgentWithAgentNotFound FAIL at internal/herdr/contract_test.go:90 (saw agent_not_found, wanted agent_not_running); restored → PASS against the real binary. So the retarget is the one fact the fix is safe on, proven both directions. Behaviour path: await maps it to ErrCancelled at internal/host/agent.go:169, exercised by internal/host/agent_test.go:221. Contract renegotiated in the same commit as the constant (a1b4405). CI run 37156975796 green — note the contract suite skips there (no herdr installed); its live hold is the local run above. PR MERGEABLE/CLEAN and git merge-tree vs origin/main 570ded5: 0 conflicts. Non-blocking finding filed as docket/TASK-60: CHANGELOG bullet sits under the released, tagged v0.8.0 section instead of Unreleased.

fleet: https://github.com/DnzzL/herdr-docket/pull/12 is held for a human — internal/host/agent_test.go is a CODEOWNERS path (/internal/host/).
<!-- SECTION:NOTES:END -->
