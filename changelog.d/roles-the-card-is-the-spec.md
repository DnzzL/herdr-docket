- **The dev and reviewer roles read the card as the spec.** A card's body is
  the spec and its subtasks are the tickets, fetched the way the repo's
  `docs/agents/issue-tracker.md` says; a card with no subtasks is one ticket's
  worth of work. Roles no longer look for a `Spec:` line or a `.scratch` path,
  so an existing `roles/dev.md` or `roles/reviewer.md` written by an earlier
  `init` keeps the old wording until you delete it and re-run `init`.
