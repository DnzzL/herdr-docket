---
id: TASK-98
title: >-
  init <repo> appends Triage and Question after Done, and Backlog.md takes the
  last status as done
status: Triage
assignee: []
created_date: '2026-10-28 15:45'
labels: []
dependencies: []
ordinal: 107000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Backlog.md treats the last entry of backlog/config.yml statuses as the completed status (task complete / archive / cleanup check it). init <repo> appends the fleet's missing statuses at the end, so Question became the terminal status in this repo: archive refused a duplicate and complete refused every Done card until the list was reordered by hand to [Triage, Question, To Do, In Progress, Blocked, needs human validation, Done]. init must keep the queue's done word last (insert new statuses before it), and a test must say so.
<!-- SECTION:DESCRIPTION:END -->
