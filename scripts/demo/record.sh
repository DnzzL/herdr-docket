#!/bin/sh
# Regenerates docs/board.gif.
#
#   nix-shell -p asciinema asciinema-agg tmux jetbrains-mono --run scripts/demo/record.sh
#
# The font is not decoration: agg rasterises the recording and a machine with
# no fonts on it has nothing to rasterise with — the nix shell above carries
# one into the store, and $FONTDIR overrides the search.
#
# The TUI runs inside tmux because tmux answers the terminal capability probes
# bubbletea sends at startup. Without a terminal that replies, the app stalls
# for a while and then replays every queued keystroke at once, collapsing the
# whole demo into a single frame.
set -eu

D="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$D/../.." && pwd)"
BIN="${BIN:-$ROOT/bin/herdr-docket}"
DEMO="${DEMO:-/tmp/herdr-docket-demo}"
SOCK=hdemo
CAST="$D/board.cast"
GIF="${GIF:-$ROOT/docs/board.gif}"
W=108
H=22

export HERDR_PLUGIN_CONFIG_DIR="$DEMO/cfg"
export HERDR_PLUGIN_STATE_DIR="$DEMO/state"
export TERM=xterm-256color

[ -x "$BIN" ] || { echo "build first: go build -o bin/herdr-docket ." >&2; exit 1; }
python3 "$D/seed.py" "$DEMO"
tmux -L "$SOCK" kill-server 2>/dev/null || true
rm -f "$CAST"

asciinema rec "$CAST" --headless --overwrite --window-size "${W}x${H}" \
	-c "tmux -L $SOCK -f $D/tmux.conf new-session -s demo -x $W -y $H '$BIN pane'" &
REC=$!

# Wait for the board to actually be on screen before touching anything.
for _ in $(seq 1 40); do
	sleep 0.5
	if tmux -L "$SOCK" capture-pane -p -t demo 2>/dev/null | grep -q Running; then break; fi
done

key() { sleep "$2"; tmux -L "$SOCK" send-keys -t demo "$1"; }

sleep 2.5
key j 1.4      # the stale run: its timer belongs to the agent that started it
key k 1.4
key v 3.4      # read a task: description, criteria, notes — the `task view` renderer
key j 1.4
key k 1.2
key Escape 1.6
key g 3.0      # the roster: busy, on what, and who is paused
key p 3.0      # pause dev — the run already in flight keeps going
key g 3.4      # back to the board: the row now reads `dev paused`
key q 1.0

wait $REC
python3 "$D/trim.py" "$CAST" "Running" "dev paused" 2.0

fontdir() {
	for name in JetBrainsMono-Regular.ttf DejaVuSansMono.ttf FiraCode-Regular.ttf; do
		hit=$(find /nix/store -maxdepth 5 -name "$name" 2>/dev/null | head -1)
		if [ -n "$hit" ]; then
			dirname "$hit"
			return 0
		fi
	done
	return 1
}
FONTDIR="${FONTDIR:-$(fontdir || true)}"
FONT=""
[ -n "$FONTDIR" ] && FONT="--font-dir $FONTDIR"

# shellcheck disable=SC2086 — $FONT is a flag pair, and the path holds no spaces.
agg --theme monokai --font-size 18 --idle-time-limit 2 $FONT "$CAST" "$GIF"
echo "wrote $GIF"
