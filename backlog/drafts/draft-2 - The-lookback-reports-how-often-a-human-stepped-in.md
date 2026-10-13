---
id: DRAFT-2
title: The lookback reports how often a human stepped in
status: Draft
assignee: []
created_date: '2026-10-13 12:06'
labels: []
dependencies:
  - DRAFT-1
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The factory's promise is fewer rescues week over week, but no number says so.
Done when: the weekly lookback opens with the share of closed tasks a human had to touch — merged_by human, a hold, a second FAIL — this 30 days against the 30 before, from history.jsonl.
Verify by: one lookback run on the docket queue whose report states the number and the task ids behind it.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The lookback states the intervention share, this period against the last
- [ ] #2 Each number cites the task ids behind it
<!-- AC:END -->
