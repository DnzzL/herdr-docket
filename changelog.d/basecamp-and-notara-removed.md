- **Basecamp and Notara are no longer queues.** A `fleet.yaml` with
  `kind: basecamp` or `kind: notara` now fails to start and names the kinds
  left: `backlogmd` and `github`. Move the work into the repo's Backlog.md
  project or a GitHub Projects board, point `source:` at it, and delete the old
  block and its section of `credentials.yaml`. `auth basecamp` and
  `auth notara` are gone with them.
