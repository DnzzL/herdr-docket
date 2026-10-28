- **`go test ./...` no longer opens panes in your Herdr session.** The herdr
  contract suite execs the real binary, and it used to do so in the session you
  were working in — every test run opened and closed `docket contract`
  workspaces under your cursor. It now runs in a throwaway named session that
  TestMain starts and deletes, so your session is untouched; on a machine
  without herdr the suite still skips.
