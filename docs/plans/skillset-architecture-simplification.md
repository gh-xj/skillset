# Skillset Architecture Simplification Plan

Date: 2026-05-17
Status: approved direction

## Goal

Make `skillset` simpler and more elegant without expanding its product scope.
This is a v1 hardening and simplification pass, not a v2 redesign.

The target architecture should keep the current public contract mostly stable
while making the internals easier to reason about:

```text
skills.profile.yaml
  -> resolved source roots
  -> desired placements
  -> filesystem observations
  -> placement decisions
  -> command-specific projections
  -> narrow write operations
```

## Non-Goals

Do not add these in this plan:

- profile overlays, packs, sets, or multi-profile merging
- repo-tier apply or prune
- GitHub ref pinning
- package-provider abstractions beyond `npx skills`
- `normalize --write`
- first-class `bridge:` schema
- dynamic runtime/plugin discovery
- broad command/package renames before behavior is covered
- ARK skill-authoring architecture changes

ARK is only a source repository from `skillset`'s perspective in this plan.

## Agreed Decisions

### Scope

The scope is `skillset` core architecture plus its real
`private-config/skills.profile.yaml` integration. External systems are treated
as contracts:

- `npx skills`
- Claude/Codex runtime skill roots
- OpenCLI upstream skills
- ARK as a local source repo

### Profile Shape

Keep `skills.profile.yaml` human-friendly. Continue to support grouped skill
entries with `agents: [...]`.

Add profile-local named roots:

```yaml
schema_version: 1
roots:
  ark: ../github/gh-xj/agent-repo-kit/skills
  agents: ../.agents/skills
  private: .
skills:
  - name: skill-builder
    tier: user
    owner: first_party
    source: local:ark//skill-builder
    agents: [codex]
```

Rules:

- root names must be simple identifiers: `[a-z][a-z0-9_-]*`
- root paths resolve relative to the profile file
- absolute root paths are allowed but should warn
- `local:<root-name>//<skill-dir>` resolves through named roots first
- if no named root matches, keep current raw-path behavior for compatibility
- `discover --suggested-profile` should emit named roots when possible

Root matching:

- use cleaned lexical paths for profile display
- use evaluated real paths for containment checks
- choose the longest matching root
- warn when multiple roots normalize to the same path

### Internal Placement Model

Explode each profile skill entry into atomic desired placements:

```text
agent + tier + name + owner + source + source_path + target_path
```

Rename the concept of `planner.Item` toward `DesiredPlacement`, but avoid a
large rename-first diff. The implementation can add aliases or adapter helpers
first.

Split responsibilities conceptually:

- identity: `agent`, `tier`, `name`
- policy: `owner`, `source`
- resolved paths: `root_path`, `source_path`, `target_path`
- observation: target kind, source validity, target validity, lock validity
- decision: status, action, reason

### Typed Sources

Replace the optional-field source model over time. Current `profile.Source`
allows fields that only make sense for some schemes. The target should be a
typed/discriminated model:

```go
type Source struct {
    Scheme SourceScheme
    GitHub *GitHubSource
    Local  *LocalSource
    System *SystemSource
}
```

This avoids scattered checks like "if scheme is github, then owner/repo/skillDir
must be present."

### Owner Semantics

Keep `owner` explicit in v1. Do not derive it from source.

Tighten validation:

- `tier=system` requires `owner=system` and `source=system:...`
- `owner=system` requires `tier=system` and `source=system:...`
- `source=github` with `owner=private` or `owner=repo` should be a diagnostic
- `source=local` with `owner=upstream` is allowed as a bridge convention

Do not introduce a first-class `bridge:` field yet. Bridges remain represented
as local sources with the appropriate owner, for example:

```yaml
- name: opencli-browser
  tier: user
  owner: upstream
  source: local:agents//opencli-browser
  agents: [claude-code]
```

### Reconciliation Boundary

Create a small shared reconcile layer. It should not become a large framework.

The layer owns:

- expanding profile entries into desired placements
- resolving source and target roots
- observing filesystem state
- producing typed placement decisions

CLI commands become projections:

- `check`: decisions with errors
- `diff`: safe creates plus errors
- `adopt`: record present user-tier desired placements
- `apply`: execute missing user-tier create actions
- `prune`: compare managed state keys against desired placement keys

### Read Model vs Write Model

Separate read and write concepts:

```text
Profile + Roots + Filesystem -> PlacementObservation
PlacementDecision -> Operation
```

The planner/reconciler observes and classifies. Writers execute only narrow
operations produced from decisions.

### Actions vs Errors

Clarify safe creates separately from error states.

Target direction:

```text
ActionCreateLocal
ActionInstallGitHub
ActionNone
ActionAudit
ActionIgnore
```

Wrong target, wrong kind, invalid source, and invalid lock are errors, not
repair actions. `apply` must not replace or repair existing targets.

`diff` should expose:

- `creates`: safe actions that `apply` may perform
- `errors`: states humans must resolve
- `ignored`: system/repo audit items

### Filesystem Boundary

Move filesystem knowledge into `skillfs`.

Target responsibilities:

- inspect source directories
- inspect target entries
- validate `SKILL.md` frontmatter
- validate GitHub lock metadata
- compute target kind
- read symlink target
- enforce root containment

Planner/reconcile code should consume observations rather than perform
filesystem mechanics directly.

### State Model

Keep prune based on desired placement keys, but make keys typed:

```go
type PlacementKey struct {
    Agent      profile.Agent
    Tier       profile.Tier
    Name       string
    Source     string
    TargetPath string
}
```

Add methods:

```go
func (p DesiredPlacement) Key() PlacementKey
func (e ManagedEntry) Key() PlacementKey
```

Move state toward root-relative targets:

```yaml
agent: codex
tier: user
name: skill-builder
target_rel: skill-builder
target_kind: symlink
source: local:ark//skill-builder
```

Compatibility:

- read existing `target_path`
- new writes prefer `target_rel`
- runtime resolves `root(agent,tier) + target_rel`

This makes outside-root deletion harder by construction.

### JSON Output

Introduce one internal envelope shape while preserving existing top-level fields
during v1:

```json
{
  "schema_version": "v1",
  "ok": true,
  "command": "check",
  "profile_path": "skills.profile.yaml",
  "summary": {},
  "result": {},
  "warnings": [],
  "errors": []
}
```

The CLI can continue emitting legacy fields such as `items`, `changes`,
`planned`, and `applied` until a future output-contract change.

### System Skills

Keep system/plugin skills as optional explicit profile entries. They document
runtime-provided capabilities but do not need dynamic discovery in v1.

Semantics:

- profile system entries are observe/document only
- missing system entries in the profile are not errors
- if future runtime detection exists, unavailable system entries should warn
  rather than fail

### Bridge Creation

Allow `apply --apply` to create missing user-tier local bridge symlinks.

Example:

```yaml
- name: opencli-browser
  tier: user
  owner: upstream
  source: local:agents//opencli-browser
  agents: [claude-code]
```

This may create:

```text
~/.claude/skills/opencli-browser -> ~/.agents/skills/opencli-browser
```

It must not call `npx`. Repo-tier bridges remain audit-only in v1.

## Phased Refactor Plan

Each phase should keep the focused gates green. Full `task verify` is reserved
for final consolidation unless a phase changes Taskfile or smoke contracts.

### Phase 1 - Named Roots

Add profile `roots:` support while preserving raw `local:<path>//...`.

Implementation notes:

- extend `profile.Profile` with `Roots map[string]string`
- validate root names and root paths
- resolve named roots in the source/path resolution layer
- preserve existing raw local path fallback
- update docs and fixtures

Focused gates:

```sh
go test ./internal/profile ./internal/planner ./internal/skillsetcli
task smoke
skillset --profile <private-config>/skills.profile.yaml check
```

### Phase 2 - DesiredPlacement and Typed Keys

Introduce `DesiredPlacement` and `PlacementKey` without broad renames.

Implementation notes:

- keep `planner.Item` compatibility initially
- add `Key()` methods for placement and managed state
- replace string-joined keys in prune
- keep JSON output compatible

Focused gates:

```sh
go test ./internal/planner ./internal/prune ./internal/state
skillset --profile <private-config>/skills.profile.yaml check
```

### Phase 3 - Typed Source Model

Move source parsing toward discriminated source data.

Implementation notes:

- add typed source structs
- keep existing `profile.Source` JSON shape if needed for output compatibility
- migrate planner/apply/discover from optional-field access
- strengthen owner/source coherence diagnostics

Focused gates:

```sh
go test ./internal/profile ./internal/planner ./internal/apply ./internal/discover
task smoke
```

### Phase 4 - Filesystem Observations in skillfs

Centralize filesystem inspection in `skillfs`.

Implementation notes:

- add source and target inspection functions
- move target kind and symlink read logic out of adopt/prune/apply
- move root containment check into `skillfs`
- return typed observations and diagnostics

Focused gates:

```sh
go test ./internal/skillfs ./internal/planner ./internal/adopt ./internal/apply ./internal/prune
skillset --profile <private-config>/skills.profile.yaml check
```

### Phase 5 - Observation/Decision Split

Split read model from write model.

Implementation notes:

- introduce placement observations
- introduce placement decisions
- make `check` and `diff` projections over decisions
- make `adopt` and `apply` consume decisions instead of reinterpreting status
- separate safe creates from errors

Focused gates:

```sh
go test ./internal/planner ./internal/skillsetcli ./internal/adopt ./internal/apply
task smoke
```

### Phase 6 - JSON Envelope Internals

Introduce a shared internal output envelope while preserving legacy fields.

Implementation notes:

- add an envelope helper for command output
- include `ok`, `command`, `summary`, `result`, `warnings`, `errors`
- keep existing top-level fields in v1 output
- update smoke tests only if shape additions need assertions

Focused gates:

```sh
go test ./internal/skillsetcli
task smoke
```

### Phase 7 - Root-Relative State

Write new managed state with `target_rel`; read old `target_path`.

Implementation notes:

- add `target_rel` to `ManagedEntry`
- compute absolute target path at runtime from root and `target_rel`
- keep `target_path` read compatibility
- update prune safety around resolved runtime path
- avoid rewriting existing state unless an operation naturally updates it

Focused gates:

```sh
go test ./internal/state ./internal/adopt ./internal/apply ./internal/prune
skillset --profile <private-config>/skills.profile.yaml check
```

### Phase 8 - Private Profile Migration

Migrate the real private profile to named roots after compatibility and discover
support are in place.

Target shape:

```yaml
roots:
  ark: ../github/gh-xj/agent-repo-kit/skills
  agents: ../.agents/skills
  private: .
```

Then convert repeated sources:

```yaml
source: local:ark//skill-builder
source: local:agents//opencli-browser
source: local:private//.claude/skills/config-manager
```

Focused gates:

```sh
skillset --profile <private-config>/skills.profile.yaml validate
skillset --profile <private-config>/skills.profile.yaml check
```

### Final Consolidation

Run broad verification once the staged migration is complete:

```sh
task verify
task -d <private-config> dotfiles:verify
```

## Architecture Smells This Plan Addresses

### Overengineering

Avoids provider abstractions, overlays, profile writeback, and first-class bridge
schema before there is evidence they are needed.

### Data Model / Contracts

Replaces optional-everywhere source fields, string-joined placement keys, and
flat mixed planner items with typed concepts.

### Module Boundaries

Moves filesystem mechanics into `skillfs`, command projections into CLI/write
packages, and source/profile parsing into `profile`.

### Silent Failures

Strengthens invalid owner/source combinations, root ambiguity, GitHub lock
mismatch, and outside-root state into explicit diagnostics.

### Evolvability

Keeps profile compatibility while creating seams for future v2 work:

- typed sources
- typed placements
- named roots
- typed state keys
- consistent JSON envelope

## Verification Policy

Use fast focused gates during the refactor. Full `task verify` can be skipped
for intermediate phases unless the phase changes Taskfile, smoke behavior, or
repo convention surfaces.

Run full gates at the end.
