#!/usr/bin/env bash
# Mint the sandbox's git credential and prove it works over https.
#
# A sandboxed run has no ~/.ssh, so its only git credential is one fine-grained
# personal access token scoped to a single repository. GitHub's API cannot mint
# fine-grained tokens (404) and `gh auth login` does not run inside the sandbox,
# so minting is a human step in the GitHub UI — this script is the walkthrough
# and the proof.
#
# It stores nothing: the token is typed at the terminal, lives only in this
# process's environment, and the one file written (an askpass helper) echoes
# that environment and is deleted on exit.
set -euo pipefail

REPO="DnzzL/DishNow-v2"

say() { printf '%s\n' "$*"; }
step() { printf '\n== %s ==\n' "$*"; }

step "What you are about to mint"
say "One fine-grained personal access token, with exactly these settings:"
say "  Resource owner:         DnzzL"
say "  Repository access:      Only select repositories -> $REPO"
say "  Expiration:             90 days"
say "  Repository permissions: Contents          Read and write"
say "                          Pull requests     Read and write"
say "                          Metadata          Read-only (GitHub grants it to every fine-grained token)"
say ""
say "No ssh anywhere inside: the sandbox mounts no ~/.ssh, and this token is"
say "the credential a sandboxed run pushes with. The host's gh oauth token"
say "(gho_) is refused by git-over-https with 401 'Invalid username or token'."

step "Mint it in the GitHub UI"
say "Open:  https://github.com/settings/personal-access-tokens/new"
say "Set the form to the settings above, click Generate token, copy it."
printf 'Press enter when you have copied the token... '
read -r _

step "Paste it (this script stores nothing)"
if [ ! -t 0 ]; then
  say "error: run this on a terminal; the token must be typed, never piped or passed as an argument."
  exit 1
fi
printf 'Token (input hidden): '
IFS= read -rs PAT
printf '\n'
case "$PAT" in
  github_pat_*) ;;
  *)
    say "error: that is not a fine-grained token (fine-grained tokens start with github_pat_)."
    say "A gho_/ghp_ token is exactly what git-over-https already refuses — mint the fine-grained one."
    exit 1
    ;;
esac

# Everything below proves the token by using it; nothing is written to disk
# except an askpass helper that echoes the environment variable and dies with
# this process.
WORK=$(mktemp -d)
PUSHED=0
BRANCH="fleet/wizard-verify-$(date +%s)-$$"
cleanup() {
  status=$?
  if [ "$PUSHED" = 1 ]; then
    say ""
    say "Cleaning up the throwaway branch and pull request..."
    GH_TOKEN="$SANDBOX_PAT" gh pr close "$BRANCH" --repo "$REPO" \
      --comment "Verification run of scripts/sandbox-git-token.sh — nothing to review." \
      >/dev/null 2>&1 || true
    git -c credential.helper= push --delete \
      "https://x-access-token@github.com/$REPO.git" "refs/heads/$BRANCH" \
      >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK"
  exit "$status"
}
trap cleanup EXIT
printf '#!/bin/sh\nprintf "%%s" "$SANDBOX_PAT"\n' > "$WORK/askpass.sh"
chmod +x "$WORK/askpass.sh"
export SANDBOX_PAT="$PAT" GIT_ASKPASS="$WORK/askpass.sh" GIT_TERMINAL_PROMPT=0

step "Prove git-over-https accepts it"
# credential.helper= drops any helper a host has configured, so the answer
# below comes from this token alone and cannot be a cached success.
if git -c credential.helper= ls-remote \
  "https://x-access-token@github.com/$REPO.git" HEAD; then
  say "ok: git over https answered for $REPO."
else
  say "error: git over https refused the token. In the GitHub form, check the"
  say "resource owner, the repository selection ($REPO) and the permissions."
  exit 1
fi

step "Prove a push and a pull request"
git rev-parse --verify HEAD >/dev/null 2>&1 || {
  say "error: run this from inside a checkout of $REPO — a sandboxed pane's worktree."
  exit 1
}
git var GIT_COMMITTER_IDENT >/dev/null 2>&1 || {
  say "error: git has no committer identity here; set user.name/user.email first."
  exit 1
}
COMMIT=$(printf 'sandbox git credential verification\n' |
  git commit-tree "$(git rev-parse 'HEAD^{tree}')" -p HEAD)
git -c credential.helper= push \
  "https://x-access-token@github.com/$REPO.git" "$COMMIT:refs/heads/$BRANCH" \
  && PUSHED=1
say "ok: pushed the throwaway branch $BRANCH."
PR_URL=$(GH_TOKEN="$SANDBOX_PAT" gh pr create --repo "$REPO" --base main \
  --head "$BRANCH" \
  --title "sandbox credential verification" \
  --body "Opened and closed automatically by scripts/sandbox-git-token.sh to prove the fine-grained token can open a pull request. Nothing to review.")
say "ok: gh pr create answered: $PR_URL"

step "Where the token must live now"
say "The sandbox reads it as ordinary environment: export GITHUB_TOKEN and"
say "GH_TOKEN in the shell that starts herdr's panes (then restart herdr)."
say "Run this script from inside a sandboxed pane so the proof covers the"
say "environment that will actually use the token."
say "The sandbox's flake references \"\$GITHUB_TOKEN\" and never a value, so"
say "the token stays out of every file. This script kept its copy only in its"
say "own environment, and the cleanup above closed the pull request and"
say "deleted the throwaway branch."
