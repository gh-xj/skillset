package skillsetcli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gh-xj/skillset/internal/appctx"
)

func runSkillset(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := execWriters(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestExecuteUnknownCommandReturnsUsageCode(t *testing.T) {
	code, _, _ := runSkillset(t, "not-a-real-command")
	if code != appctx.ExitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
}

func TestExecuteVersionJSON(t *testing.T) {
	code, stdout, stderr := runSkillset(t, "--json", "version")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal version: %v", err)
	}
	if payload.SchemaVersion != "v1" || payload.Name != binaryName {
		t.Fatalf("unexpected version payload: %#v", payload)
	}
}

func TestValidateJSON(t *testing.T) {
	path := writeCLIProfile(t, validProfile)
	code, stdout, stderr := runSkillset(t, "--profile", path, "--json", "validate")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Valid         bool   `json:"valid"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal validate: %v", err)
	}
	if payload.SchemaVersion != "v1" || !payload.Valid {
		t.Fatalf("unexpected validate payload: %#v", payload)
	}
}

func TestValidateInvalidJSONExitsOne(t *testing.T) {
	path := writeCLIProfile(t, "schema_version: 99\nskills: []\n")
	code, stdout, stderr := runSkillset(t, "--profile", path, "--json", "validate")
	if code != appctx.ExitError {
		t.Fatalf("expected error exit, got %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, `"valid": false`) {
		t.Fatalf("expected invalid JSON payload, got %q", stdout)
	}
}

func TestListJSONIncludesSourceScheme(t *testing.T) {
	path := writeCLIProfile(t, validProfile)
	code, stdout, stderr := runSkillset(t, "--profile", path, "--json", "list", "--agent", "codex")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		Count  int `json:"count"`
		Skills []struct {
			Name         string `json:"name"`
			SourceScheme string `json:"source_scheme"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if payload.Count != 2 {
		t.Fatalf("expected two codex skills, got %#v", payload)
	}
	if payload.Skills[0].Name != "skill-builder" || payload.Skills[0].SourceScheme != "local" {
		t.Fatalf("unexpected first list item: %#v", payload.Skills[0])
	}
}

func TestNormalizeJSONIncludesProfile(t *testing.T) {
	path := writeCLIProfile(t, validProfile)
	code, stdout, stderr := runSkillset(t, "--profile", path, "--json", "normalize")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stdout, `"profile"`) || !strings.Contains(stdout, `"schema_version": "v1"`) {
		t.Fatalf("expected normalized profile JSON, got %q", stdout)
	}
}

func TestRootsJSONUsesGlobalHomeAndRepo(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	code, stdout, stderr := runSkillset(t, "--home", env.home, "--repo", env.repo, "--json", "roots", "--agent", "codex")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		Count int `json:"count"`
		Roots []struct {
			Agent string `json:"agent"`
			Tier  string `json:"tier"`
		} `json:"roots"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal roots: %v", err)
	}
	if payload.Count != 3 || payload.Roots[0].Agent != "codex" {
		t.Fatalf("unexpected roots payload: %#v", payload)
	}
}

func TestCheckJSONPassesWithInstalledFixture(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "check")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	var payload struct {
		OK      bool `json:"ok"`
		Summary struct {
			Errors int `json:"errors"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal check: %v", err)
	}
	if !payload.OK || payload.Summary.Errors != 0 {
		t.Fatalf("unexpected check payload: %#v", payload)
	}
}

func TestDiffJSONReportsMissingGitHubInstall(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	if err := os.RemoveAll(filepath.Join(env.home, ".agents", "skills", "opencli-browser")); err != nil {
		t.Fatalf("remove github target: %v", err)
	}
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "diff")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr)
	}
	var payload struct {
		Changes []struct {
			Name   string `json:"name"`
			Action string `json:"action"`
		} `json:"changes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if len(payload.Changes) != 1 || payload.Changes[0].Name != "opencli-browser" || payload.Changes[0].Action != "install_github" {
		t.Fatalf("unexpected diff payload: %#v", payload)
	}
}

func TestDoctorJSONReportsStateErrors(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	if err := os.RemoveAll(filepath.Join(env.home, ".agents", "skills", "skill-builder")); err != nil {
		t.Fatalf("remove local target: %v", err)
	}
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "doctor")
	if code != appctx.ExitError {
		t.Fatalf("expected error exit, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	if !strings.Contains(stdout, `"name": "skill_state"`) || !strings.Contains(stdout, `"status": "error"`) {
		t.Fatalf("expected doctor skill_state error, got %q", stdout)
	}
}

func TestAdoptDryRunDoesNotWriteState(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "adopt")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	var payload struct {
		DryRun  bool `json:"dry_run"`
		Summary struct {
			Adopted int `json:"adopted"`
			Skipped int `json:"skipped"`
			Written int `json:"written"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal adopt: %v", err)
	}
	if !payload.DryRun || payload.Summary.Adopted != 3 || payload.Summary.Skipped != 1 || payload.Summary.Written != 0 {
		t.Fatalf("unexpected dry-run adopt payload: %#v", payload)
	}
	if _, err := os.Stat(filepath.Join(env.profileDir, ".skillset", "state.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write state, err=%v", err)
	}
}

func TestAdoptApplyAndManagedJSON(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "adopt", "--apply")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	var adoptPayload struct {
		DryRun  bool `json:"dry_run"`
		Summary struct {
			Written int `json:"written"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &adoptPayload); err != nil {
		t.Fatalf("unmarshal adopt apply: %v", err)
	}
	if adoptPayload.DryRun || adoptPayload.Summary.Written != 3 {
		t.Fatalf("unexpected adopt apply payload: %#v", adoptPayload)
	}

	code, stdout, stderr = runSkillset(t, "--profile", env.profilePath, "--json", "managed")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	var managedPayload struct {
		Count   int `json:"count"`
		Managed []struct {
			Name       string `json:"name"`
			TargetKind string `json:"target_kind"`
		} `json:"managed"`
	}
	if err := json.Unmarshal([]byte(stdout), &managedPayload); err != nil {
		t.Fatalf("unmarshal managed: %v", err)
	}
	if managedPayload.Count != 3 {
		t.Fatalf("expected three managed entries, got %#v", managedPayload)
	}
}

func TestApplyDryRunDoesNotCreateLocalSymlink(t *testing.T) {
	env := newMissingLocalCLIEnv(t)
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "apply")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	var payload struct {
		DryRun  bool `json:"dry_run"`
		Summary struct {
			Planned int `json:"planned"`
			Applied int `json:"applied"`
			Written int `json:"written"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal apply dry-run: %v", err)
	}
	if !payload.DryRun || payload.Summary.Planned != 1 || payload.Summary.Applied != 0 || payload.Summary.Written != 0 {
		t.Fatalf("unexpected apply dry-run payload: %#v", payload)
	}
	if _, err := os.Lstat(filepath.Join(env.home, ".agents", "skills", "skill-builder")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create target, err=%v", err)
	}
}

func TestApplyApplyCreatesLocalSymlinkAndManagedState(t *testing.T) {
	env := newMissingLocalCLIEnv(t)
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "apply", "--apply")
	if code != appctx.ExitSuccess {
		t.Fatalf("expected success, got %d (stderr=%q stdout=%q)", code, stderr, stdout)
	}
	var payload struct {
		DryRun  bool `json:"dry_run"`
		Summary struct {
			Applied int `json:"applied"`
			Written int `json:"written"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal apply: %v", err)
	}
	if payload.DryRun || payload.Summary.Applied != 1 || payload.Summary.Written != 1 {
		t.Fatalf("unexpected apply payload: %#v", payload)
	}
	target := filepath.Join(env.home, ".agents", "skills", "skill-builder")
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink target, info=%v err=%v", info, err)
	}
}

func TestPruneDryRunAndApplyManagedState(t *testing.T) {
	env := newCLIEnv(t, installedProfile)
	code, stdout, stderr := runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "adopt", "--apply")
	if code != appctx.ExitSuccess {
		t.Fatalf("adopt apply failed: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	if err := os.WriteFile(env.profilePath, []byte(profileWithoutSkillBuilder), 0o644); err != nil {
		t.Fatalf("rewrite profile: %v", err)
	}

	code, stdout, stderr = runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "prune")
	if code != appctx.ExitSuccess {
		t.Fatalf("prune dry-run failed: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	var dryRun struct {
		DryRun  bool `json:"dry_run"`
		Summary struct {
			Planned int `json:"planned"`
			Deleted int `json:"deleted"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &dryRun); err != nil {
		t.Fatalf("unmarshal prune dry-run: %v", err)
	}
	if !dryRun.DryRun || dryRun.Summary.Planned != 2 || dryRun.Summary.Deleted != 0 {
		t.Fatalf("unexpected prune dry-run payload: %#v", dryRun)
	}
	if _, err := os.Lstat(filepath.Join(env.home, ".agents", "skills", "skill-builder")); err != nil {
		t.Fatalf("dry-run should not delete codex skill-builder: %v", err)
	}

	code, stdout, stderr = runSkillset(t, "--profile", env.profilePath, "--home", env.home, "--repo", env.repo, "--json", "prune", "--apply")
	if code != appctx.ExitSuccess {
		t.Fatalf("prune apply failed: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	var applied struct {
		Summary struct {
			Deleted int `json:"deleted"`
			Written int `json:"written"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &applied); err != nil {
		t.Fatalf("unmarshal prune apply: %v", err)
	}
	if applied.Summary.Deleted != 2 || applied.Summary.Written != 2 {
		t.Fatalf("unexpected prune apply payload: %#v", applied)
	}
	if _, err := os.Lstat(filepath.Join(env.home, ".agents", "skills", "skill-builder")); !os.IsNotExist(err) {
		t.Fatalf("expected codex skill-builder deleted, err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(env.home, ".claude", "skills", "skill-builder")); !os.IsNotExist(err) {
		t.Fatalf("expected claude skill-builder deleted, err=%v", err)
	}
}

const validProfile = `
schema_version: 1
skills:
  - name: skill-builder
    tier: user
    owner: first_party
    source: local:agent-repo-kit//skills/skill-builder
    agents:
      - codex
      - claude-code
  - name: browser-use
    tier: system
    owner: system
    source: system:codex/browser-use
`

const installedProfile = `
schema_version: 1
skills:
  - name: opencli-browser
    tier: user
    owner: upstream
    source: github:jackwener/opencli//skills/opencli-browser
    agents:
      - codex
  - name: skill-builder
    tier: user
    owner: first_party
    source: local:.//sources/skill-builder
    agents:
      - codex
      - claude-code
  - name: browser-use
    tier: system
    owner: system
    source: system:codex/browser-use
`

const missingLocalProfile = `
schema_version: 1
skills:
  - name: skill-builder
    tier: user
    owner: first_party
    source: local:.//sources/skill-builder
    agents:
      - codex
`

const profileWithoutSkillBuilder = `
schema_version: 1
skills:
  - name: opencli-browser
    tier: user
    owner: upstream
    source: github:jackwener/opencli//skills/opencli-browser
    agents:
      - codex
  - name: browser-use
    tier: system
    owner: system
    source: system:codex/browser-use
`

type cliEnv struct {
	root        string
	home        string
	repo        string
	profileDir  string
	profilePath string
}

func newCLIEnv(t *testing.T, profileBody string) cliEnv {
	t.Helper()
	root := t.TempDir()
	env := cliEnv{
		root:        root,
		home:        filepath.Join(root, "home"),
		repo:        filepath.Join(root, "repo"),
		profileDir:  filepath.Join(root, "profile"),
		profilePath: filepath.Join(root, "profile", "skills.profile.yaml"),
	}
	for _, dir := range []string{
		filepath.Join(env.home, ".agents", "skills", "opencli-browser"),
		filepath.Join(env.home, ".claude", "skills"),
		filepath.Join(env.repo, ".codex", "skills"),
		filepath.Join(env.repo, ".claude", "skills"),
		filepath.Join(env.profileDir, "sources", "skill-builder"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	writeCLISkill(t, filepath.Join(env.home, ".agents", "skills", "opencli-browser"), "opencli-browser")
	writeCLILock(t, filepath.Join(env.home, ".agents", ".skill-lock.json"))
	writeCLISkill(t, filepath.Join(env.profileDir, "sources", "skill-builder"), "skill-builder")
	if err := os.WriteFile(env.profilePath, []byte(profileBody), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	source := filepath.Join(env.profileDir, "sources", "skill-builder")
	for _, target := range []string{
		filepath.Join(env.home, ".agents", "skills", "skill-builder"),
		filepath.Join(env.home, ".claude", "skills", "skill-builder"),
	} {
		if err := os.Symlink(source, target); err != nil {
			t.Fatalf("symlink %s -> %s: %v", target, source, err)
		}
	}
	return env
}

func newMissingLocalCLIEnv(t *testing.T) cliEnv {
	t.Helper()
	root := t.TempDir()
	env := cliEnv{
		root:        root,
		home:        filepath.Join(root, "home"),
		repo:        filepath.Join(root, "repo"),
		profileDir:  filepath.Join(root, "profile"),
		profilePath: filepath.Join(root, "profile", "skills.profile.yaml"),
	}
	for _, dir := range []string{
		filepath.Join(env.home, ".agents", "skills"),
		filepath.Join(env.repo, ".codex", "skills"),
		filepath.Join(env.repo, ".claude", "skills"),
		filepath.Join(env.profileDir, "sources", "skill-builder"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	writeCLISkill(t, filepath.Join(env.profileDir, "sources", "skill-builder"), "skill-builder")
	if err := os.WriteFile(env.profilePath, []byte(missingLocalProfile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return env
}

func writeCLISkill(t *testing.T, dir, name string) {
	t.Helper()
	body := "---\nname: " + name + "\ndescription: Fixture skill.\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md for %s: %v", name, err)
	}
}

func writeCLILock(t *testing.T, path string) {
	t.Helper()
	body := `{"skills":{"opencli-browser":{"source":"jackwener/opencli","sourceType":"github","skillPath":"skills/opencli-browser/SKILL.md"}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
}

func writeCLIProfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "skills.profile.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile fixture: %v", err)
	}
	return path
}
