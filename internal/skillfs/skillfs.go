package skillfs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gh-xj/skillset/internal/profile"
	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type lockFile struct {
	Skills map[string]lockSkill `json:"skills"`
}

type lockSkill struct {
	Source     string `json:"source"`
	SourceType string `json:"sourceType"`
	SkillPath  string `json:"skillPath"`
}

func ValidateSkillDir(path, expectedName string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read skill directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("read SKILL.md: %w", err)
	}
	meta, err := parseFrontmatter(data)
	if err != nil {
		return err
	}
	if meta.Name == "" {
		return fmt.Errorf("SKILL.md frontmatter name is required")
	}
	if meta.Description == "" {
		return fmt.Errorf("SKILL.md frontmatter description is required")
	}
	if expectedName != "" && meta.Name != expectedName {
		return fmt.Errorf("SKILL.md frontmatter name %q does not match profile name %q", meta.Name, expectedName)
	}
	return nil
}

func ValidateGitHubInstall(skillRoot, targetPath, expectedName string, source profile.Source) error {
	if source.Scheme != profile.SourceGitHub {
		return fmt.Errorf("source is not github")
	}
	if err := ValidateSkillDir(targetPath, expectedName); err != nil {
		return err
	}
	lock, err := readLock(filepath.Join(filepath.Dir(skillRoot), ".skill-lock.json"))
	if err != nil {
		return err
	}
	locked, ok := lock[expectedName]
	if !ok {
		return fmt.Errorf("missing .skill-lock.json entry for %q", expectedName)
	}
	expectedSource := source.Owner + "/" + source.Repo
	if locked.SourceType != "github" {
		return fmt.Errorf("lock sourceType is %q, want github", locked.SourceType)
	}
	if locked.Source != expectedSource {
		return fmt.Errorf("lock source is %q, want %q", locked.Source, expectedSource)
	}
	expectedPath := filepath.ToSlash(filepath.Join(source.SkillDir, "SKILL.md"))
	if filepath.ToSlash(locked.SkillPath) != expectedPath {
		return fmt.Errorf("lock skillPath is %q, want %q", locked.SkillPath, expectedPath)
	}
	return nil
}

func parseFrontmatter(data []byte) (frontmatter, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, fmt.Errorf("SKILL.md YAML frontmatter is required")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{}, fmt.Errorf("SKILL.md YAML frontmatter is not closed")
	}
	var meta frontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
		return frontmatter{}, fmt.Errorf("decode SKILL.md frontmatter: %w", err)
	}
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	return meta, nil
}

func readLock(path string) (map[string]lockSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("read .skill-lock.json: %w", err)
		}
		return nil, fmt.Errorf("read lock %s: %w", path, err)
	}
	var lock lockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("decode lock %s: %w", path, err)
	}
	if lock.Skills == nil {
		return map[string]lockSkill{}, nil
	}
	return lock.Skills, nil
}
