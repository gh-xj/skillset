package apply

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/state"
)

func TestRunDryRunPlansMissingTargetsWithoutWriting(t *testing.T) {
	env := newApplyEnv(t)
	plan := env.plan(t)
	result, err := Run(plan, Options{ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.DryRun || result.Summary.Planned != 2 || result.Summary.Applied != 0 {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if _, err := os.Stat(result.StatePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write state, err=%v", err)
	}
}

func TestRunApplyCreatesLocalSymlinkAndRecordsState(t *testing.T) {
	env := newApplyEnv(t)
	plan := env.localOnlyPlan(t)
	result, err := Run(plan, Options{Apply: true, ProfilePath: env.profilePath, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Applied != 1 || result.Summary.Written != 1 || len(result.Failed) != 0 {
		t.Fatalf("unexpected apply result: %#v", result)
	}
	target := filepath.Join(env.home, ".agents", "skills", "skill-builder")
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected local symlink at %s, info=%v err=%v", target, info, err)
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		t.Fatalf("Load() state error = %v", err)
	}
	if len(store.Managed) != 1 || store.Managed[0].TargetKind != "symlink" || store.Managed[0].TargetRel != "skill-builder" {
		t.Fatalf("unexpected managed state: %#v", store.Managed)
	}
}

func TestRunApplyDelegatesGitHubAndRequiresTarget(t *testing.T) {
	env := newApplyEnv(t)
	plan := env.githubOnlyPlan(t)
	var called []string
	result, err := Run(plan, Options{
		Apply:       true,
		ProfilePath: env.profilePath,
		Now:         fixedNow,
		Runner: func(cmd Command) CommandResult {
			called = append([]string(nil), cmd.Args...)
			target := filepath.Join(env.home, ".agents", "skills", "opencli-browser")
			if err := os.MkdirAll(target, 0o755); err != nil {
				return CommandResult{ExitCode: 1, Err: err.Error()}
			}
			if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\nname: opencli-browser\ndescription: Fixture skill.\n---\n\n# opencli-browser\n"), 0o644); err != nil {
				return CommandResult{ExitCode: 1, Err: err.Error()}
			}
			lockPath := filepath.Join(env.home, ".agents", ".skill-lock.json")
			lock := `{"skills":{"opencli-browser":{"source":"jackwener/opencli","sourceType":"github","skillPath":"skills/opencli-browser/SKILL.md"}}}`
			if err := os.WriteFile(lockPath, []byte(lock), 0o644); err != nil {
				return CommandResult{ExitCode: 1, Err: err.Error()}
			}
			return CommandResult{}
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Applied != 1 || result.Summary.Written != 1 || len(called) == 0 || called[0] != "npx" {
		t.Fatalf("unexpected github apply result=%#v command=%#v", result, called)
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		t.Fatalf("Load() state error = %v", err)
	}
	if len(store.Managed) != 1 || len(store.Managed[0].InstallCommand) == 0 {
		t.Fatalf("expected install command in state, got %#v", store.Managed)
	}
}

func TestRunApplyDoesNotRecordFailedGitHub(t *testing.T) {
	env := newApplyEnv(t)
	plan := env.githubOnlyPlan(t)
	result, err := Run(plan, Options{
		Apply:       true,
		ProfilePath: env.profilePath,
		Now:         fixedNow,
		Runner: func(Command) CommandResult {
			return CommandResult{ExitCode: 1, Err: "boom"}
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Failed != 1 || result.Summary.Written != 0 {
		t.Fatalf("unexpected failed apply result: %#v", result)
	}
	if _, err := os.Stat(result.StatePath); !os.IsNotExist(err) {
		t.Fatalf("failed apply should not write state, err=%v", err)
	}
}

type applyEnv struct {
	root        string
	home        string
	repo        string
	profileDir  string
	profilePath string
}

func newApplyEnv(t *testing.T) applyEnv {
	t.Helper()
	root := t.TempDir()
	env := applyEnv{
		root:        root,
		home:        filepath.Join(root, "home"),
		repo:        filepath.Join(root, "repo"),
		profileDir:  filepath.Join(root, "profile"),
		profilePath: filepath.Join(root, "profile", "skills.profile.yaml"),
	}
	for _, dir := range []string{
		filepath.Join(env.home, ".agents", "skills"),
		filepath.Join(env.profileDir, "sources", "skill-builder"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	writeApplySkill(t, filepath.Join(env.profileDir, "sources", "skill-builder"), "skill-builder")
	if err := os.WriteFile(env.profilePath, []byte("schema_version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatalf("write profile placeholder: %v", err)
	}
	return env
}

func writeApplySkill(t *testing.T, dir, name string) {
	t.Helper()
	body := "---\nname: " + name + "\ndescription: Fixture skill.\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md for %s: %v", name, err)
	}
}

func (e applyEnv) plan(t *testing.T) planner.Plan {
	t.Helper()
	return e.buildPlan(t, []profile.Skill{localSkill(), githubSkill(), systemSkill()})
}

func (e applyEnv) localOnlyPlan(t *testing.T) planner.Plan {
	t.Helper()
	return e.buildPlan(t, []profile.Skill{localSkill()})
}

func (e applyEnv) githubOnlyPlan(t *testing.T) planner.Plan {
	t.Helper()
	return e.buildPlan(t, []profile.Skill{githubSkill()})
}

func (e applyEnv) buildPlan(t *testing.T, skills []profile.Skill) planner.Plan {
	t.Helper()
	p := profile.Profile{SchemaVersion: profile.CurrentSchemaVersion, Skills: skills}
	plan, err := planner.Build(p, planner.Options{ProfilePath: e.profilePath, HomeDir: e.home, RepoDir: e.repo})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return plan
}

func localSkill() profile.Skill {
	return profile.Skill{Name: "skill-builder", Tier: profile.TierUser, Owner: profile.OwnerFirstParty, Source: "local:.//sources/skill-builder", Agents: []profile.Agent{profile.AgentCodex}}
}

func githubSkill() profile.Skill {
	return profile.Skill{Name: "opencli-browser", Tier: profile.TierUser, Owner: profile.OwnerUpstream, Source: "github:jackwener/opencli//skills/opencli-browser", Agents: []profile.Agent{profile.AgentCodex}}
}

func systemSkill() profile.Skill {
	return profile.Skill{Name: "browser-use", Tier: profile.TierSystem, Owner: profile.OwnerSystem, Source: "system:codex/browser-use", Agents: []profile.Agent{profile.AgentCodex}}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 17, 7, 0, 0, 0, time.UTC)
}
