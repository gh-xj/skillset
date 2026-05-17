# Core Belief

The `skillset` CLI exists to make agent skill state explicit, reviewable, and
reproducible without turning skill installation into another hidden runtime
habit.

## Beliefs

1. Desired state belongs in one profile. `skills.profile.yaml` is the human
   reviewed contract; local roots are derived state.
2. Scope is not ownership. `tier` says where a skill is active; `owner` says
   who maintains it; `source` says where it comes from.
3. Read-only truth comes first. Validation, normalization, listing, checking,
   and diffing must be trustworthy before any command writes.
4. Destruction must be named. `apply` may create or update, but deletion
   belongs only to `prune`.
5. Package mechanics stay delegated. `github:` sources use `npx skills`;
   `local:` sources use symlinks; `system:` sources are observed, not managed.
6. State is evidence, not authority. `.skillset/` records what `skillset`
   manages so prune can be safe; the profile remains the desired state.
7. Keep v1 small. No overlays, packs, profile merging, repo-tier apply,
   reinstall/replace, export, or GitHub ref pinning until the base contract
   has proven itself.

When a feature conflicts with these beliefs, name the conflict before coding.
