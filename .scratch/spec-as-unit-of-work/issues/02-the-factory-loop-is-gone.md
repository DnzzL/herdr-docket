# 02: The factory loop is gone

**What to build:** `herdr-docket init --factory` no longer exists: it exits non-zero with a message that the flag was removed and that plain `init` writes the roles. The factory loop goes with it: its personas (intake, stall, lookback, and its copies of dev and reviewer), its four schedules (intake, stall-sweep, lookback, merge-digest), its package and its doc. Holds still raise the herdr notification and the `merge-needed` label, which the runner owns.

**Blocked by:** 01

**Status:** resolved

- [x] `init --factory` exits non-zero and its message names the removal and points at plain `init`
- [x] No persona or schedule of the loop ships in the binary any more
- [x] `docs/factory.md` and the loop's sections of `docs/examples.md` are gone; no doc links to them
- [x] The usage text no longer mentions `--factory`
- [x] A held PR still gets its notification and `merge-needed` label (existing tests stay green)

**Done when:** the loop is gone from binary, usage and docs, and the one flag that installed it fails loudly.

**Verify by:** `go test ./...` green; `herdr-docket init --factory; echo $?` prints the removal message and a non-zero status; `grep -rn -- "--factory\|factory.md" docs README.md main.go` finds nothing.
