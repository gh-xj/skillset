---
name: skillset-operator
description: Use when operating the skillset CLI for agent skill desired-state management: creating or reviewing skills.profile.yaml, discovering installed skills, checking drift, adopting existing installs, applying missing user-tier skills, or pruning skillset-managed entries safely.
---

# skillset-operator

Use the `skillset` CLI as a desired-state manager for agent skills. The profile
is the source of truth; live skill roots are current state; `.skillset/` is
local evidence for safe pruning.

## First Principles

- Treat `skills.profile.yaml` as intent. Do not treat generated state or live
  roots as the durable contract.
- Prefer read-only commands before write commands.
- Never edit live skill roots directly when `skillset` can inspect or plan the
  change.
- `apply` is non-destructive: it creates missing desired user-tier entries but
  does not replace existing targets.
- `prune` is the only delete path. It may delete only entries already recorded
  as `skillset`-managed and no longer desired by the profile.
- Always dry-run `adopt`, `apply`, and `prune` before using `--apply`.

## Install The CLI

Check whether the CLI is already available:

```sh
skillset version
```

If it is missing:

```sh
go install github.com/gh-xj/skillset/cmd/skillset@latest
```

If the repo is checked out locally, prefer its built binary or Taskfile while
developing:

```sh
task build
./bin/skillset version
```

## New Profile Flow

For an existing machine, discover current roots and save a draft profile:

```sh
skillset discover --suggested-profile > skills.profile.yaml
skillset validate
skillset check
skillset diff
skillset doctor
```

Then edit the profile so sources are intentional and stable. Replace absolute
machine paths with stable relative `local:` roots when possible.

For a fresh profile, start small:

```yaml
schema_version: 1
skills:
  - name: opencli-browser
    tier: user
    owner: upstream
    source: github:jackwener/opencli//skills/opencli-browser
    agents: [codex]
```

## Adopt Existing Installs

Use this when live roots already contain the desired skills and you want
`skillset` to record managed-entry evidence for future safe pruning.

```sh
skillset adopt
skillset adopt --apply
skillset managed
```

`adopt --apply` writes only `.skillset/state.yaml` and
`.skillset/events.ndjson` beside the profile path. It must not rewrite live
skill roots.

## Apply Missing Skills

Use this on a fresh machine or after adding desired user-tier skills to the
profile.

```sh
skillset diff
skillset apply
skillset apply --apply
skillset check
```

`local:` sources become symlinks. `github:` sources delegate to `npx skills`
in copy mode. `system:` sources are check/ignore only. `repo` tier is
audit/check only in v1.

## Prune Safely

Use this only after removing a skill from `skills.profile.yaml`.

```sh
skillset prune
skillset prune --apply
skillset check
```

Stop if the dry-run output includes anything unexpected. `prune --apply`
should delete only entries that are:

- absent from the current profile
- recorded in `.skillset/state.yaml`
- user-tier
- still matching the recorded target kind and safety evidence

## Triage Output

- `missing_target`: expected when a desired user-tier skill is absent. Use
  `apply` after reviewing `diff`.
- `missing_source`: the profile points at a local source path that does not
  exist. Fix the profile or source checkout first.
- `wrong_kind`: local sources should be symlinks; GitHub sources should be
  copied directories in v1.
- `wrong_target`: a local symlink points somewhere other than the desired
  source. Do not repair blindly; inspect the live target first.
- `repo_audit_only`: expected for repo-tier entries in v1.
- `system_ignored`: expected for system-tier entries.

## Profile Entry Patterns

Upstream copied skill:

```yaml
- name: opencli-browser
  tier: user
  owner: upstream
  source: github:jackwener/opencli//skills/opencli-browser
  agents: [codex]
```

First-party local symlink:

```yaml
- name: work-cli
  tier: user
  owner: first_party
  source: local:../agent-repo-kit/skills//work-cli
  agents: [codex]
```

Runtime-provided system skill:

```yaml
- name: imagegen
  tier: system
  owner: system
  source: system:codex/imagegen
```

## Red Flags

- Do not add GitHub ref pinning; v1 rejects it.
- Do not make `skillset` manage repo-tier writes or deletes; v1 is audit-only
  for repo tier.
- Do not run `prune --apply` just because `check` has errors. Understand the
  state entry and dry-run plan first.
- Do not use `.skillset/state.yaml` as intent. Update `skills.profile.yaml`
  instead.
