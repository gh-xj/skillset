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
	if len(plan.Creates()) != 1 || plan.Creates()[0].Action != ActionInstallGitHub || len(plan.Changes()) != len(plan.Creates()) {
		t.Fatalf("expected one github create decision, got changes=%#v creates=%#v", plan.Changes(), plan.Creates())
	}
	if len(plan.Ignored()) != 1 || plan.Ignored()[0].Status != StatusSystemIgnored {
		t.Fatalf("expected one ignored system decision, got %#v", plan.Ignored())
	}
}

func TestBuildResolvesNamedLocalRoots(t *testing.T) {
	env := newPlannerEnv(t)
	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Roots: map[string]string{
			"fixtures": "sources",
		},
		Skills: []profile.Skill{{
			Name:   "local-skill",
			Tier:   profile.TierUser,
			Owner:  profile.OwnerPrivate,
			Source: "local:fixtures//local-skill",
			Agents: []profile.Agent{profile.AgentCodex},
		}},
	}

	plan, err := Build(p, Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Summary.Present != 1 {
		t.Fatalf("expected named-root local skill to be present, got summary=%#v item=%#v", plan.Summary, plan.Items)
	}
	if got, want := plan.Items[0].SourcePath, filepath.Join(env.profileDir, "sources", "local-skill"); got != want {
		t.Fatalf("SourcePath = %q, want %q", got, want)
	}
	key := plan.Items[0].Key()
	if key.Agent != profile.AgentCodex || key.Tier != profile.TierUser || key.Name != "local-skill" || key.Source != "local:fixtures//local-skill" || key.TargetPath == "" {
		t.Fatalf("unexpected placement key: %#v", key)
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
	if len(plan.Creates()) != 0 || plan.Summary.Changes != 0 || plan.Items[0].Action != ActionNone {
		t.Fatalf("wrong target must not be advertised as a create action, got summary=%#v creates=%#v item=%#v", plan.Summary, plan.Creates(), plan.Items[0])
	}
}

func TestBuildRejectsLocalSourceWithoutSkillFile(t *testing.T) {
	env := newPlannerEnv(t)
	badSource := filepath.Join(env.profileDir, "sources", "bad-source")
	if err := os.MkdirAll(badSource, 0o755); err != nil {
		t.Fatalf("mkdir bad source: %v", err)
	}
	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Skills: []profile.Skill{{
			Name:   "bad-source",
			Tier:   profile.TierUser,
			Owner:  profile.OwnerPrivate,
			Source: "local:.//sources/bad-source",
			Agents: []profile.Agent{profile.AgentCodex},
		}},
	}
	plan, err := Build(p, Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Summary.MissingSource != 1 || len(plan.ErrorItems()) != 1 {
		t.Fatalf("expected invalid local source error, got summary=%#v errors=%#v", plan.Summary, plan.ErrorItems())
	}
}

func TestBuildRejectsGitHubTargetWithoutMatchingLock(t *testing.T) {
	env := newPlannerEnv(t)
	target := filepath.Join(env.home, ".agents", "skills", "github-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir github target: %v", err)
	}
	writeSkill(t, target, "github-skill")
	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Skills: []profile.Skill{{
			Name:   "github-skill",
			Tier:   profile.TierUser,
			Owner:  profile.OwnerUpstream,
			Source: "github:owner/repo//skills/github-skill",
			Agents: []profile.Agent{profile.AgentCodex},
		}},
	}
	plan, err := Build(p, Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Summary.WrongKind != 1 || len(plan.ErrorItems()) != 1 {
		t.Fatalf("expected github lock error, got summary=%#v errors=%#v", plan.Summary, plan.ErrorItems())
	}
}

func TestBuildRejectsGitHubSymlinkWithoutCreateAction(t *testing.T) {
	env := newPlannerEnv(t)
	target := filepath.Join(env.home, ".agents", "skills", "github-skill")
	if err := os.Symlink(filepath.Join(env.profileDir, "sources", "local-skill"), target); err != nil {
		t.Fatalf("write github target symlink: %v", err)
	}
	p := profile.Profile{
		SchemaVersion: profile.CurrentSchemaVersion,
		Skills: []profile.Skill{{
			Name:   "github-skill",
			Tier:   profile.TierUser,
			Owner:  profile.OwnerUpstream,
			Source: "github:owner/repo//skills/github-skill",
			Agents: []profile.Agent{profile.AgentCodex},
		}},
	}
	plan, err := Build(p, Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Summary.WrongKind != 1 || len(plan.Creates()) != 0 || plan.Items[0].Action != ActionNone {
		t.Fatalf("expected github symlink to be an error without create action, got summary=%#v creates=%#v item=%#v", plan.Summary, plan.Creates(), plan.Items[0])
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
	writeSkill(t, localSource, "local-skill")
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

func writeSkill(t *testing.T, dir, name string) {
	t.Helper()
	body := "---\nname: " + name + "\ndescription: Fixture skill.\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md for %s: %v", name, err)
	}
}
