# Issue tracker: Backlog.md

Specs and tickets for this repo live in its Backlog.md project, `backlog/`.
Drive it with the `backlog` CLI, never by editing the task files by hand.

A **Spec** is a parent task assigned to an agent: it is the card the fleet
picks up. Each **Ticket** is a subtask of its Spec; the fleet never routes a
subtask on its own.

## When a skill says "publish to the issue tracker"

Publish the Spec first, then one Ticket per slice:

```sh
backlog task create "<spec title>" -d "<spec body>" -a <agent> -s "To Do"
backlog task create "<ticket title>" --parent <spec-id> --dep <ticket-ids> \
  --ac "<criterion>" --ac "<criterion>"
```

`--dep` names the Tickets this one is blocked by (comma-separated); leave it
out when there are none. Each acceptance criterion is one `--ac`.

## When a skill says "fetch the relevant ticket"

```sh
backlog task <id> --plain
```

## When a skill says "resolve the ticket"

Tick each acceptance criterion as it is met, then mark the task done:

```sh
backlog task edit <id> --check-ac <n>
backlog task edit <id> -s Done
```

## Triage labels

Triage roles are task labels: add one with `backlog task edit <id> --add-label <label>`.

| Role in the skills | Here                                    |
| ------------------ | --------------------------------------- |
| `needs-triage`     | label `needs-triage`                    |
| `needs-info`       | label `needs-info`                      |
| `ready-for-agent`  | status `To Do` and an assignee (`-a`)   |
| `ready-for-human`  | label `ready-for-human`                 |
| `wontfix`          | label `wontfix`, then `-s Done`         |
