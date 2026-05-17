# Invariants

These facts must remain true. Cite by ID in review.

## INV-1 - `skills.profile.yaml` is the desired-state authority

Commands may derive plans, diagnostics, state records, and events from the
profile, but no derived artifact replaces it as the source of intent.

## INV-2 - `tier`, `owner`, and `source` stay distinct

`tier` is activation/install scope: `system`, `user`, or `repo`.
`owner` is maintenance/source ownership: `upstream`, `first_party`, `private`,
`repo`, or `system`. `source` is the resolvable origin URI.

## INV-3 - v1 source schemes are exactly three forms

- `github:<owner>/<repo>//<skill-dir>`
- `local:<root>//<skill-dir>`
- `system:<agent>/<skill>`

Adding another source form is a profile schema change.
`local:<root>//...` may resolve through a profile-local named `roots:` entry
before falling back to raw path behavior; this does not add a fourth source
scheme.

## INV-4 - Read-only commands do not touch live skill roots

`validate`, `normalize`, `list`, `check`, `diff`, and `doctor` may read files,
but they must not install, delete, rewrite, or relink skills.

## INV-5 - `apply` is non-destructive

When write commands exist, `apply` may create missing desired entries or
refresh desired entries through their source mechanism. It must not delete
unexpected entries.

`apply` must also refuse to replace existing targets. Wrong-kind and
wrong-target states are diagnostics, not repair instructions.

## INV-6 - `prune` is the only delete path

`prune` defaults to dry-run and may delete only entries previously recorded as
`skillset`-managed in profile-local state.

`prune --apply` must not delete unknown or unmanaged filesystem entries.

## INV-7 - Every JSON command emits `schema_version: "v1"`

The root JSON object is a CLI output contract. Profile files still use numeric
`schema_version: 1`.

## INV-8 - `os.Exit` lives only in `cmd/skillset/main.go`

Every other path returns an integer or an error so the CLI can be tested
end-to-end through in-memory writers.

## INV-9 - v1 has one profile, not overlays

The CLI may accept a profile path. It must not merge machine overlays, packs,
sets, or multiple profiles in v1.

## INV-10 - `.skillset/` is evidence, not intent

Profile-local `.skillset/state.yaml` and `.skillset/events.ndjson` may record
managed entries and command events. They must not override
`skills.profile.yaml` or make unknown filesystem entries safe to delete.
