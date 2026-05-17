package prune

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/state"
)

func TestRunDryRunPlansOnlyUndesiredManagedEntries(t *testing.T) {
	env := newPruneEnv(t)
	plan := env.plan(t, []profile.Skill{env.desiredSkill()})
	result, err := Run(plan, Options{ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.DryRun || result.Summary.Planned != 1 || result.Summary.Skipped != 1 {
		t.Fatalf("unexpected dry-run prune result: %#v", result)
	}
	if _, err := os.Lstat(env.staleTarget); err != nil {
		t.Fatalf("dry-run should not delete stale target: %v", err)
	}
}

func TestRunApplyDeletesOnlyManagedUndesiredEntries(t *testing.T) {
	env := newPruneEnv(t)
	plan := env.plan(t, []profile.Skill{env.desiredSkill()})
	result, err := Run(plan, Options{Apply: true, ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Deleted != 1 || result.Summary.Written != 1 || result.Summary.Failed != 0 {
		t.Fatalf("unexpected prune apply result: %#v", result)
	}
	if _, err := os.Lstat(env.staleTarget); !os.IsNotExist(err) {
		t.Fatalf("expected stale target deleted, err=%v", err)
	}
	if _, err := os.Lstat(env.desiredTarget); err != nil {
		t.Fatalf("desired target should remain: %v", err)
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		t.Fatalf("Load() state error = %v", err)
	}
	if len(store.Managed) != 1 || store.Managed[0].Name != "keep" {
		t.Fatalf("expected only desired managed entry, got %#v", store.Managed)
	}
}

func TestRunApplyRefusesUnsafeDirectory(t *testing.T) {
	env := newPruneEnv(t)
	noSkill := filepath.Join(env.home, ".agents", "skills", "not-a-skill")
	if err := os.MkdirAll(noSkill, 0o755); err != nil {
		t.Fatalf("mkdir unsafe dir: %v", err)
	}
	store, err := state.Load(state.StatePathForProfile(env.profilePath))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	store = state.MergeManaged(store, []state.ManagedEntry{{
		Agent:         profile.AgentCodex,
		Tier:          profile.TierUser,
		Name:          "not-a-skill",
		Source:        "github:owner/repo//skills/not-a-skill",
		SourceScheme:  profile.SourceGitHub,
		TargetPath:    noSkill,
		TargetKind:    "directory",
		RecordedBy:    "skillset",
		RecordedAt:    fixedNow(),
		LastSeenAt:    fixedNow(),
		PruneEligible: true,
	}})
	if err := state.Save(state.StatePathForProfile(env.profilePath), store); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	result, err := Run(env.plan(t, nil), Options{Apply: true, ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Failed != 1 {
		t.Fatalf("expected one failed unsafe prune, got %#v", result)
	}
	if _, err := os.Stat(noSkill); err != nil {
		t.Fatalf("unsafe directory should remain: %v", err)
	}
}

func TestRunApplyRefusesTargetOutsideConfiguredRoot(t *testing.T) {
	env := newPruneEnv(t)
	outside := filepath.Join(env.root, "outside-target")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside target: %v", err)
	}
	writePruneSkill(t, outside, "outside-target")
	store, err := state.Load(state.StatePathForProfile(env.profilePath))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	store = state.MergeManaged(store, []state.ManagedEntry{{
		Agent:         profile.AgentCodex,
		Tier:          profile.TierUser,
		Name:          "outside-target",
		Source:        "github:owner/repo//skills/outside-target",
		SourceScheme:  profile.SourceGitHub,
		TargetPath:    outside,
		TargetKind:    "directory",
		RecordedBy:    "skillset",
		RecordedAt:    fixedNow(),
		LastSeenAt:    fixedNow(),
		PruneEligible: true,
	}})
	if err := state.Save(state.StatePathForProfile(env.profilePath), store); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	result, err := Run(env.plan(t, nil), Options{Apply: true, ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Failed != 1 {
		t.Fatalf("expected outside-root prune failure, got %#v", result)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside target should remain: %v", err)
	}
}

func TestRunApplyRefusesChangedSymlinkTarget(t *testing.T) {
	env := newPruneEnv(t)
	changedSource := filepath.Join(env.profileDir, "sources", "changed")
	if err := os.MkdirAll(changedSource, 0o755); err != nil {
		t.Fatalf("mkdir changed source: %v", err)
	}
	if err := os.Remove(env.staleTarget); err != nil {
		t.Fatalf("remove stale target: %v", err)
	}
	if err := os.Symlink(changedSource, env.staleTarget); err != nil {
		t.Fatalf("symlink changed target: %v", err)
	}

	result, err := Run(env.plan(t, []profile.Skill{env.desiredSkill()}), Options{Apply: true, ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Failed != 1 || result.Summary.Deleted != 0 {
		t.Fatalf("expected changed symlink to be refused, got %#v", result)
	}
	if _, err := os.Lstat(env.staleTarget); err != nil {
		t.Fatalf("changed symlink should remain: %v", err)
	}
}

type pruneEnv struct {
	root          string
	home          string
	repo          string
	profileDir    string
	profilePath   string
	desiredTarget string
	staleTarget   string
}

func newPruneEnv(t *testing.T) pruneEnv {
	t.Helper()
	root := t.TempDir()
	env := pruneEnv{
		root:          root,
		home:          filepath.Join(root, "home"),
		repo:          filepath.Join(root, "repo"),
		profileDir:    filepath.Join(root, "profile"),
		profilePath:   filepath.Join(root, "profile", "skills.profile.yaml"),
		desiredTarget: filepath.Join(root, "home", ".agents", "skills", "keep"),
		staleTarget:   filepath.Join(root, "home", ".agents", "skills", "stale"),
	}
	source := filepath.Join(env.profileDir, "sources", "keep")
	staleSource := filepath.Join(env.profileDir, "sources", "stale")
	for _, dir := range []string{source, staleSource, filepath.Dir(env.desiredTarget)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	writePruneSkill(t, source, "keep")
	if err := os.Symlink(source, env.desiredTarget); err != nil {
		t.Fatalf("symlink desired: %v", err)
	}
	if err := os.Symlink(staleSource, env.staleTarget); err != nil {
		t.Fatalf("symlink stale: %v", err)
	}
	if err := os.WriteFile(env.profilePath, []byte("schema_version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	now := fixedNow()
	store := state.MergeManaged(state.EmptyStore(), []state.ManagedEntry{
		{
			Agent:         profile.AgentCodex,
			Tier:          profile.TierUser,
			Name:          "keep",
			Source:        "local:.//sources/keep",
			SourceScheme:  profile.SourceLocal,
			TargetPath:    env.desiredTarget,
			TargetKind:    "symlink",
			SymlinkTarget: source,
			RecordedBy:    "skillset",
			RecordedAt:    now,
			LastSeenAt:    now,
			PruneEligible: true,
		},
		{
			Agent:         profile.AgentCodex,
			Tier:          profile.TierUser,
			Name:          "stale",
			Source:        "local:.//sources/stale",
			SourceScheme:  profile.SourceLocal,
			TargetPath:    env.staleTarget,
			TargetKind:    "symlink",
			SymlinkTarget: staleSource,
			RecordedBy:    "skillset",
			RecordedAt:    now,
			LastSeenAt:    now,
			PruneEligible: true,
		},
	})
	if err := state.Save(state.StatePathForProfile(env.profilePath), store); err != nil {
		t.Fatalf("Save() state error = %v", err)
	}
	return env
}

func (e pruneEnv) desiredSkill() profile.Skill {
	return profile.Skill{Name: "keep", Tier: profile.TierUser, Owner: profile.OwnerPrivate, Source: "local:.//sources/keep", Agents: []profile.Agent{profile.AgentCodex}}
}

func writePruneSkill(t *testing.T, dir, name string) {
	t.Helper()
	body := "---\nname: " + name + "\ndescription: Fixture skill.\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md for %s: %v", name, err)
	}
}

func (e pruneEnv) plan(t *testing.T, skills []profile.Skill) planner.Plan {
	t.Helper()
	p := profile.Profile{SchemaVersion: profile.CurrentSchemaVersion, Skills: skills}
	plan, err := planner.Build(p, planner.Options{ProfilePath: e.profilePath, HomeDir: e.home, RepoDir: e.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return plan
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 17, 7, 20, 0, 0, time.UTC)
}
