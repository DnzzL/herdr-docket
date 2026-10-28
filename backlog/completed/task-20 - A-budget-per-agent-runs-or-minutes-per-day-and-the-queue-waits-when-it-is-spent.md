---
id: TASK-20
title: >-
  A budget per agent: runs or minutes per day, and the queue waits when it is
  spent
status: Done
assignee: []
created_date: '2026-09-12 15:09'
updated_date: '2026-09-12 15:26'
labels: []
dependencies:
  - TASK-19
priority: high
ordinal: 20000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Nothing bounds how much an agent runs. Example 3 (the marketer that decomposes its own plan) and Example 4 (the reviewer's sweep task) both re-task themselves, which is the design — and also an unbounded loop: an agent that keeps creating a follow-up assigned to itself runs every 15 seconds until the machine or the API bill stops it. A per-run timeout_minutes caps one run, not the day. This is the one Paperclip feature (budgets with auto-pause) that guards against something real.

Keep it to what the fleet can measure honestly. Claude Code does not expose spend reliably, so no dollars: runs_per_day and minutes_per_day in AGENT.md frontmatter, both optional, unset means unbounded as today. The window is a rolling 24h read from history.jsonl (TASK-19 writes the duration), computed at pick time. Budget is state, not config: the fleet never writes disabled: true; a spent agent is Unavailable this tick and the task stays open unremarked, exactly like a busy one. herdr-docket agent list and the pane show the count (dev  4/6 runs today, 130/180 min). Manual herdr-docket run ignores the budget — pressing the button is human intent, same rule as pause.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 AGENT.md accepts runs_per_day and minutes_per_day; absent means unbounded and nothing changes for existing agents
- [ ] #2 An agent past either budget in the last 24h takes no new poll-triggered run; its To Do tasks stay open with no comment written
- [ ] #3 herdr-docket run TASK-N still starts a run for an agent over budget
- [ ] #4 agent list and the pane show used/limit for agents that have a budget
- [ ] #5 A pick test covers: under budget runs, at budget waits, budget re-opens as old runs leave the 24h window
- [ ] #6 README documents both fields under Writing an agent, with the self-tasking loop as the reason
- [ ] #7 go test ./... is green
<!-- AC:END -->
