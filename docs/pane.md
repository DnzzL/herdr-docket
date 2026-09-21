# The board pane

[Back to the README](../README.md)

The board is `herdr-docket pane` in a terminal, or an overlay inside Herdr.
What the rows mean, what the keys do, and what the pane deliberately refuses is
in the [README](../README.md#the-board). This page is about getting it on
screen, and about the one failure that is hard to see because it is silent.

## Binding it to a chord

The chord is ordinary Herdr configuration — `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+f"
type = "shell"
command = "herdr plugin pane open --plugin dnzzl.herdr-docket --entrypoint board --placement overlay"
```

Then make Herdr read it:

```bash
herdr config check            # config: ok
herdr server reload-config    # applied without restarting the server
```

The id is the manifest's — `dnzzl.herdr-docket`, not `dnzzl.docket`: a chord is
a plain shell command, so a name Herdr does not know answers `plugin_not_found`
and the key does nothing at all. `--placement` takes `overlay`, `split`, `tab`
or `zoomed`.

## When the key does nothing

A linked plugin is registered but not necessarily *built*, and it can be
*disabled*. Because the chord is a plain shell command, it fails silently and
`herdr config check` still reports `ok`. Ask for the pane directly instead —
this prints the error the keybinding swallows:

```bash
herdr plugin list                      # enabled? and does bin/ actually exist?
herdr plugin pane open --plugin dnzzl.herdr-docket --entrypoint board --placement overlay
herdr plugin pane close PANE_ID        # closes the board again, given the id open printed
```

What each answer means:

- `plugin_not_found` → the plugin is not installed or linked. On a checkout,
  build it and link it (`go build -o bin/herdr-docket . && herdr plugin link .`),
  because the manifest's build step fetches the *released* binary, not the tree
  you are in.
- `plugin_disabled` → `herdr plugin enable dnzzl.herdr-docket`.
- `... because it does not exist` → run the manifest's build step
  (`sh scripts/install.sh`).

All of these take effect without restarting the Herdr server, so no running
agent loses its pane.

## See also

- [The board](../README.md#the-board) — the keys, the rows, the GIF.
- [ADR 0008](adr/0008-the-board-is-a-triage-surface.md) — what the pane shows
  and what it deliberately refuses.
- [Writing an agent](agents.md) — `agent pause`, which the pane's `p` writes.
