---
id: TASK-70
title: A hold settles when its pull request does
status: To Do
assignee: []
created_date: '2026-10-13 12:06'
updated_date: '2026-10-13 12:06'
labels: []
dependencies: []
ordinal: 70000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A held task stays Blocked forever after the human merges or closes its PR: TASK-48, 53, 57, 62, 63, 66 and 69 are Blocked although PRs #12, #13, #19, #23, #24, #27, #31 are merged. The queue lies and the Blocked column fills with noise.
Done when: every few minutes (not every tick) the fleet reads the PR of each held task through the forge; MERGED closes the task done, CLOSED unmerged closes it failed with a note saying a human closed it; OPEN leaves it alone. The fleet never merges here. CONTEXT.md's Hold entry says a hold waits on its PR and settles with it.
Verify by: a runner test through the seam for MERGED, CLOSED and OPEN; and live, the seven tasks above close done on the first pass.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A held task whose PR is merged closes done
- [ ] #2 A held task whose PR is closed unmerged closes failed, saying a human closed it
- [ ] #3 An open PR leaves its task Blocked
- [ ] #4 The PR state is read every few minutes, not every tick
- [ ] #5 CONTEXT.md's Hold says a hold settles with its PR
<!-- AC:END -->
