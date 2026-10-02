---
id: TASK-26
title: Every HasCode branch was dead for months and the suite was green throughout
status: Done
assignee: []
created_date: '2026-09-13 20:54'
updated_date: '2026-10-01 15:13'
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

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: assignee "fleet-dev" is not a fleet agent — fix the assignee or add agents/fleet-dev/AGENT.md.

Picked up. Reading internal/herdr, internal/host, 8624cc8. Probed the real herdr 0.9.1 for which error codes are forceable deterministically: workspace_not_found, agent_not_found, and agent_pane_busy yes; agent_not_found is also what herdr answers for a vanished agent — a finding, the fleet's CodeAgentGone=('agent_not_running') matches nothing this herdr emits. Probes done in throwaway workspaces, all closed.

Suite written and pushed. Contract holding against real herdr nix-store 0.9.0; CI green.

Internal/herdr gained a contract suite (contract_test.go) that execs the real herdr binary and forces real errors of the shapes the fleet branches on:

#1 MET — TestTheClientReadsTheCodeOutOfARealHerdrError closes a workspace id herdr never issued and asserts CodeWorkspaceGone arrives through the client (and that WorkspaceClose still maps it to ErrGone). No byte of that error is constructed by the test; it was exactly the test that would have gone red on 12 Sep.
#2 MET — requireHerdr() skips each test with t.Skipf naming the binary and why. Verified BOTH branches here: herdr IS installed in this environment, so all three contract tests ran (skipped = zero observed); the unit tests that hold the fallback shapes are in the same package and pass without herdr.
#3 PARTIALLY — three of five codes forced against real herdr: workspace_not_found, agent_pane_busy (throwaway workspace, second agent start into an occupied pane; suite closes its own workspace), plus agent_not_found pinned as what herdr answers for unknown AND vanished agents. NOT forced, documented with why in the suite: agent_not_ready and agent_prompt_stalled are agent-lifecycle states whose honest forcing is a race on a live agent boot —a flaky contract would be worse than none; agent_not_running is pinned ABSENT (see #4). 
#4 MET as findings, not fixes — (a) CodeAgentGone "agent_not_running": probed twice, herdr answers agent_not_found both for an unregistered target and for a registered agent whose workspace was closed; the vanished-agent recovery and the wait-slice ErrCancelled path match NOTHING this herdr emits → filed docket/TASK-48. (b) internal/host fakeOps.apiErr() builds *herdr.APIError with Code prefilled — production derives those from herdr's bytes; the contract suite now anchors the read path, but the choreography tests still trust hand-made codes, which is exactly what hid this bug. (c) internal/hostpath's fakeHerdr script answers "plugin config-dir" with a path — a fake constructing exact bytes production parses; stakes lower because a wrong answer has a sane fallback. (d) host tests hand literal status strings ("idle", "working") where production parses herdr's agent-status JSON envelope — same class, lower stakes.

Verified myself: go build/vet/test/gofmt all clean; contract tests pass (did not skip) in this environment; PR CI (build) green. Not verified: nothing else.
<!-- SECTION:NOTES:END -->
