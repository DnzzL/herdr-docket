# 01: Every ADR has its own number

**What to build:** a contributor whose ADR collides with another's number sees the test suite fail, naming the duplicated number and both files. The current collision is resolved: the Notara ADR, the later of the two 0020s, becomes 0022 (file name and title line; content unchanged), and every reference to it follows — the Go comments in the queue vocabulary and in the Notara and Backlog.md adapters, and the queues manual's link. The lock-file ADR keeps 0020, and everything citing it stays.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [ ] A test in the root package reads the ADR directory and fails listing each number used by more than one file, with the file names; it is red on main before the rename (commit it red first)
- [ ] The Notara ADR is 0022 in its file name and title; no other ADR content changes
- [ ] Every comment and doc link that meant the Notara decision names 0022; references to the lock-file ADR are untouched
- [ ] The adapters gain no behaviour: comment edits only

**Done when:** no two ADRs share a number and a new collision turns the suite red.

**Verify by:** `go test ./...` green; the new test fails on main's tree before the rename; `ls docs/adr | cut -c1-4 | sort | uniq -d` prints nothing; `grep -rn "ADR-0020\|0020-a-notara" internal docs` finds no Notara reference.
