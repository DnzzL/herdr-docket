---
id: TASK-81
title: A spec's ticket files are resolved when its pull request delivers
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-22 15:35'
updated_date: '2026-10-22 15:39'
labels: []
dependencies: []
ordinal: 81000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
After TASK-80 merged, .scratch/adr-0020-duplicate/issues/*.md still read Status: ready-for-agent: the author never resolved them and the verifier never checked.

Change the two shipped roles (internal/fleet/roles, mirrored verbatim in docs/examples.md):
- dev, under Hand back: before delivering a spec, set every ticket file of the spec to Status: resolved.
- reviewer, in the spec paragraph: with a spec, `grep -L 'Status:.*resolved' <spec dir>/issues/*.md` prints nothing, or the PR FAILs.
Also resolve the two TASK-80 ticket files under .scratch/adr-0020-duplicate/issues/.

Done when: both roles carry the rule, and no ticket file of a merged spec on main reads ready-for-agent.

Verify by: go test ./... green (role drift test); grep -L 'Status:.*resolved' .scratch/*/issues/*.md prints nothing on the PR head.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Both roles now carry the rule, doc mirror updated, TASK-80 tickets resolved.

- internal/fleet/roles/dev.md: Hand back adds 'before delivering a spec, set every ticket file under its issues/ to Status: resolved'.
- internal/fleet/roles/reviewer.md: spec paragraph adds the grep -L 'Status:.*resolved' <spec dir>/issues/*.md must-print-nothing rule, FAIL otherwise.
- docs/examples.md: both paragraphs mirrored verbatim (TestTheRolesInitWritesAreTheOnesTheDocsShow green).
- .scratch/adr-0020-duplicate/issues/01+02: ready-for-agent -> resolved.
- changelog.d/TASK-81.md: user-visible role text change.

Verified myself: go build/vet/gofmt -l (empty)/go test ./... all green; grep -L 'Status:.*resolved' .scratch/*/issues/*.md prints nothing on this head. PR: https://github.com/DnzzL/herdr-docket/pull/53

Both role files carry the rule, docs/examples.md mirrors them verbatim, TASK-80's two tickets are resolved. Criteria: (1) roles carry the rule - met, see diffs in internal/fleet/roles/dev.md (Hand back) and reviewer.md (spec paragraph); (2) no ticket file of a merged spec reads ready-for-agent - met, grep -L 'Status:.*resolved' .scratch/*/issues/*.md prints nothing on the PR head. Verification: go build ./..., go vet ./..., gofmt -l . (empty), go test ./... all green, including the role drift test. PR: https://github.com/DnzzL/herdr-docket/pull/53

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/53: Verified on the provisioned worktree at a295c09 (diff vs origin/main = 6 files: dev.md, reviewer.md, docs/examples.md, changelog.d/TASK-81.md, TASK-80 issues/01+02). Criteria: (1) dev rule present, internal/fleet/roles/dev.md:14-16 'Before delivering a spec, set every ticket file under its issues/ to Status: resolved'; reviewer rule present, internal/fleet/roles/reviewer.md:25-27 'grep -L Status:.*resolved <spec dir>/issues/*.md ... that FAILs'; docs mirror at docs/examples.md:117 and :205, confirmed by TestTheRolesInitWritesAreTheOnesTheDocsShow. (2) grep -L 'Status:.*resolved' .scratch/*/issues/*.md printed nothing on this head (all 7 ticket files, incl. the two TASK-80 ones at .scratch/adr-0020-duplicate/issues/, now read **Status:** resolved). No ticket in the diff beyond the two the task names. Bar: go build ./... OK, go vet ./... OK, gofmt -l . empty (0 files), go test ./... all packages ok. Drift test is not a fake: copying the tree to /tmp and breaking one word in docs/examples.md makes TestTheRolesInitWritesAreTheOnesTheDocsShow red ('role dev is not in docs/examples.md verbatim'), green on this head. No executable code touched, so no red-before test owed. Non-blocking observation for plugin-dev: each spec's own .scratch/*/spec.md still reads 'Status: ready-for-agent' after merge (e.g. .scratch/adr-0020-duplicate/spec.md:1); out of TASK-81's scope (ticket files only), worth its own card. Human look not required.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/53 — verified PASS on patch 7c7514513aaa2aa3855e582e45bbb11ed62cfa13, CI green.
<!-- SECTION:NOTES:END -->
