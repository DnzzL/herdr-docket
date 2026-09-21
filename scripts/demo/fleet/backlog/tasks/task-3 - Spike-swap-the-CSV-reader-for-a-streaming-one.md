---
id: TASK-3
title: 'Spike: swap the CSV reader for a streaming one'
status: To Do
assignee: []
created_date: '2026-09-16 16:03'
updated_date: '2026-09-16 16:03'
labels: []
dependencies: []
priority: low
ordinal: 3000
---

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
The current reader buffers the whole file; a streaming one would bound memory but changes the progress callback.
<!-- SECTION:NOTES:END -->
