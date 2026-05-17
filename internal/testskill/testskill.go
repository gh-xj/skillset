package testskill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteSkill(path, name string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`---
name: %s
description: Use when testing the %s skill fixture.
---

# %s

Fixture skill used by tests.
`, name, name, name)
	return os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(body), 0o644)
}

func WriteGitHubLock(skillRoot, name, source, skillPath string) error {
	lock := map[string]any{
		"version": 3,
		"skills": map[string]any{
			name: map[string]any{
				"source":          source,
				"sourceType":      "github",
				"sourceUrl":       "https://github.com/" + source + ".git",
				"skillPath":       filepath.ToSlash(skillPath),
				"skillFolderHash": "hash",
				"installedAt":     "2026-04-24T04:35:02.535Z",
				"updatedAt":       "2026-05-17T01:26:16.214Z",
			},
		},
	}
	data, err := json.Marshal(lock)
	if err != nil {
		return err
	}
	lockPath := filepath.Join(filepath.Dir(skillRoot), ".skill-lock.json")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(lockPath, data, 0o644)
}
