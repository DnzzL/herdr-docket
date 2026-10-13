---
id: TASK-71
title: The verdict comment on a PR carries the verifier's evidence
status: To Do
assignee: []
created_date: '2026-10-13 12:06'
updated_date: '2026-10-13 12:06'
labels: []
dependencies: []
ordinal: 71000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
A held PR shows only 'docket-verdict: PASS patch-id=…' (main.go:641). The evidence the verifier re-derived lives in the task note, so the human who must merge reviews blind.
Done when: the docket-verdict comment carries the verifier's note under the verdict line; the line itself is unchanged, so anything parsing it still works.
Verify by: a test through task verdict that the comment body starts with the unchanged line and contains the note.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The comment keeps its first line unchanged
- [ ] #2 The verifier's note follows it on the PR
<!-- AC:END -->
