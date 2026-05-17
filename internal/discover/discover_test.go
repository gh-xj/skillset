package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunClassifiesLockSymlinkAndUnknownEntries(t *testing.T) {
	env := newDiscoverEnv(t)

	result, err := Run(Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Total != 4 || result.Summary.GitHub != 1 || result.Summary.Local != 2 || result.Summary.Unknown != 1 || result.Summary.Suggested != 3 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.SuggestedProfile.Skills) != 2 {
		t.Fatalf("expected merged suggested profile with 2 skills, got %#v", result.SuggestedProfile.Skills)
	}
	for _, entry := range result.Entries {
		if entry.Suggested != nil && entry.Reason != "" {
			t.Fatalf("suggested entry should not carry a reason: %#v", entry)
		}
	}
	local := result.SuggestedProfile.Skills[1]
	if local.Name != "skill-builder" || len(local.Agents) != 2 || local.Source != "local:.//sources/skill-builder" {
		t.Fatalf("unexpected local suggestion: %#v", local)
	}
}

func TestRunReportsBrokenSymlinkWithoutSuggestion(t *testing.T) {
	env := newDiscoverEnv(t)
	broken := filepath.Join(env.home, ".agents", "skills", "broken")
	if err := os.Symlink(filepath.Join(env.root, "missing"), broken); err != nil {
		t.Fatalf("symlink broken: %v", err)
	}

	result, err := Run(Options{ProfilePath: env.profilePath, HomeDir: env.home, RepoDir: env.repo})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary.Broken != 1 {
		t.Fatalf("expected one broken symlink, got %#v", result.Summary)
	}
	for _, entry := range result.Entries {
		if entry.Name == "broken" && entry.Suggested != nil {
			t.Fatalf("broken symlink should not have suggestion: %#v", entry)
		}
	}
}

func TestRunResolvesRelativeSymlinkThroughSymlinkedRoot(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	profilePath := filepath.Join(root, "profile", "skills.profile.yaml")
	for _, dir := range []string{
		filepath.Join(home, ".agents", "skills", "bridge"),
		filepath.Join(repo, ".claude", "skills"),
		filepath.Dir(profilePath),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(profilePath, []byte("schema_version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatalf("write profile fixture: %v", err)
	}
	if err := os.Symlink(filepath.Join(repo, ".claude"), filepath.Join(home, ".claude")); err != nil {
		t.Fatalf("symlink home .claude: %v", err)
	}
	if err := os.Symlink("../../../home/.agents/skills/bridge", filepath.Join(repo, ".claude", "skills", "bridge")); err != nil {
		t.Fatalf("symlink bridge: %v", err)
	}

	result, err := Run(Options{ProfilePath: profilePath, HomeDir: home, RepoDir: repo})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, entry := range result.Entries {
		if entry.Name != "bridge" || entry.Agent != "claude-code" || entry.Tier != "user" {
			continue
		}
		if entry.Status != StatusSuggested {
			t.Fatalf("expected symlinked user root to resolve relative target, got %#v", entry)
		}
		return
	}
	t.Fatalf("did not find bridge entry in %#v", result.Entries)
}

type discoverEnv struct {
	root        string
	home        string
	repo        string
	profileDir  string
	profilePath string
}

func newDiscoverEnv(t *testing.T) discoverEnv {
	t.Helper()
	root := t.TempDir()
	env := discoverEnv{
		root:        root,
		home:        filepath.Join(root, "home"),
		repo:        filepath.Join(root, "repo"),
		profileDir:  filepath.Join(root, "profile"),
		profilePath: filepath.Join(root, "profile", "skills.profile.yaml"),
	}
	for _, dir := range []string{
		filepath.Join(env.home, ".agents", "skills", "opencli-browser"),
		filepath.Join(env.home, ".agents", "skills", "unknown-copy"),
		filepath.Join(env.home, ".claude", "skills"),
		filepath.Join(env.repo, ".codex", "skills"),
		filepath.Join(env.repo, ".claude", "skills"),
		filepath.Join(env.profileDir, "sources", "skill-builder"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(env.profilePath, []byte("schema_version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	source := filepath.Join(env.profileDir, "sources", "skill-builder")
	for _, target := range []string{
		filepath.Join(env.home, ".agents", "skills", "skill-builder"),
		filepath.Join(env.home, ".claude", "skills", "skill-builder"),
	} {
		if err := os.Symlink(source, target); err != nil {
			t.Fatalf("symlink %s: %v", target, err)
		}
	}
	lock := map[string]any{
		"version": 3,
		"skills": map[string]any{
			"opencli-browser": map[string]any{
				"source":          "jackwener/opencli",
				"sourceType":      "github",
				"sourceUrl":       "https://github.com/jackwener/opencli.git",
				"skillPath":       "skills/opencli-browser/SKILL.md",
				"skillFolderHash": "hash",
				"installedAt":     "2026-04-24T04:35:02.535Z",
				"updatedAt":       "2026-05-17T01:26:16.214Z",
			},
		},
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(env.home, ".agents", ".skill-lock.json"), data, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	return env
}
