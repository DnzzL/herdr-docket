---
id: TASK-87
title: The factory reads well from a phone
status: In Progress
assignee: []
created_date: '2026-10-28 13:29'
labels: []
dependencies: []
ordinal: 90000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Problem

From a phone (Termux + `herdr --remote`) the owner cannot tell whether the daemon is alive, what waits on them, or answer a grill round without typing the answer syntax by hand. A terminal under 50 columns breaks the runs pane.

## Solution

The daemon writes a heartbeat each tick; every view shows `daemon live · tick 3s ago` or `daemon down for 4 min`. `herdr-docket questions` is a line-based prompt: it lists the `question` cards, shows the newest round one question at a time (Enter = the reco, a letter = that option, anything else = free text), then calls `task answer`. The runs pane opens with the daemon line and a "Your turn" section (question cards, Blocked cards, PRs at PASS awaiting a merge) and lays out under 50 columns.

## Done when

On a 45-column terminal, the owner sees whether the daemon is alive, what waits on them, and answers a round with Enter/letters only.

## Verify by

`go test ./...` green, including: a heartbeat older than 3 ticks reads as down; `questions` fed "\n B\n keep it\n" produces `1.A 2.B 3. keep it` for a three-question round whose recos are A; the pane renders with no line over 45 columns for a fixture with long titles.

## Out of scope

A full-screen TUI library; push notifications (ntfy).
<!-- SECTION:DESCRIPTION:END -->
