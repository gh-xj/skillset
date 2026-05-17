# skillset CLI

`skillset` is a CLI for managing `skills.profile.yaml` as the desired state
for agent skills across tiers. It delegates upstream package mechanics to
`npx skills` and keeps policy, planning, validation, and reconciliation in one
small Go binary.

Status: guarded reconcile slice. Implemented commands are
`validate`, `normalize`, `list`, `roots`, `check`, `diff`, `doctor`,
`discover`, `adopt`, `apply`, `prune`, `managed`, and `version`.

## Install

```sh
go install github.com/gh-xj/skillset/cmd/skillset@latest
```

v0.1.0 is distributed through `go install` only. Binary release artifacts are
deferred until the CLI has a release packaging pipeline.

## Agent Skill

The `skillset` repo ships an official operator skill for agents that need to
use the CLI safely:

```sh
npx skills add gh-xj/skillset -g -s skillset-operator -a codex -y --copy
```

After `skillset` is already in use, the same skill can be represented in a
profile:

```yaml
- name: skillset-operator
  tier: user
  owner: first_party
  source: github:gh-xj/skillset//skills/skillset-operator
  agents:
    - codex
```

Contributor guidance remains separate in the repo-local
`.claude/skills/skillset-maintainer/` skill.

## New User Flow

1. Install the CLI:

   ```sh
   go install github.com/gh-xj/skillset/cmd/skillset@latest
   ```

2. Install the operator skill for the agent that will manage skill state:

   ```sh
   npx skills add gh-xj/skillset -g -s skillset-operator -a codex -y --copy
   ```

3. Generate a draft profile from the current machine:

   ```sh
   skillset discover --suggested-profile > skills.profile.yaml
   ```

4. Edit `skills.profile.yaml` so sources are intentional, then check it:

   ```sh
   skillset validate
   skillset check
   skillset diff
   skillset doctor
   ```

5. If existing installs should become managed, dry-run first and then record
   evidence:

   ```sh
   skillset adopt
   skillset adopt --apply
   ```

6. On later changes, dry-run write commands before applying them:

   ```sh
   skillset apply
   skillset prune
   ```

## Profile Shape

```yaml
schema_version: 1
skills:
  - name: opencli-browser
    tier: user
    owner: upstream
    source: github:jackwener/opencli//skills/opencli-browser
    agents:
      - codex
```

v1 tiers:

- `system` - runtime-provided skill; check/ignore only.
- `user` - user-level activation or install scope.
- `repo` - repo-level activation; audit/check only in v1.

v1 owners:

- `upstream`
- `first_party`
- `private`
- `repo`
- `system`

v1 source schemes:

- `github:<owner>/<repo>//<skill-dir>`
- `local:<root>//<skill-dir>`
- `system:<agent>/<skill>`

## Usage

```sh
skillset --profile skills.profile.yaml validate
skillset --profile skills.profile.yaml normalize
skillset --profile skills.profile.yaml --json list
skillset --profile skills.profile.yaml --json list --agent codex
skillset --json roots
skillset --profile skills.profile.yaml --json check
skillset --profile skills.profile.yaml --json diff
skillset --profile skills.profile.yaml --json doctor
skillset --profile skills.profile.yaml --json discover
skillset --profile skills.profile.yaml discover --suggested-profile
skillset --profile skills.profile.yaml --json adopt
skillset --profile skills.profile.yaml --json adopt --apply
skillset --profile skills.profile.yaml --json apply
skillset --profile skills.profile.yaml --json apply --apply
skillset --profile skills.profile.yaml --json prune
skillset --profile skills.profile.yaml --json prune --apply
skillset --profile skills.profile.yaml --json managed
```

Every command supports `--json` and emits root `schema_version: "v1"`.
Use `--home` and `--repo` to point root resolution at fixture or alternate
directories without touching the real machine roots.

## Boundaries

`skillset` is not a replacement package manager. Write commands create a plan,
use local symlinks for `local:` sources, delegate `github:` sources to
`npx skills`, ignore `system:` sources, and record managed state under the
profile directory's ignored `.skillset/` folder.

`apply` is non-destructive. `prune` is the only delete path and deletes only
entries previously recorded as `skillset`-managed.

`adopt` defaults to dry-run. `adopt --apply` writes only profile-local
`.skillset/state.yaml` and `.skillset/events.ndjson`; it does not change live
skill roots. `apply` is non-destructive and creates only missing desired
user-tier skills. `prune` defaults to dry-run and deletes only entries already
recorded as `skillset`-managed. Local state design lives in
[docs/taxonomy/state.md](docs/taxonomy/state.md).

## Development

```sh
task ci
task verify
```

`task verify` is also the GitHub Actions CI gate. The smoke fixture lives at
`test/fixtures/skills.profile.yaml`.

## License

MIT.
