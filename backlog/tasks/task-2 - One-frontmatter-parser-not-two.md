---
id: TASK-2
title: 'One frontmatter parser, not two'
status: Done
assignee:
  - thomas
created_date: '2026-09-09 13:59'
updated_date: '2026-09-12 13:09'
labels: []
dependencies: []
priority: medium
ordinal: 2000
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 splitFrontmatter and frontmatterRange agree on every input by construction: one rule defines where the YAML header ends
- [x] #2 loader error wording for missing/unterminated frontmatter keeps its meaning, or its tests change deliberately
- [x] #3 go test ./... green
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Found while adding SetDisabled (TASK-1). Both helpers locate the frontmatter: splitFrontmatter by string prefix and Index, frontmatterRange by trimmed line compare. Divergent edges: trailing space after the closing fence, '---garbage'. Left alone in TASK-1 on purpose — rewriting the loader every agent depends on does not belong in a pause-feature commit.

AC1: frontmatterRange (with its isFence predicate) is now the only rule for where the YAML header ends; splitFrontmatter splits the file into lines and reads that same span instead of searching for its own '\n---'. TestFrontmatterHelpersAgree holds both to it across the edges that used to diverge (trailing space on either fence, CRLF, ---junk closing, only-opening-fence, empty).

AC2: wording kept: 'no frontmatter (file must start with ---)' when the first line is not a fence, 'unterminated frontmatter' when it is but no closing fence follows. TestSplitFrontmatterErrorWordingKeepsItsMeaning locks the distinction. No existing test asserted the wording.

AC3: go test ./... green; go vet clean.

Behaviour change (deliberate): a file with trailing whitespace after a fence, or CRLF line endings, now loads (splitFrontmatter used to reject it while frontmatterRange accepted it); a '---junk' line no longer silently ends the header and is reported as unterminated.
<!-- SECTION:NOTES:END -->
