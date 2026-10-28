- **`init <repo>` no longer breaks a board whose `Done` is not last.** The
  statuses it adds now go before the config's last entry instead of after it,
  so Backlog.md keeps reading `Done` (or whatever the repo ends with) as the
  completed status and `task complete` and `task archive` work again.
