---
id: TASK-88
title: An idea reaches the box in one tap
status: In Progress
assignee: []
created_date: '2026-10-28 13:30'
labels: []
dependencies: []
ordinal: 94000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Problem

An idea comes on the go and is lost before the owner reaches a laptop. Running the factory on an always-on box takes a dozen manual steps nobody remembers.

## Solution

`task create` without an assignee files into `triage` when the queue has a triager (else into the pickup status, as today). `scripts/termux/idea` is a Termux:Widget script: it asks for the repo (from `herdr-docket queues` over ssh) and the text (keyboard dictation works), then runs `herdr-docket task create` on the box. `scripts/box-setup` is a bash wizard for the steps only a human can do on a fresh VPS: Tailscale, Herdr server, `gh auth login`, agent CLI login, `init <repo>` per repo, one `intake` workspace per repo, `max_runs`. README gains a "Day to day" section (phone loop, laptop loop).

## Done when

From the phone home screen, one tap and one sentence put a card in `triage` on the box.

## Verify by

`go test ./...` green including: `task create` with no assignee on a queue with a triager files into Triage. `bash -n scripts/termux/idea scripts/box-setup`; `scripts/termux/idea --dry-run` prints the ssh command it would run.

## Out of scope

Push notifications; a hosted web view.
<!-- SECTION:DESCRIPTION:END -->
