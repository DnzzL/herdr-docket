---
id: TASK-80
title: 'Every ADR has its own number, and the author role speaks the glossary'
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-22 15:25'
updated_date: '2026-10-22 15:30'
labels: []
dependencies: []
ordinal: 80000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Spec: .scratch/adr-0020-duplicate/spec.md

Done when: no two ADRs share a number, every reference to the Notara ADR names 0022, the dev role uses the glossary's terms for a no-spec card, and this spec merged through the pipeline with no human step.

Verify by: go test ./... green, including the new ADR-uniqueness test (red on main before the rename); ls docs/adr | cut -c1-4 | sort | uniq -d prints nothing; grep -rn "ADR-0020\|0020-a-notara" internal docs finds no Notara reference; grep -n "card is one ticket" internal/fleet/roles/dev.md docs/examples.md finds nothing.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #52 on branch fleet/docket-task-80-…, two commits: ADR rename+test, then role wording+fragment. Criteria: (1) unique ADR numbers — new main_adr_test.go reads docs/adr off disk and failed first with 'ADR 0020 is used by 2 files: 0020-a-notara-database-is-a-queue-and-the-status-table-is-shared.md, 0020-a-settled-run-removes-its-lock-file.md'; green after renaming Notara to 0022. (2) every Notara reference names 0022 — grep -rn 'ADR-0020\|0020-a-notara' internal docs finds none; docs/queues.md link and comments in internal/work/{vocabulary.go,notara,backlogmd} updated; lock-file ADR stays 0020. (3) dev role glossary terms — 'Otherwise the card is worked as one ticket's worth of work, with no spec:' in internal/fleet/roles/dev.md and its verbatim copy docs/examples.md; grep 'card is one ticket' finds nothing; drift test green. Checks: go build/vet clean, gofmt -l . empty, go test ./... all green, gh PR checks build pass. Changelog fragment changelog.d/TASK-80.md. (4) merged with no human step is the verifier+gate's to prove, not mine — delivery is this PR.

Verified myself: unnumbered ADR test red before rename naming both 0020 files, green after; ls docs/adr | cut -c1-4 | sort | uniq -d empty; no 'ADR-0020'/'0020-a-notara' left in internal or docs; 'card is one ticket' gone from the shipped role and docs/examples.md, drift test green; go build/vet clean, gofmt -l . empty, go test ./... green, PR check build pass. Lock-file ADR keeps 0020. Remaining criterion — merged through the pipeline with no human step — is the verifier and gate's to satisfy.

verdict PASS on https://github.com/DnzzL/herdr-docket/pull/52: Verified at 9420383 in the fleet worktree. (1) ADR uniqueness: new main_adr_test.go reads docs/adr off disk; copied into a throwaway worktree at 3d1a076 it goes red exactly as intended — "ADR 0020 is used by 2 files: 0020-a-notara-...md, 0020-a-settled-run-removes-its-lock-file.md" — and passes at the PR head; ls docs/adr | cut -c1-4 | sort | uniq -d empty; renamed file is 97% similarity, title line only, content untouched; lock-file ADR keeps 0020. (2) References: repo-wide grep for 0020 matches only the test's own comment lines; queues.md link and internal/work/{vocabulary.go,notara/notara.go x2,backlogmd/vocabulary.go} now cite ADR-0022, all were Notara-table references, no lock-file reference was touched. (3) Role: internal/fleet/roles/dev.md and docs/examples.md carry the same paragraph now — "Otherwise the card is worked as one ticket's worth of work, with no spec" — verbatim match to CONTEXT.md:64-69; grep 'card is one ticket' finds nothing; TestTheRolesInitWritesAreTheOnesTheDocsShow green. Bar on the branch: go build ./... , go vet ./... , go test ./... all clean, gofmt -l . empty. Commits are the two tickets' own plus the required changelog.d/TASK-80.md; no behaviour change in the adapters (comments only). Criterion 4 (merged through the pipeline with no human step) is this run and the gate's to prove, not the diff's — proceed to the gate.

fleet: merged https://github.com/DnzzL/herdr-docket/pull/52 — verified PASS on patch 9107e6dc686726e05646af95f5eca2025522bdc0, CI green.
<!-- SECTION:NOTES:END -->
