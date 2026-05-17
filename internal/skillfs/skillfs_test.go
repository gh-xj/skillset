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

func TestInspectSkillDirAndPath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	writeSkillFile(t, source, "---\nname: example\ndescription: Fixture skill.\n---\n\n# example\n")
	if observation := InspectSkillDir(source, "example"); !observation.Exists || !observation.Valid {
		t.Fatalf("expected valid skill dir observation, got %#v", observation)
	}
	target := filepath.Join(root, "target")
	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("symlink target: %v", err)
	}
	observation, err := InspectPath(target)
	if err != nil {
		t.Fatalf("InspectPath() error = %v", err)
	}
	if !observation.Exists || observation.Kind != KindSymlink || observation.SymlinkTarget != source {
		t.Fatalf("unexpected path observation: %#v", observation)
	}
	missing, err := InspectPath(filepath.Join(root, "missing"))
	if err != nil {
		t.Fatalf("InspectPath(missing) error = %v", err)
	}
	if missing.Exists || missing.Kind != KindMissing {
		t.Fatalf("unexpected missing observation: %#v", missing)
	}
}

func TestValidatePathUnderRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "skills", "example")
	if err := ValidatePathUnderRoot(inside, root); err != nil {
		t.Fatalf("expected inside path to validate: %v", err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside")
	if err := ValidatePathUnderRoot(outside, root); err == nil {
		t.Fatalf("expected outside path to be rejected")
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
