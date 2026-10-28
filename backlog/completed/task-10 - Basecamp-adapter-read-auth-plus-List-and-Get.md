---
id: TASK-10
title: 'Basecamp adapter (read): auth plus List and Get'
status: Done
assignee: []
created_date: '2026-09-10 20:10'
updated_date: '2026-09-10 20:33'
labels: []
dependencies:
  - TASK-9
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add internal/work/basecamp. `herdr-docket auth basecamp` runs the OAuth code flow (a single registered Launchpad client id), stores the access and refresh tokens in credentials.yaml (0600, never in fleet.yaml) and refreshes automatically (a Basecamp access token lives two weeks). List reads one to-do list per agent; Get returns one to-do with its description and comments; rich text is converted HTML to markdown; polling honours Retry-After on 429. Webhooks are out: a local fleet has no public endpoint.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 herdr-docket auth basecamp stores tokens in credentials.yaml and refreshes an expired token without user action
- [ ] #2 List maps each configured to-do list to its agent and returns Items with Open set from the to-do's completed flag
- [ ] #3 Get returns title, markdown body and criteria; HTML is converted, not passed through raw
- [ ] #4 429 responses are retried after Retry-After; the adapter never spins
- [ ] #5 go test ./... green (HTTP via a fake server)
<!-- AC:END -->
