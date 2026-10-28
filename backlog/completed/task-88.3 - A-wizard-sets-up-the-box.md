---
id: TASK-88.3
title: A wizard sets up the box
status: Done
assignee: []
created_date: '2026-10-28 13:30'
updated_date: '2026-10-28 13:34'
labels: []
dependencies: []
parent_task_id: TASK-88
ordinal: 97000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
`scripts/box-setup`: an interactive bash wizard (use the `wizard` skill) for a fresh VPS: Tailscale up, Herdr install + server, `gh auth login`, the agent CLIs' logins, `herdr plugin install`, `herdr-docket init <repo>` per repo with `triager`/`verifier` set, one `intake` workspace per repo, `max_runs` sized from RAM (1.5 GB per run). Each step checks before acting and can be re-run. Linked from README.
<!-- SECTION:DESCRIPTION:END -->
