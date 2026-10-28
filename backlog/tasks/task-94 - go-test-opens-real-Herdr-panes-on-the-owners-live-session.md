---
id: TASK-94
title: go test opens real Herdr panes on the owner's live session
status: Triage
assignee: []
created_date: '2026-10-28 15:39'
labels: []
dependencies: []
ordinal: 103000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
internal/herdr/contract_test.go runs against the real herdr binary whenever it is installed (it only skips when herdr is missing). Every go test ./... — the owner's, and every fleet agent's — opens 'docket contract' panes in the owner's live Herdr session, mid-typing, then closes them. The contract suite must never touch the owner's session: opt-in (an env var CI sets), or against its own isolated herdr server/socket.
<!-- SECTION:DESCRIPTION:END -->
