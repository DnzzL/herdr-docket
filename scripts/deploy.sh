#!/bin/sh
# Builds origin/main into bin/herdr-docket, so a merged fix is the one the
# daemon runs: the daemon re-executes itself when its binary changes, but
# nothing rebuilt it after a merge. Meant for a timer; idempotent.
#
# The build happens in a worktree of its own, never in this checkout — the
# fleet's queue lives here and a human works here. The binary is swapped with
# a rename, so the daemon never sees a half-written file.
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SRC=${HERDR_DOCKET_BUILD_DIR:-$HOME/.cache/herdr-docket-build}
OUT=$ROOT/bin/herdr-docket
STAMP=$ROOT/bin/.deployed-commit

git -C "$ROOT" fetch -q origin main
want=$(git -C "$ROOT" rev-parse origin/main)
[ -f "$STAMP" ] && [ "$(cat "$STAMP")" = "$want" ] && exit 0

if [ ! -d "$SRC" ]; then
	git -C "$ROOT" worktree add -q --detach "$SRC" "$want"
fi
git -C "$SRC" checkout -q --detach "$want"

VERSION=$(sed -n 's/^version *= *"\(.*\)"/\1/p' "$SRC/herdr-plugin.toml" | head -1)
(cd "$SRC" && go build -ldflags "-X main.Version=${VERSION}" -o "$OUT.new" .)
mv "$OUT.new" "$OUT"
echo "$want" > "$STAMP"
echo "herdr-docket: deployed $(git -C "$SRC" log -1 --format='%h %s')"
