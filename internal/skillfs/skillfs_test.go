package skillfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gh-xj/skillset/internal/profile"
)

func TestValidateSkillDirRequiresNameAndDescription(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "---\nname: example\n---\n\n# example\n")

	err := ValidateSkillDir(dir, "example")
	if err == nil || !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("expected missing description error, got %v", err)
	}
}

func TestValidateGitHubInstallRequiresMatchingLock(t *testing.T) {
	root := t.TempDir()
	skillRoot := filepath.Join(root, ".agents", "skills")
	target := filepath.Join(skillRoot, "opencli-browser")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	writeSkillFile(t, target, "---\nname: opencli-browser\ndescription: Fixture skill.\n---\n\n# opencli-browser\n")
	lock := `{"skills":{"opencli-browser":{"source":"jackwener/opencli","sourceType":"github","skillPath":"skills/opencli-browser/SKILL.md"}}}`
	if err := os.WriteFile(filepath.Join(root, ".agents", ".skill-lock.json"), []byte(lock), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	source, err := profile.ParseSource("github:jackwener/opencli//skills/opencli-browser")
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	if err := ValidateGitHubInstall(skillRoot, target, "opencli-browser", source); err != nil {
		t.Fatalf("ValidateGitHubInstall() error = %v", err)
	}
}

func writeSkillFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}
