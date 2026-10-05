#!/bin/sh
# Re-records docs/demo.gif: the real CLI on a throwaway fleet — its own HOME,
# config and state dirs, so nothing lands in your history.jsonl. Needs nix
# (asciinema, agg, a font). Run from the repo root after `go build -o bin/herdr-docket .`.
set -eu
ROOT=$(pwd)
T=$(mktemp -d)
mkdir -p "$T/home" "$T/cfg" "$T/state"
export HOME="$T/home" HERDR_PLUGIN_CONFIG_DIR="$T/cfg" HERDR_PLUGIN_STATE_DIR="$T/state"
"$ROOT/bin/herdr-docket" init >/dev/null

cat > "$T/play.sh" <<PLAY
export PATH="$ROOT/bin:\$PATH"
type_run() { printf '\033[32m\$\033[0m '; printf '%s' "\$1" | while IFS= read -r -n1 c || [ -n "\$c" ]; do printf '%s' "\$c"; sleep 0.025; done; printf '\n'; sleep 0.3; sh -c "\$1"; sleep "\${2:-1.6}"; }
clear
type_run 'herdr-docket task create "Search ignores accents" -a example -d "Users who type « creme » find none of the « crème » products." --ac "a search test for « creme » is red on main, green after"'
type_run 'herdr-docket list'
type_run 'herdr-docket task view TASK-1' 2.6
type_run 'herdr-docket pause | head -1'
type_run 'herdr-docket runs' 2.2
PLAY

FONT=$(nix build --no-link --print-out-paths nixpkgs#dejavu_fonts)/share/fonts
nix shell nixpkgs#asciinema nixpkgs#asciinema-agg nixpkgs#bash -c sh -c "
  asciinema rec -q --overwrite --headless --window-size 100x22 -c 'bash $T/play.sh' $T/demo.cast &&
  agg --font-dir $FONT --font-family 'DejaVu Sans Mono' --font-size 16 --theme monokai --last-frame-duration 3 $T/demo.cast $ROOT/docs/demo.gif"
rm -rf "$T"
