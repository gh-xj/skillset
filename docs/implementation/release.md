# Release

Release `skillset` only after the checked-in CLI, installable skill, and
profile examples are consistent.

## v0.1 Policy

v0.1 releases are distributed through Go module installation only:

```sh
go install github.com/gh-xj/skillset/cmd/skillset@latest
```

Do not attach binary tarballs or checksums until a release packaging workflow
exists and is covered by CI.

## Checklist

1. Run the local gate:

```sh
task verify
```

2. Confirm the public operator skill installs from the current remote:

```sh
npx skills add gh-xj/skillset -g -s skillset-operator -a codex -y --copy
```

3. Confirm Go installation works:

```sh
go install github.com/gh-xj/skillset/cmd/skillset@latest
skillset version
```

4. Check profile examples and docs for current owner/source semantics. The
   first-party operator skill should use:

```yaml
owner: first_party
source: github:gh-xj/skillset//skills/skillset-operator
```

5. Tag the release after the commit intended for release:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

6. Wait for CI to pass on the tag.

7. Create the GitHub release from the tag. State clearly whether the release
   has binary artifacts or is `go install` only.

8. If private-config or another profile tracks `skillset-operator`, run its
   profile check after refreshing the installed skill.

## Draft Notes For Next Release

- `skills.profile.yaml` supports optional named `roots:` for reusable local
  source roots. `local:<name>//<skill>` resolves through `roots.<name>` before
  falling back to raw path behavior.
- `discover --suggested-profile` reuses declared roots and can infer simple
  profile-local roots such as `sources`.
- Managed state writes prefer `target_rel` and continue to read older
  `target_path` entries for compatibility.
- `diff --json` includes `creates`, `errors`, and `ignored` projections while
  preserving the legacy `changes` and `items` fields.
- JSON output now has a common command envelope with `ok`, `command`,
  `summary`, `result`, `warnings`, and `errors` fields while retaining v1
  top-level command fields.

## Artifact Decision

Before adding binary artifacts, create a real packaging path first:

- release workflow or GoReleaser config
- `darwin`/`linux` builds for `amd64`/`arm64`
- checksums
- smoke or install verification against the generated artifacts
- README release instructions updated in the same change
