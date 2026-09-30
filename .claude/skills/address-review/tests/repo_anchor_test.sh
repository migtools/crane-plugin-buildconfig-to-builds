#!/usr/bin/env bash
# The repo skills also run from sessions started in a folder outside this repo.
# Each Bash call then starts in that other folder, so a git
# call that does not name its checkout reads the wrong repo. This test fails on any
# such call in the skills that act on the repo.
#
# It also fails on a dollar sign followed by a digit. When a skill loads, Claude Code
# replaces that with the matching skill argument, so an awk field or a shell positional
# parameter in a snippet breaks as soon as the user passes arguments.
set -uo pipefail
SKILLS=$(cd "$(dirname "$0")/../.." && pwd)
FILES="deep-review/src/header.md address-review/SKILL.md babysit-prs/SKILL.md create-pr/SKILL.md edit-pr/SKILL.md tech-document/SKILL.md tech-review/SKILL.md"
fail=0
for f in $FILES; do
  # A bare repo-reading git call, or $(pwd). Lines that choose between the session's
  # checkout and the skill's repo (they mention HERE_REPO) are the one allowed place.
  hits=$(grep -nE '(^|[^-])\bgit (rev-parse --show-toplevel|remote|worktree (list|add|remove)|for-each-ref|show-ref|fetch|reset (HEAD|--hard)|apply|ls-remote)\b|\$\(pwd\)' "$SKILLS/$f" \
    | grep -vE 'git -C|HERE_REPO' || true)
  if [ -n "$hits" ]; then
    printf 'bare repo call in %s:\n%s\n' "$f" "$hits"; fail=1
  fi
done
# Every skill is loaded the same way, so this check covers every SKILL.md, plus
# deep-review's source header that bin/sync builds its SKILL.md from.
for f in deep-review/src/header.md $(cd "$SKILLS" && printf "%s\n" */SKILL.md); do
  # Prose that states the rule names it in words and never matches.
  hits=$(grep -nE '\$[0-9]' "$SKILLS/$f" || true)
  if [ -n "$hits" ]; then
    printf 'dollar sign followed by a digit in %s:\n%s\n' "$f" "$hits"; fail=1
  fi
done
[ "$fail" -eq 0 ] && echo "repo_anchor_test: ok"
exit "$fail"
