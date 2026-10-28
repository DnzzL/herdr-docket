Status: done

# A stranger's comment never reaches an agent

## Problem Statement

On a GitHub queue, every comment on an issue becomes a Note in the agent's
prompt ("Notes from previous runs"). On a public repository anyone can
comment, so anyone can steer an agent that holds write access to the repo.
The verdict of a closed issue is read from the same thread, whoever wrote it.

## Solution

The GitHub adapter keeps only comments by trusted authors: the login the
fleet's token signs in as, plus `source.github.trusted`, a list of logins in
`fleet.yaml`. Every other comment is left out of Notes and of the verdict;
Notes say how many were left out, so neither the agent nor the human believes
the thread is complete. The issue body is not filtered: a card is routed only
once someone with write access puts it on the board.

## Done when

A comment by a login outside the trusted set is absent from the task's Notes
and cannot set a closed task's verdict.

## Verify by

`go test ./internal/work/github/ -run Trusted` — red on main, green after.

## Out of scope

- Backlog.md, Basecamp, Notara: their authors are already people with access.
- Checking repository permissions through the API.
