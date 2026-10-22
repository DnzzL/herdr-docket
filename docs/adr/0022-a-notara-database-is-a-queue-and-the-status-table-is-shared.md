# 0022 — A Notara database is a queue, and the status table is shared

Date: 2026-10-14 · Status: accepted

## The problem

Notara's own backlog moves from `backlog/` into a Notara database (Notara
ADR-012: Notara is a dumb queue backend; claims, routing and the meaning of a
status stay with the runner). The fleet needs to pick, work and close those
cards without an agent ever calling Notara itself.

## The decision

1. **A fourth adapter, `kind: notara`**, behind the unchanged `work.Source`
   port, over Notara's REST `/api/v1`. A card is a record; its `Status` and
   `Agent` selects carry the lifecycle and the routing key; `Labels` and
   `Priority` feed the gate and the pick; the page's to-dos are the criteria;
   the record's comment thread is the notes. It takes `Phaser`, `Assigner`,
   `TodoStarter`, `CriterionWriter` (through Notara's append endpoint, added
   for this) and `BaseBrancher` (the source's `worktree_base:` only — a hosted
   queue has no repo to derive one from).

2. **The status table moves from `backlogmd` to `work`.** A Notara database
   speaks its own words exactly like a Backlog.md project does, so both read
   the same `statuses:` block through one `work.Vocabulary` — one whitelist of
   what is open, one reading of a word back into a verdict. `backlogmd` keeps
   its default lifecycle (the words `fleet init` lays down); a Notara source
   has no default and must name its words.

3. **Plain `Create` lands in the `todo` word too.** Backlog.md files a task
   into its project's `default_status`; a Notara select has no default, and a
   card with no status is invisible to the fleet forever. `CreateTodo` is the
   same write, reported as started.

## Rejected

- **Driving Notara through `backlogmd` and an export**: two sources of truth,
  and the humans' board would lag the agents' by a sync.
- **A second copy of the status table inside the notara adapter**: two
  whitelists of "open" that drift, and the one that drifts runs an agent on a
  triage column.

## Known gaps

- `fragile` — a card's body is its whole page as markdown, so criteria show
  twice in a prompt: in the body, and as the numbered list Get fills.
- `not done` — one API key per fleet: every agent's note is signed with the
  key's name, not the agent's.
- `fragile` — `List` reads the whole database each tick (163 cards: 31 ms,
  53 kB on localhost); no paging until Notara has one.
