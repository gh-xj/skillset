package adopt

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/state"
)

func TestRunDryRunDoesNotWriteState(t *testing.T) {
	env := newAdoptEnv(t)
	plan := env.plan(t)
	result, err := Run(plan, Options{ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.DryRun || result.Summary.Adopted != 2 || result.Summary.Skipped != 1 {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if _, err := os.Stat(result.StatePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write state, stat err=%v", err)
	}
}

func TestRunApplyWritesStateAndEvents(t *testing.T) {
	env := newAdoptEnv(t)
	plan := env.plan(t)
	result, err := Run(plan, Options{Apply: true, ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.DryRun || result.Summary.Written != 2 {
		t.Fatalf("unexpected apply result: %#v", result)
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		t.Fatalf("Load() state error = %v", err)
	}
	if len(store.Managed) != 2 {
		t.Fatalf("expected two managed entries, got %#v", store.Managed)
	}
	if store.Managed[0].TargetKind != "directory" || len(store.Managed[0].InstallCommand) == 0 {
		t.Fatalf("expected github directory install command, got %#v", store.Managed[0])
	}
	if store.Managed[1].TargetKind != "symlink" || store.Managed[1].SymlinkTarget == "" {
		t.Fatalf("expected local symlink metadata, got %#v", store.Managed[1])
	}
	events, err := state.LoadEvents(result.EventsPath)
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 2 || events[0].Operation != "adopt" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

type adoptEnv struct {
	root        string
	home        string
	repo        string
	profileDir  string
	profilePath string
}

func newAdoptEnv(t *testing.T) adoptEnv {
	t.Helper()
	root := t.TempDir()
	env := adoptEnv{
		root:        root,
		home:        filepath.Join(root, "home"),
		repo:        filepath.Join(root, "repo"),
		profileDir:  filepath.Join(root, "profile"),
		profilePath: filepath.Join(root, "profile", "skills.profile.yaml"),
	}
	for _, dir := range []string{
		filepath.Join(env.home, ".agents", "skills", "opencli-browser"),
		filepath.Join(env.home, ".agents", "skills"),
		filepath.Join(env.profileDir, "sources", "skill-builder"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.Symlink(filepath.Join(env.profileDir, "sources", "skill-builder"), filepath.Join(env.home, ".agents", "skills", "skill-builder")); err != nil {
		t.Fatalf("symlink local skill: %v", err)
	}
	if err := os.WriteFile(env.profilePath, []byte("schema_version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatalf("write profile placeholder: %v", err)
	}
	return env
}

func (e adoptEnv) plan(t *testing.T) planner.Plan {
	t.Helper()
	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Skills: []profile.Skill{
			{
				Name:   "opencli-browser",
				Tier:   profile.TierUser,
				Owner:  profile.OwnerUpstream,
				Source: "github:jackwener/opencli//skills/opencli-browser",
				Agents: []profile.Agent{profile.AgentCodex},
			},
			{
				Name:   "skill-builder",
				Tier:   profile.TierUser,
				Owner:  profile.OwnerFirstParty,
				Source: "local:.//sources/skill-builder",
				Agents: []profile.Agent{profile.AgentCodex},
			},
			{
				Name:   "browser-use",
				Tier:   profile.TierSystem,
				Owner:  profile.OwnerSystem,
				Source: "system:codex/browser-use",
				Agents: []profile.Agent{profile.AgentCodex},
			},
		},
	}
	plan, err := planner.Build(p, planner.Options{ProfilePath: e.profilePath, HomeDir: e.home, RepoDir: e.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return plan
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 17, 6, 30, 0, 0, time.UTC)
}
