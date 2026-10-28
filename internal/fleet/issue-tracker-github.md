# Issue tracker: GitHub

Specs and tickets for this repo are GitHub issues in `{{repo}}`, worked from
project board {{project}} of `{{owner}}`. Drive them with the `gh` CLI.

A **Spec** is an issue put on the board with its `Agent` field set: that
issue is the card the fleet picks up. Each **Ticket** is a sub-issue of its
Spec; the fleet never routes a sub-issue on its own.

## When a skill says "publish to the issue tracker"

Publish the Spec first, put it on the board and name its agent:

```sh
url=$(gh issue create -R {{repo}} -t "<spec title>" -b "<spec body>")
gh project item-add {{project}} --owner {{owner}} --url "$url"
gh project item-edit {{project}} --owner {{owner}} --url "$url" --field Agent --value <agent>
```

Then one Ticket per slice, as a sub-issue of the Spec:

```sh
gh issue create -R {{repo}} --parent <spec-number> -t "<ticket title>" -b "Blocked by: #<n>, #<m>

- [ ] <criterion>
- [ ] <criterion>"
```

The `Blocked by:` line names the Tickets this one waits on; leave it out when
there are none. Each acceptance criterion is one checklist line.

## When a skill says "fetch the relevant ticket"

```sh
gh issue view <n> -R {{repo}} --comments
```

## When a skill says "resolve the ticket"

Tick each checklist box in the body as it is met, then close the issue:

```sh
gh issue edit <n> -R {{repo}} --body-file <body-with-boxes-ticked>
gh issue close <n> -R {{repo}}
```

## Triage labels

Triage roles are issue labels: add one with `gh issue edit <n> -R {{repo}} --add-label <label>`.

| Role in the skills | Here                                          |
| ------------------ | --------------------------------------------- |
| `needs-triage`     | label `needs-triage`                          |
| `needs-info`       | label `needs-info`                            |
| `ready-for-agent`  | on the board with its `Agent` field set       |
| `ready-for-human`  | label `ready-for-human`                       |
| `wontfix`          | label `wontfix`, then `gh issue close <n> --reason "not planned"` |
