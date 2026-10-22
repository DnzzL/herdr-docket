# 02: The author role speaks the glossary

**What to build:** an author reading the `dev` role shipped by `init` meets the glossary's words: a card with no `Spec:` line is worked as one ticket's worth of work, with no spec — not "is one ticket" (a ticket lives only in `.scratch`). The same line changes in the role's verbatim copy in the examples manual. Nothing else in the role changes.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] The shipped `dev` role no longer calls a no-spec card a ticket, and says it is worked as one ticket's worth of work
- [ ] The examples manual carries the same text (drift test green)
- [ ] The role's diff is that line only (rewrapping the paragraph it sits in is fine)

**Done when:** the role an author reads first agrees with CONTEXT.md on card and ticket.

**Verify by:** `go test ./...` green; `grep -n "card is one ticket" internal/fleet/roles/dev.md docs/examples.md` finds nothing.
