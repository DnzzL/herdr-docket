- **A GitHub-tracked repo is prepared in one command.**
  `herdr-docket init <repo> --github <owner>/<project-number>` writes a GitHub
  `docs/agents/issue-tracker.md` (Specs as issues on the board, Tickets as
  sub-issues) and a `kind: github` source whose `repo` is read from `origin`,
  without creating a Backlog.md project.
