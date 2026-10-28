- **Tasks can carry acceptance criteria at creation.** `herdr-docket task
  create` now takes repeatable `--ac "<criterion>"` flags, so the bar lives on
  the task instead of in prose or in commands written past the fleet CLI.
  Backlog.md and GitHub store criteria.
  The write follows the pickup-status landing (ADR-0012): one create files
  the task where the daemon can claim it and puts the bar on it right after —
  a queue that cannot carry criteria says so instead of dropping them.
