# Anti-Patterns

These feature shapes are out of scope without a design change.

## ANTI-1 - Replacement package manager

`skillset` should not learn GitHub package fetching, registry resolution, or
archive extraction when `npx skills` already owns upstream mechanics.

## ANTI-2 - Silent deletion during apply

Deleting unexpected skills from `apply` makes a routine reconcile command
destructive. Deletion belongs behind `prune`, dry-run first.

## ANTI-3 - Profile merging

Machine overlays, packs, sets, and layered profiles make it unclear which file
is authoritative. Keep v1 to one profile.

## ANTI-4 - Sidecar state as authority

`.skillset/` may record managed entries and events, but it must not become the
desired state or silently override the profile.

## ANTI-5 - Repo-tier mutation in v1

Repo tier is audit/check-only in v1. Applying or pruning repo-local skills
needs a separate design pass because repo roots are shared project state.

## ANTI-6 - GitHub ref pinning in v1

Pinned refs may become necessary, but adding them before the unpinned desired
state model settles complicates source parsing and update behavior too early.
