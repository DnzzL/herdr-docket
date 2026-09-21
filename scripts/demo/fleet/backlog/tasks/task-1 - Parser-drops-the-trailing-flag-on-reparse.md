---
id: TASK-1
title: Parser drops the trailing flag on reparse
status: In Progress
assignee:
  - dev
created_date: '2026-09-16 16:03'
updated_date: '2026-09-16 16:03'
labels: []
dependencies: []
priority: high
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Repro: `myapp parse --x` loses --x when the file is re-read.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Suspect the flag table is rebuilt from the short names alone.
<!-- SECTION:NOTES:END -->
