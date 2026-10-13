---
id: TASK-51
title: 'Rebase PR #9 onto main — the pickup-status work landed on the same seam'
status: Done
assignee:
  - plugin-dev
created_date: '2026-10-02 08:19'
updated_date: '2026-10-03 11:08'
labels: []
dependencies: []
ordinal: 51000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
PR #9 (your TASK-22 work: acceptance criteria through the fleet CLI) no longer merges. TASK-40's pickup-status work landed as #6 and touches the same places: internal/work/work.go, internal/work/backlogmd/cli.go, internal/work/multi/multi_test.go, main.go.

Both extend the work port and the task create path, so this is a reconciliation of two designs rather than a textual fixup — which is why it comes back to you rather than being resolved by hand at the merge button.

What to do: rebase your branch onto current main, reconcile the two extensions of the port so neither loses its behaviour, and push. Keep both features: a created task lands in the queue's pickup status AND can carry acceptance criteria. The conformance suite in internal/work/worktest is the arbiter — if both capabilities pass it, the reconciliation is right.

Acceptance criteria:
#1 PR #9 merges into main with no conflict, and build/vet/test/gofmt are all green on the merge result.
#2 Neither feature regressed: a test pins a created task landing in the pickup status, and a test pins criteria surviving a create through the port.
#3 The port's conformance suite passes for every adapter, backlogmd and multi included.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Rebase complete and reconciled: both features kept, conformance suite green across adapters, PR mergeable and CI green.

PR-9 rebased onto main. Conflict resolution kept TASK-40's landing semantics (CreateIn/CreateTodo started-bool, --s status, ADR-0012) and grafted TASK-22's criteria on top: task create --ac parses, createTask keeps (id, started, err), then putCriteria(src, id, criteria) via work.CriterionWriter — one criteria seam after the create. backlogmd gained SetCriteria (task edit --ac) beside its TodoStarter create; multi's CreateIn stays TodoStarter-routing and the composite forwards WriteCriteria by sub-name. ADR renumbered to 0013 (0012 taken) with the reconciliation recorded; CHANGELOG one Unreleased section with both bullets. New test: TestTaskCreateFilesThePickupStatusThenCarriesTheBar pins both capabilities on one CLI create, in order. CI bar locally: build/vet/test(-count=1)/gofmt clean; PR #9 MERGEABLE, build SUCCESS. PR body updated with reconciliation.

fleet: this run's delivery could not be read — workspace w76 (pane w76:p1) is kept, unverified. The verdict stands.

Stall sweep (fleet/TASK-21), pass 7, unclassifiable, addressed to the human as my persona directs. gh this sweep: PR #9 OPEN at head 8128b9c, 0 reviews, 0 comments after your rebase; the live open queue (fleet/TASK-1/2/9/11/13/16/20, docket/TASK-49/50) carries no task mentioning PR #9, so the review/merge step has no owner. But none of the three stall types fits: your history record flags no uncommitted work (verdict done, no uncommitted field), your closing note promises no follow-up, and the row is not parked on a question. You cannot classify it for me — say what it is: real stall (route PR #9's review) or not. Nudge only: I do not reopen, close, assign, or file work of my own.
<!-- SECTION:NOTES:END -->
