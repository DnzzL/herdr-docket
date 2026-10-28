# 04: The roles speak "the card is the spec"

**What to build:** the `dev` and `reviewer` roles stop looking for a `Spec:` line and a `.scratch` path. The card *is* the spec: its body is the spec, its subtasks are the tickets, read with the tracker's own fetch (`docs/agents/issue-tracker.md`). A card with no subtasks is worked as one ticket's worth of work with `/tdd`. `docs/examples.md` carries the same text (drift test). README, `docs/how-it-works.md`, `docs/working-with-a-factory.md` and `CONTEXT.md` speak the same way; ADR 0021 notes it is amended by 0023.

**Blocked by:** 02, 03

**Status:** resolved

- [x] `roles/dev.md` and `roles/reviewer.md` name no `.scratch` path and no `Spec:` line
- [x] `docs/examples.md` matches (drift test green)
- [x] README, how-it-works, working-with-a-factory, CONTEXT.md updated; no `.scratch` left in `docs/` except ADRs
- [x] ADR 0021 carries "Amended by 0023"

**Done when:** an agent reading only its role and the repo's `issue-tracker.md` finds the spec and its tickets.

**Verify by:** `go test ./...` green; `grep -rn "\.scratch\|Spec:" internal/fleet/roles docs README.md CONTEXT.md | grep -v docs/adr` empty.
