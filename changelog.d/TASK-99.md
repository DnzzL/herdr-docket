- **A PR touching the pane's golden frames is verified instead of failing
  unread.** The gate now tells `gh pr diff` that escape sequences may pass
  (they only ever reach `git patch-id`, never a terminal), so a diff holding
  raw ANSI no longer kills the verify run in seconds.
