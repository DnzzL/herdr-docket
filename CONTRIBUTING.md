# Contributing

The scope is deliberately narrow: **herdr-docket is the smallest version of
paperclip.ing that still works — a queue, named agents, and a daemon that
routes. The fuss is the feature you leave out.**

An org chart, a goals tree, retries, task dependencies, auto-reassignment,
a workflow engine: each is a thing a bigger product does, and each belongs
somewhere else. A PR that grows the plugin past that line is likely to be
declined on scope rather than on quality — open an issue first and we'll work
out whether it fits before you write it.

## What is most wanted

The maintainer runs Backlog.md on Claude Code every day. Everything past that
pair is where a report is worth more than a patch.

- **A third backend.** The contract is `work.Source`; pass `worktest.Run` and
  nothing above the adapter moves. Basecamp is the second backend and the
  newest — reports of where its phases or verdict readback lie are as welcome as
  code.
- **Agent kinds tested in the wild.** The plugin claims anything
  `herdr agent start` supports — `pi`, `codex`, `gemini`, `opencode`, `cursor`.
  Only `claude` runs daily. What breaks on the others is worth a paragraph.

Deliberately not on the list: retries, task dependencies, scheduling policies,
an org chart or a goals tree. Those are the fuss.

## Working on it

```bash
go build -o bin/herdr-docket . && go test ./...
go vet ./... && gofmt -l .          # both must be clean; CI enforces them
herdr plugin link .                 # run your checkout as the installed plugin
```

`herdr plugin link` replaces any GitHub install of the plugin, and the daemon
re-executes itself when the binary changes, so a rebuild is enough to test.

Config lives in `herdr plugin config-dir dnzzl.herdr-docket`, run history in
`~/.local/state/herdr/plugins/dnzzl.herdr-docket`. Both survive uninstalls;
delete them by hand for a clean slate.

The README's board image is recorded, not drawn: `scripts/demo/` seeds a
throwaway fleet of two queues and presses the keys through tmux, because a TUI
that has no terminal answering its startup probes collapses into one frame.

```bash
nix-shell -p asciinema asciinema-agg tmux jetbrains-mono \
  --run scripts/demo/record.sh    # writes docs/board.gif
```

## House style

- **Comments explain why, not what.** The code says what it does.
- **The queue is the source of truth; the fleet keeps no copy.** A change that
  mirrors queue state into the plugin — a cache, a local snapshot of the board —
  is suspect by default.
- **The fleet reports, it never invents.** If the daemon would do something the
  queue doesn't say — retry, reassign, default a value — it reports instead and
  lets a human decide.
- **The core owns the words a human reads; the adapter owns what only it
  understands.** A phase is shown, so the core names it; a priority is only
  ordered, so the backend does. See `docs/adr/0001`.
- **A backend passes `worktest.Run` or it isn't a backend.** Nothing above an
  adapter learns which one answered.
- Every user-visible change gets a `CHANGELOG.md` entry describing what it means
  for someone using the plugin, not what was refactored.
