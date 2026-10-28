---
id: TASK-79
title: >-
  Sandbox network: derive and apply the allowedDomains allowlist for dishnow
  authors
status: Blocked
assignee:
  - plugin-dev
created_date: '2026-10-18 16:56'
updated_date: '2026-10-18 17:55'
labels: []
dependencies: []
ordinal: 79000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Network is open in the TASK-76 sandbox; this is the last least-privilege axis. Derive the list rather than guess it: run one instrumented dishnow author run and read the sandbox proxy's log (agent-sandbox documents the method), against the expected set - github.com/codeload/api.github.com (git over https, gh), registry.npmjs.org (bun install), api.anthropic.com/claude endpoints (claude). Tune until a real run passes with the list on, then keep the tuning cheap by naming the deny log in the docs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 The flake's allowedDomains is set for the dishnow author sandbox, and the notes carry the list plus how each entry was derived.
- [ ] #2 A real dishnow author run completes with the allowlist on: bun install, the repo's tests, git push over https, PR opened.
- [ ] #3 docs/agents.md names the proxy deny log, so a future failure reads as 'domain not allowed' and not as a mystery timeout.
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
fleet: the agent settled without reporting a verdict.

fleet: the run's workspace wJ6 (pane wJ6:p1) is left open — jump in to resume.

Frozen 2026-10-18 with TASK-76: good idea, not now. Thaw: a decision from the human, nothing else.
<!-- SECTION:NOTES:END -->
