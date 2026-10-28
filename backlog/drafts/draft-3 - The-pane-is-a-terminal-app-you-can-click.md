---
id: DRAFT-3
title: The pane is a terminal app you can click
status: Draft
assignee: []
created_date: '2026-10-28 14:13'
labels: []
dependencies: []
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Problem

The owner answers grill rounds from a phone (Termux) and a desktop terminal. The line prompt costs a letter and Enter per answer, shows no context, and the raw-ANSI pane takes no click. Herdr's philosophy is terminal first with click support.

## Solution

ADR 0024. `herdr-docket pane` becomes a bubbletea v1.3 + lipgloss v1.1 app (alt screen, mouse cell motion). Home screen: daemon line, Your turn (`?` question, `!` blocked, `⇡` held merge), In flight, Settled; click/tap or Enter on a `?` row opens its Round. Round screen: the round's context paragraph, then each question with its options as buttons (wrapped one per line under 60 columns, inline when they fit), the `(reco)` preselected, a "in plain words…" button that opens a text field, and a footer `1.A 2.– 3.–  [ Send ]`. Send calls the `task answer` path (lock and status checks included) and returns Home. Keys keep working: arrows/j/k, Enter, letters, Tab, Esc back, q quit. `herdr-docket questions` opens the Round screen on a TTY; without a TTY it stays the line prompt.

`grill.Round` gains `Context string`: the lines between `## Round N` and question 1. The triager role writes 2–3 lines there (current state, files, linked card).

## Tickets

1. Round context: `grill` parses Context; roles/triager.md (+ docs/examples.md copy) writes it; the line prompt prints it. Blocked by: none.
2. Pane as a bubbletea app, Home screen, keys + clicks, resize, replaces the raw-ANSI pane; 30 s queue / 2 s runs cadence kept. Blocked by: none.
3. Round screen: buttons, free text, Send through task answer; `questions` opens it on a TTY. Blocked by: 1, 2.

## Done when

On a 45-column Termux and a 120-column desktop terminal, the owner opens a question card and answers a whole round by taps/clicks only, and the answer lands as `## Answer N`.

## Verify by

`go test ./...` green, driving the bubbletea models with synthetic `tea.WindowSizeMsg`, `tea.MouseMsg` and `tea.KeyMsg`: a click on option B of question 2 then on Send posts `1.A 2.B …`; at width 45 no rendered line exceeds 45 cells and options wrap one per line; at width 120 they sit inline; a click on a `?` row on Home opens its Round.

## Out of scope

Theming; v2 of bubbletea; prototype runs for "try and show me".
<!-- SECTION:DESCRIPTION:END -->
