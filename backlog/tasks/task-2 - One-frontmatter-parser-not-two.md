---
id: TASK-2
title: 'One frontmatter parser, not two'
status: To Do
assignee:
  - thomas
created_date: '2026-09-09 13:59'
labels: []
dependencies: []
priority: medium
ordinal: 2000
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 splitFrontmatter and frontmatterRange agree on every input by construction: one rule defines where the YAML header ends
- [ ] #2 loader error wording for missing/unterminated frontmatter keeps its meaning, or its tests change deliberately
- [ ] #3 go test ./... green
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Found while adding SetDisabled (TASK-1). Both helpers locate the frontmatter: splitFrontmatter by string prefix and Index, frontmatterRange by trimmed line compare. Divergent edges: trailing space after the closing fence, '---garbage'. Left alone in TASK-1 on purpose — rewriting the loader every agent depends on does not belong in a pause-feature commit.
<!-- SECTION:NOTES:END -->
