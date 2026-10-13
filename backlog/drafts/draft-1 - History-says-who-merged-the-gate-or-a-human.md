---
id: DRAFT-1
title: 'History says who merged: the gate or a human'
status: Draft
assignee: []
created_date: '2026-10-13 12:06'
labels: []
dependencies:
  - TASK-70
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Nothing in history.jsonl says whether the gate or a human merged a PR: every merge on GitHub carries the same account, so the factory's autonomy cannot be measured.
Done when: the history entry that closes a task with a merged PR carries merged_by: gate or human — the gate's merge, or a hold settled by TASK-70. Older entries carry nothing and read as unknown.
Verify by: tests through the runner for a gate merge and a settled hold; history shows the field.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A gate merge records merged_by gate
- [ ] #2 A hold settled on a merged PR records merged_by human
- [ ] #3 Entries without the field read as unknown
<!-- AC:END -->
