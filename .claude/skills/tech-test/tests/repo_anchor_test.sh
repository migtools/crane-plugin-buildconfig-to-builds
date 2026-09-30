#!/usr/bin/env bash
# tech-test, tech-implement and tech-design also run from sessions started in a folder
# outside this repo. Each Bash call then starts in that other folder, so a git call that
# does not name its checkout reads the wrong repo, and a path such as
# .claude/skills/plain-words/SKILL.md points at nothing. This test fails on either.
#
# It also fails on a dollar sign followed by a digit. When a skill loads, Claude Code
# replaces that with the matching skill argument, so an awk field or a shell positional
# parameter in a snippet breaks as soon as the user passes arguments.
#
# It covers these three skills only. The other repo skills get their own test with the
# same fix.
set -uo pipefail
SKILLS=$(cd "$(dirname "$0")/../.." && pwd)
FILES="tech-test/SKILL.md tech-implement/SKILL.md tech-design/SKILL.md"
fail=0
for f in $FILES; do
  # A bare repo-reading git call, or $(pwd). Lines that choose between the session's
  # checkout and the skill's repo (they mention HERE_REPO) are the one allowed place.
  hits=$(grep -nE '(^|[^-])\bgit (rev-parse --show-toplevel|remote|worktree (list|add|remove)|for-each-ref|show-ref|fetch|reset (HEAD|--hard)|apply|ls-remote|show|diff|merge-base|archive|grep|tag)\b|\$\(pwd\)' "$SKILLS/$f" \
    | grep -vE 'git -C|HERE_REPO' || true)
  if [ -n "$hits" ]; then
    printf 'bare repo call in %s:\n%s\n' "$f" "$hits"; fail=1
  fi
  # A skill file named from the session's folder, in backticks or as a relative link.
  hits=$(grep -nE '`\.claude/skills/|\]\(\.\./' "$SKILLS/$f" || true)
  if [ -n "$hits" ]; then
    printf 'skill path read from the session folder in %s:\n%s\n' "$f" "$hits"; fail=1
  fi
  # The block that finds the repo from the skill's own folder.
  if ! grep -qF 'git -C "${CLAUDE_SKILL_DIR}" rev-parse --path-format=absolute --git-common-dir' "$SKILLS/$f"; then
    printf 'no repo block in %s\n' "$f"; fail=1
  fi
  # Prose that states the rule names it in words and never matches.
  hits=$(grep -nE '\$[0-9]' "$SKILLS/$f" || true)
  if [ -n "$hits" ]; then
    printf 'dollar sign followed by a digit in %s:\n%s\n' "$f" "$hits"; fail=1
  fi
done
[ "$fail" -eq 0 ] && echo "repo_anchor_test: ok"
exit "$fail"
