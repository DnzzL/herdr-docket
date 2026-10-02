---
id: TASK-47
title: >-
  Root-mode runs open a tab in the project's workspace, not a workspace beside
  it
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-01 11:14'
updated_date: '2026-10-01 14:10'
labels: []
dependencies: []
ordinal: 47000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A fleet of three projects puts 29 workspaces in the Herdr sidebar, and only three of them mean anything.

WHAT THE EXPERIMENT SETTLED (2026-10-01):
- herdr derives workspace grouping from worktree.repo_key, not from a parent pointer. Proven: 'worktree create --workspace <id>' and '--cwd <path>' return byte-identical workspaces (is_linked_worktree: true, same repo_key). The --workspace flag names the repo, it is not a parent.
- So the 17 worktree runs are ALREADY grouped under the project's primary workspace. Nothing to fix there; 'workspace close --group' reaches them.
- The 12 root-mode runs are the loose ones. 'workspace create' takes only --cwd/--label/--env, so herdr can never associate them with a repo. Their worktree field is null.
- The one route is 'herdr tab create --workspace <id> --cwd <path>': the run becomes a tab inside the project's primary workspace.
- The parent is DERIVED, never configured: the primary is the workspace whose worktree.repo_key is <project dir>/.git with is_linked_worktree false. No workspace_id in fleet.yaml — a pasted id rots.
- Agent-name collisions (ADR 0009's problem in herdr-automations) do not apply: RunTag already makes every run's agent name unique.

THE HAZARD, which is the whole cost: a run is stopped today by closing its workspace. For a tab-run that would close every sibling tab in the project's workspace, including the human's own. Stopping must close the tab. The board's x stops a run from the history record, not a live session, so what the run owns has to reach history.

Acceptance criteria:
#1 A root-mode run whose project has a primary workspace opens a tab in it; the run's cwd is unchanged.
#2 A root-mode run whose project has no primary workspace opens its own workspace, as today. The plugin never creates a primary it did not make.
#3 Stopping or cleaning up a tab-run closes the tab and leaves the workspace and its siblings alone — pinned by a test, because getting this wrong closes a human's work.
#4 Worktree runs are untouched: they are already grouped by repo_key.
#5 The board's x does the right thing from the history record alone.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Delivered by d429e8c (this session). I filed this ticket with the research and then implemented it in the same sitting, so it never reached plugin-dev. AC#1: a root run borrows the project's primary workspace as a tab, cwd unchanged. AC#2: no primary, no borrowing — the fleet never creates one. AC#3: TestClosingABorrowedWorkspaceClosesOnlyTheTab pins the hazard; my first patch had exactly that bug (closed workspace AND tab) and the test is why it did not ship. AC#4: worktree runs untouched. AC#5: closeTarget reads the decision from the history record alone. Verified the herdr call shapes against a live probe tab before relying on them.
<!-- SECTION:NOTES:END -->
