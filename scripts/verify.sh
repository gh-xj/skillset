#!/usr/bin/env bash
# Deterministic floor for `task verify`: assert typed opt-ins declared in
# .conventions.yaml exist on disk. Free-form checks remain agent-reviewed.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if [ ! -f .conventions.yaml ]; then
  echo "verify: .conventions.yaml is missing" >&2
  exit 1
fi

if ! command -v yq >/dev/null 2>&1; then
  echo "verify: yq is required (https://github.com/mikefarah/yq)" >&2
  exit 1
fi

fail=0
fail() { echo "verify: $*" >&2; fail=1; }

while IFS= read -r doc; do
  [ -z "$doc" ] && continue
  [ -f "$doc" ] || fail "agent_docs: missing $doc"
done < <(yq -r '.agent_docs[]? // ""' .conventions.yaml)

docs_root=$(yq -r '.docs_root // ""' .conventions.yaml)
if [ -n "$docs_root" ]; then
  [ -d "$docs_root" ] || fail "docs_root: $docs_root is not a directory"
  for sub in requests planning plans implementation taxonomy; do
    [ -d "$docs_root/$sub" ] || fail "docs_root: $docs_root/$sub missing"
  done
fi

if [ "$(yq -r '.core_beliefs.enabled // .core_beliefs // false' .conventions.yaml)" = "true" ]; then
  beliefs_path=$(yq -r '.core_beliefs.path // .docs_root // "docs"' .conventions.yaml)
  for file in core-belief.md invariants.md anti-patterns.md; do
    [ -f "$beliefs_path/$file" ] || fail "core_beliefs: $beliefs_path/$file missing"
  done
fi

if [ "$(yq -r '.taskfile // false' .conventions.yaml)" = "true" ]; then
  [ -f Taskfile.yml ] || fail "taskfile: Taskfile.yml missing"
  grep -q '^[[:space:]]*verify:' Taskfile.yml || fail "taskfile: verify task not declared"
fi

while IFS= read -r root; do
  [ -z "$root" ] && continue
  [ -d "$root" ] || fail "skill_roots: $root missing"
done < <(yq -r '.skill_roots[]? // ""' .conventions.yaml)

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "verify: opt-ins ok"
