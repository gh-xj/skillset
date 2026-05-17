---
name: skillset-maintainer
description: Use when maintaining the skillset CLI repo: profile schema, tier/owner/source semantics, read-only planner commands, write-command safety, or repo docs.
---

# Skillset Maintainer

Maintain `skillset` as a small Go + kong CLI whose durable contract is
`skills.profile.yaml`.

## Before Changing Schema Or Write Behavior

Read:

- `docs/core-belief.md`
- `docs/invariants.md`
- `docs/anti-patterns.md`
- `AGENTS.md`

## Current Surface

Implemented commands:

- profile schema model
- profile loader
- `validate`
- `normalize`
- `list`
- `roots`
- `check`
- `diff`
- `doctor`
- `discover`
- `adopt`
- `apply`
- `prune`
- `managed`
- `version`

Do not add writes to live skill roots as part of planner, discovery, or adopt
work. `adopt --apply` may write only profile-local `.skillset/` state.
`apply --apply` may create missing desired user-tier entries. `prune --apply`
may delete only entries recorded as `skillset`-managed and no longer desired.

## Command Rules

- Keep one command file per leaf under `internal/skillsetcli/`.
- Every leaf command implements `Run(*CLI) error`.
- Every `--json` response has root `schema_version: "v1"`.
- Prefer profile-domain code in `internal/profile/`; keep CLI files thin.

## Safety Rules

- `apply` is non-destructive.
- `prune` is the only delete path and defaults to dry-run.
- `prune` may delete only entries recorded as `skillset`-managed.
- `system` tier is check/ignore-only.
- `repo` tier is audit/check-only in v1.

## Verification

Run:

```sh
task verify
```
