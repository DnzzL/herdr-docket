---
id: TASK-48
title: The vanished-agent recovery branches on a code herdr 0.9.x never emits
status: To Do
assignee:
  - plugin-dev
created_date: '2026-10-01 15:13'
labels: []
dependencies: []
ordinal: 48000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Found while building the herdr contract suite (TASK-26): herdr 0.9.1 answers {"error":{"code":"agent_not_found"}} to agent get / wait / prompt for a target that never registered — and, probed in a throwaway workspace closed under a live agent, to a REGISTERED agent whose workspace was closed too. The fleet's CodeAgentGone is "agent_not_running" (internal/host/agent.go:169, and the await slice that maps it to ErrCancelled); HasCode matches nothing this herdr emits, so a run whose workspace dies mid-flight never reports ErrCancelled — it waits out its remaining slices and reports the last wait error. Decide the fix: broaden the match (both codes), retarget the constant, or pin that herdr emits agent_not_running somewhere the probes did not look. The contract suite in internal/herdr/contract_test.go pins the actual emission; change goes with contract renegotiated in the same commit.
<!-- SECTION:DESCRIPTION:END -->
