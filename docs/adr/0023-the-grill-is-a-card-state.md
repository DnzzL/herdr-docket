# 0023 — The grill is a card state

An unclear idea needs a grill — questions put to the owner until the product
decisions stop being guesses — and until now a grill was a live chat session
at a laptop. The owner steers from a phone in two-minute gaps, so the session
never happened, or an agent guessed the decisions and built the wrong thing.

**The grill becomes two queue statuses, and the card stays the unit.** An
idea filed in the queue's `triage` word routes to the queue's `triager` agent
whatever assignee the card names. That run either finds the idea clear —
`Done when` and `Verify by` writable without a product decision, every
assumption under `ASSUMPTIONS:` — rewrites the card body as the spec,
publishes its tickets the way the repo's `docs/agents/issue-tracker.md` says,
assigns the author and files the card into the pickup word; or it posts the
next round as a note and moves the card to `question`. The owner answers with
`task answer <id> "<1.A 2.T …>"`, which posts `## Answer N` on the latest
round and moves the card back to `triage`. A `grill` label forces the second
path. The last round is always "Build on these assumptions? A yes / B change",
and only a yes publishes.

**The queue is the state machine, the fleet only reads it.** Both words are in
`work.Vocabulary` beside the lifecycle — a queue that names neither is simply
not grilled — and both are *open*: a triaged card owes the triager a run and a
question card owes the owner an answer. A closed card would leave the queue
and `task view` would stop showing the conversation. `work.Task` carries
`Triaging` and `Asking`, filled by the adapter the same way `Blocked` already
is, and `pick` reads exactly those two facts: `Triaging` routes to the
triager, `Asking` is never picked. This is the one place a status decides what
runs, and it is why the two facts live on the task rather than on the
display-only phase.

**Two capabilities, not a widened `Phaser`.** `Phaser.SetPhase` is display
only, written best-effort and never read back, and ADR 0012 built `TodoStarter`
on that contract. The grill's words are load-bearing — a question card nobody
picks, a triaged card that is somebody's — so `work.Griller.Move(id, phase)`
writes the three words (`Triage`, `Question`, `To Do`) and refuses when the
queue has none of them. `work.BodyWriter` exists for the same reason: the card
body is what the author's prompt carries, so the triager rewriting it as the
spec is a real write, not a note. Both are optional capabilities; a queue that
does not implement one says so by its absence and the CLI refuses rather than
swallowing the write. A composite forwards both to the queue the id names.

**A run that parks the card ended cleanly.** `reconcile` treats a card left in
`question` as the grill's own open-on-purpose — the same shape as a delivery,
so it is not closed "settled without reporting a verdict". A card left in
`triage` is *not* that: the run neither asked nor specced, so it ends Failed
and a human reopens it, rather than the next tick spinning on the same card
forever. `claim` leaves a triaged card's status alone, because that status is
what routed the run.

**Rejected.** *A live grill session* — a laptop-bound loop the owner cannot
join, and the failure this answers. *"A triage card is simply assigned to the
triager"* — an assignee is a routing key a human can edit back, and the rule
must hold against the card's own contents. *A `triage`/`question` column
inferred from any status that looks like one* — the vocabulary is already a
whitelist, and reading a project's own triage column as the fleet's would run
an agent on work the fleet was never told about. *Widening `Phaser` or
`Create`* — a load-bearing write behind a display-only contract is exactly the
confusion ADR 0012 avoided. *Closing the card on either grill ending* — the
author's run is the card's next step, and a closed card cannot be handed on.

**Status:** accepted
