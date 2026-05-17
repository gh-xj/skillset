# AGENTS.md - skillset CLI

Source of the `skillset` CLI, a Go command that manages `skills.profile.yaml`
as the authoritative desired state for agent skills. The CLI is a policy and
reconcile layer over local symlinks plus `npx skills`; it is not a
replacement package manager.

## Philosophy

Read before changing profile schema, tier semantics, ownership semantics,
write commands, or repo-wide workflow:

- [docs/core-belief.md](docs/core-belief.md) - what `skillset` is for.
- [docs/invariants.md](docs/invariants.md) - verifiable hard constraints.
- [docs/anti-patterns.md](docs/anti-patterns.md) - feature shapes this repo
  rejects without an explicit design change.

## Architecture

- `cmd/skillset/main.go` - entry point. It only calls
  `os.Exit(skillsetcli.Execute(os.Args[1:]))`.
- `internal/skillsetcli/` - kong command surface, global flags, output
  helpers, and one file per command.
- `internal/profile/` - durable `skills.profile.yaml` schema, source URI
  parsing, normalization, and validation.
- `internal/discover/` - read-only root scanning and suggested profile
  generation from symlinks plus `.skill-lock.json` metadata.
- `internal/state/` - profile-local `.skillset/` state/event schema types for
  `adopt`, `apply`, `prune`, and `managed`.
- `skills/skillset-operator/` - public installable skill for agents operating
  `skillset` in user repos.
- `internal/cliruntime/` - testable kong runner with captured exits.
- `internal/appctx/` - exit-code policy: success `0`, runtime error `1`,
  usage error `2`.
- `internal/io/`, `internal/log/` - JSON and logging primitives.

The implemented write commands are guarded: `adopt`/`apply`/`prune` default
to dry-run. Read-only commands are `validate`, `normalize`, `list`, `roots`,
`check`, `diff`, `doctor`, and `discover`. `adopt --apply` writes only
profile-local `.skillset/` state; `apply --apply` creates missing desired
user-tier skills; `prune --apply` deletes only entries recorded as
`skillset`-managed. `managed` reads state.

## Commands

```sh
task fmt          # gofmt -w .
task fmt:check    # fail if gofmt would change files
task lint         # go vet ./...
task test         # go test ./...
task build        # binary into bin/skillset
task smoke        # black-box JSON checks against the built binary
task ci           # lint + test + build + smoke
task verify       # ci plus convention opt-in checks
```

Development loop:

```sh
task run -- --profile test/fixtures/skills.profile.yaml validate
task run -- --profile test/fixtures/skills.profile.yaml --json list
```

## Non-Negotiable Rules

- No `os.Exit` outside `cmd/skillset/main.go`.
- Every leaf subcommand implements `Run(*CLI) error`.
- Every machine-readable command emits root `schema_version: "v1"`.
- `tier`, `owner`, and `source` are separate concepts. Do not collapse
  install scope into maintenance ownership.
- `apply` is non-destructive: it may create missing desired user-tier entries,
  but must not delete, replace, or repair existing targets.
- `prune` is the only delete path and may delete only entries recorded as
  `skillset`-managed and no longer desired by the profile.
- The CLI must not write to live skill roots from read-only commands:
  `validate`, `normalize`, `list`, `roots`, `check`, `diff`, `doctor`, or
  `discover`.
- `adopt --apply` may write only `.skillset/state.yaml` and
  `.skillset/events.ndjson` beside the profile path.
- Do not copy the old `skillsctl` cobra/agentops/wire shape. This repo uses
  Go + kong in the compact `work-cli` style.

## Verification

The canonical gate is:

```sh
task verify
```

Any change touching command flags, JSON shapes, source URI parsing, or profile
validation must keep `task smoke` green. If smoke changes, call out the
contract change explicitly.

## Local Skill

`.claude/skills/skillset-maintainer/SKILL.md` is repo-local guidance for
maintaining this CLI. Do not turn it into a global installable `skillset`
skill in v1.

The public installable operator skill lives at
`skills/skillset-operator/SKILL.md`. Keep it focused on using the CLI safely;
put contributor-only guidance in the repo-local maintainer skill.
