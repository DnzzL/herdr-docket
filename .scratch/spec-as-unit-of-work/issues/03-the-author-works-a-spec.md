# 03: The author works a spec

**What to build:** the `dev` role shipped by `init` tells the author how to work a card. If the card body has a `Spec:` line, it reads the `implement-spec` skill's `SKILL.md` with its file tool (never the Skill tool, since that skill is user-invoked only) and follows it on the spec. Otherwise it treats the card as one ticket and uses `/tdd`. It uses subagents when its runtime has them and works alone when it does not. It delivers with `task done <id> --pr <url> --note`. On a rework, the verifier's note is its brief on the same branch, and it fixes only what the note names. It blocks with the one question that would settle a doubt, and never merges. Coding method is left to the engineering skills and the repo's AGENTS.md: the role does not restate them.

**Blocked by:** 01

**Status:** resolved

- [x] The role covers the spec and no-spec cases, the read-not-invoke rule and where the skill file usually lives
- [x] The role keeps delivery, rework-scope, block-with-one-question and never-merge
- [x] The role is about ten lines and restates no rule the engineering skills already carry
- [x] The role is verbatim in `docs/examples.md` (drift test green)

**Done when:** an author reading only this role and a card knows which procedure to follow and how to hand back.

**Verify by:** `go test ./...` green; read the role against the five rules above, one line each.
