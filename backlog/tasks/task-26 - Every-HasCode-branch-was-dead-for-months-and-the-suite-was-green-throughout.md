---
id: TASK-26
title: Every HasCode branch was dead for months and the suite was green throughout
status: To Do
assignee:
  - fleet-dev
created_date: '2026-09-13 20:54'
labels: []
dependencies: []
priority: high
ordinal: 26000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`8624cc8` fixed a one-line defect: `newAPIError` read the error envelope from stdout, herdr writes it to stderr, so `APIError.Code` was empty on every error the fleet ever saw. Seven `HasCode` branches never ran — the prompt-stall recovery, the `agent_not_ready` and `pane_busy` retries at start, the vanished agent, the vanished workspace.

That is not the interesting part. The interesting part is that `internal/host` has a thorough test suite, `internal/herdr` had a test named `TestNewAPIErrorPrefersHerdrsOwnCode`, and all of it was green for months while none of that code could execute.

It went green because every test built its `APIError` the way the code wished the world worked. `TestNewAPIErrorPrefersHerdrsOwnCode` passes an envelope on **stdout** — the stream herdr does not use. `internal/host`'s fake ops returns a `*herdr.APIError` constructed by hand, with `Code` already filled in, so `HasCode` matched and the recovery ran beautifully in the test and never once in production.

The bug was not in the code under test. It was in the boundary the tests agreed to pretend about.

## What to do about it

Not "add a test for stderr" — `8624cc8` already did that. The question is which other seams the suite is currently agreeing to pretend about, and what kind of test would have caught this one.

Some of what is worth weighing:

- **A contract test against the real `herdr` binary**, marked so it is skipped when herdr is absent: force one real error of each shape the fleet branches on and assert the fleet reads a code out of it. That is what would have caught this, and nothing cheaper would have.
- **`internal/work/worktest` is the model to copy.** The adapters have a conformance suite precisely so "does this behave like a queue?" is a test rather than a review. The herdr boundary has no equivalent, and it is the other process boundary the whole system rests on.
- **Every fake that hand-builds a type the real code parses** deserves the same suspicion. `host`'s fake ops is one. Ask what else constructs a value that production derives.
- Whether a code the fleet branches on but herdr no longer emits should fail loudly rather than silently never match.

## The bar

Not coverage. The suite already covered this line. What is missing is one test that would have gone red on 12 September and did not.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A test exists that fails if the fleet stops reading a real herdr error's code, and it derives that error from herdr rather than constructing it
- [ ] #2 It is skipped rather than failed where herdr is not installed, and says which it did
- [ ] #3 Every HasCode code the fleet branches on is exercised against a real error of that shape, or is documented as unreachable and why
- [ ] #4 The closing note names the other seams where a fake constructs what production parses, as findings rather than fixes
<!-- AC:END -->
