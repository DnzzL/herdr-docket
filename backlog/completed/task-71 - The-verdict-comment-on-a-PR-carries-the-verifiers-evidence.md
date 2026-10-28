---
id: TASK-71
title: The verdict comment on a PR carries the verifier's evidence
status: Done
assignee: []
created_date: '2026-10-13 12:06'
updated_date: '2026-10-28 15:29'
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Checked HEAD before writing code: the behaviour is already shipped. main.go:789-791 builds the verdict line then appends '\n\n'+note; the regression test TestTheVerdictCommentKeepsLineOneAndCarriesTheNote (main_task_test.go) runs through task verdict and asserts line one is exactly 'docket-verdict: PASS patch-id=p42' and the note is under it. Test arrived in 87db665 (#43), note append in 848ec37 (#15); this task's premise cites the pre-#15 main.go:641 and is stale. No code change needed, no PR.

Already shipped at HEAD; no new code. AC1 met: main.go:789-791 leaves the first line 'docket-verdict: %s patch-id=%s' untouched (rewritten only to add the note after '\n\n'). AC2 met: the verifier's note follows that line on the PR. Evidence: TestTheVerdictCommentKeepsLineOneAndCarriesTheNote (main_task_test.go, added in 87db665) goes through task verdict and asserts exact first line + note presence; passes here. Full bar green: go build ./..., go vet ./..., go test ./..., gofmt -l . empty.
<!-- SECTION:NOTES:END -->
