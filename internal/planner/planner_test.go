package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gh-xj/skillset/internal/profile"
)

func TestBuildPlansInstalledAndMissingItems(t *testing.T) {
	env := newPlannerEnv(t)
	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Skills: []profile.Skill{
			{
				Name:   "local-skill",
				Tier:   profile.TierUser,
				Owner:  profile.OwnerPrivate,
				Source: "local:.//sources/local-skill",
				Agents: []profile.Agent{profile.AgentCodex},
			},
			{
				Name:   "github-skill",
				Tier:   profile.TierUser,
				Owner:  profile.OwnerUpstream,
				Source: "github:owner/repo//skills/github-skill",
				Agents: []profile.Agent{profile.AgentCodex},
			},
			{
				Name:   "system-skill",
				Tier:   profile.TierSystem,
				Owner:  profile.OwnerSystem,
				Source: "system:codex/system-skill",
				Agents: []profile.Agent{profile.AgentCodex},
			},
		},
	}

	plan, err := Build(p, Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Summary.Total != 3 || plan.Summary.Present != 1 || plan.Summary.MissingTarget != 1 || plan.Summary.SystemIgnored != 1 {
		t.Fatalf("unexpected summary: %#v", plan.Summary)
	}
	if len(plan.Changes()) != 1 || plan.Changes()[0].Action != ActionInstallGitHub {
		t.Fatalf("expected one github install change, got %#v", plan.Changes())
	}
}

func TestBuildRejectsWrongLocalSymlinkTarget(t *testing.T) {
	env := newPlannerEnv(t)
	wrong := filepath.Join(env.profileDir, "sources", "wrong")
	if err := os.MkdirAll(wrong, 0o755); err != nil {
		t.Fatalf("mkdir wrong source: %v", err)
	}
	target := filepath.Join(env.home, ".agents", "skills", "local-skill")
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove target symlink: %v", err)
	}
	if err := os.Symlink(wrong, target); err != nil {
		t.Fatalf("write wrong target symlink: %v", err)
	}

	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Skills: []profile.Skill{{
			Name:   "local-skill",
			Tier:   profile.TierUser,
			Owner:  profile.OwnerPrivate,
			Source: "local:.//sources/local-skill",
			Agents: []profile.Agent{profile.AgentCodex},
		}},
	}
	plan, err := Build(p, Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Summary.WrongTarget != 1 || len(plan.ErrorItems()) != 1 {
		t.Fatalf("expected wrong target error, got summary=%#v errors=%#v", plan.Summary, plan.ErrorItems())
	}
}

type plannerEnv struct {
	root        string
	home        string
	repo        string
	profileDir  string
	profilePath string
}

func newPlannerEnv(t *testing.T) plannerEnv {
	t.Helper()
	root := t.TempDir()
	env := plannerEnv{
		root:        root,
		home:        filepath.Join(root, "home"),
		repo:        filepath.Join(root, "repo"),
		profileDir:  filepath.Join(root, "profile"),
		profilePath: filepath.Join(root, "profile", "skills.profile.yaml"),
	}
	localSource := filepath.Join(env.profileDir, "sources", "local-skill")
	if err := os.MkdirAll(localSource, 0o755); err != nil {
		t.Fatalf("mkdir local source: %v", err)
	}
	userRoot := filepath.Join(env.home, ".agents", "skills")
	if err := os.MkdirAll(userRoot, 0o755); err != nil {
		t.Fatalf("mkdir user root: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(env.profilePath), 0o755); err != nil {
		t.Fatalf("mkdir profile dir: %v", err)
	}
	if err := os.WriteFile(env.profilePath, []byte("schema_version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatalf("write profile placeholder: %v", err)
	}
	if err := os.Symlink(localSource, filepath.Join(userRoot, "local-skill")); err != nil {
		t.Fatalf("write local skill symlink: %v", err)
	}
	return env
}
