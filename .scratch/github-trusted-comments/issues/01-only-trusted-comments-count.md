# 01: Only trusted comments become Notes or a verdict

**What to build:** `Get` on a GitHub queue keeps the comments written by the signed-in login or a login in `source.github.trusted`, and says in Notes how many others it left out.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] A comment by an untrusted login is absent from Notes
- [x] Notes say how many comments were left out
- [x] A login listed in `trusted` is kept, alongside the signed-in login
- [x] An untrusted `Verdict:` comment on a closed issue does not set the verdict

**Done when:** a comment by a login outside the trusted set is absent from the task's Notes and cannot set a closed task's verdict.

**Verify by:** `go test ./internal/work/github/ -run Trusted` — red on main, green after.
