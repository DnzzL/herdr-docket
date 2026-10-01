---
id: TASK-45
title: A run inherits whatever branch the human left checked out
status: To Do
assignee:
  - plugin-dev
created_date: '2026-10-01 09:42'
labels: []
dependencies: []
ordinal: 45000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Found dogfooding plugin-dev on 2026-10-01. The runner provisions a worktree from the main checkout's current HEAD, not from the queue's default branch. A human mid-branch therefore contaminates every agent run started while they are there.

Observed: the shell was left on fix/sweep-coverage-is-a-fact-about-the-pr with one unmerged commit. docket/TASK-23's run branched from it, and PR #2 against main carries TWO commits by two authors — the agent's ADR plus the human's unrelated fix, presented as part of the ticket. Nothing warned anybody; the run and the record both read clean.

Damage runs both ways: a PR that conflates authors and lands a human commit through an agent's PR without review of its own, and an agent inheriting half-finished human work as its base.

Close relative of TASK-38 (the host names the branch a run works on).

Acceptance criteria:
#1 A worktree run branches from the queue's default branch whatever the main checkout has checked out — a test pins it with the checkout on an unrelated branch.
#2 Where that is deliberately not wanted, the base is a stated choice, not an accident of where somebody's shell was.
#3 The run record names the commit the run branched from, so a contaminated PR is visible in history and not only on the forge.
<!-- SECTION:DESCRIPTION:END -->
