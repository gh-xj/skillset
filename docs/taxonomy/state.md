# Local State

`skills.profile.yaml` is the desired-state authority. `.skillset/` is
profile-local evidence used by guarded write commands.

## Paths

For a profile at `path/to/skills.profile.yaml`:

- `path/to/.skillset/state.yaml` - managed-entry records.
- `path/to/.skillset/events.ndjson` - append-only command events.

## Managed Entry Fields

`adopt` and `apply` record enough evidence for safe `prune` decisions:

- `agent`, `tier`, `name`
- `source`, `source_scheme`
- `target_rel`, `target_kind`
- `target_path` for compatibility with older state files
- `recorded_by`, `recorded_at`, `last_seen_at`
- `last_operation_id`
- `skill_folder_hash` for copied `github:` installs when lock metadata exists
- `symlink_target` for `local:` installs
- `install_command` for delegated `npx skills` operations
- `prune_eligible`
- `prune_safety_notes`

The state file proves what `skillset` manages. It does not override the
profile and does not make unknown filesystem entries safe to delete.

New state writes prefer `target_rel`, resolved at runtime under the configured
agent/tier root. Older `target_path` entries still load for compatibility.
When source URI syntax changes but resolves to the same desired source and
target, `prune` must treat the managed entry as still desired rather than stale.
