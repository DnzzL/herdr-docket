# 01: Basecamp and Notara are gone

**What to build:** a fleet configured with `kind: basecamp` or `kind: notara` fails to load with an error naming the two kinds left (`backlogmd`, `github`). Both adapters, their tests, their `auth` sub-commands, their config blocks and their docs are removed. `internal/credentials` stays (GitHub uses it). ADR 0022 is marked superseded by 0023. Tests that only used "notara"/"basecamp" as a source *name* are renamed, not deleted.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] `internal/work/basecamp` and `internal/work/notara` are deleted
- [ ] `kind: notara` fails `LoadSettings` naming `backlogmd` and `github`
- [ ] `main.go` usage and `auth` help name only `github`
- [ ] `docs/queues.md`, `docs/commands.md`, `README.md` no longer mention them; ADR 0022 status says superseded by 0023
- [ ] A changelog file says what a Basecamp or Notara user must do

**Done when:** no Go file names basecamp or notara and the suite is green.

**Verify by:** `go test ./...` green; `grep -rni "basecamp\|notara" --include='*.go' .` empty.
