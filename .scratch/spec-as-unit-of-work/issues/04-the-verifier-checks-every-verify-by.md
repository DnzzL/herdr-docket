# 04: The verifier checks every Verify by

**What to build:** the `reviewer` role shipped by `init` keeps its content (the checkout is not yours, evidence ladder, what fails a PR, the pinned verdict verb) and gains one rule: when the card points at a spec, read the spec and every ticket before the diff, and check the spec's and each ticket's `Verify by` against the one pull request. One ticket left unproven is a FAIL. The verdict verb and its pinning to the patch-id do not change.

**Blocked by:** 01

**Status:** resolved

- [x] The role tells the verifier to read the spec and every ticket before the diff
- [x] The role makes one unproven `Verify by` a FAIL
- [x] Nothing else in the role changes
- [x] The role is verbatim in `docs/examples.md` (drift test green)

**Done when:** a verifier reading this role judges a spec-sized PR against every ticket, with the same verdict seam.

**Verify by:** `go test ./...` green; the diff of the role is the added rule only.
