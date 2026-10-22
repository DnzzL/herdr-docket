# 01: Plain init writes the two pipeline roles

**What to build:** a user running plain `herdr-docket init` gets the `dev` and `reviewer` roles in their fleet's roles dir, beside the example role, without installing any loop. A role file that already exists is kept and the skip is reported. The shipped roles stay pinned verbatim to `docs/examples.md`. The factory loop still installs as before at this stage; it only stops owning the roles.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] `init` into an empty fleet dir writes both roles and says so
- [ ] `init` over an existing `dev` or `reviewer` role leaves it byte-identical and reports the skip
- [ ] A drift test fails if either shipped role is not verbatim in `docs/examples.md`
- [ ] `init --factory` still works, without writing the roles a second time

**Done when:** the two roles ship with plain `init` and are tested through it.

**Verify by:** `go test ./...` green; `herdr-docket init` into a fresh dir lists `roles/dev.md` and `roles/reviewer.md`; a second run reports both skipped.
