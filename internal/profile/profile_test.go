package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNormalizeValidateProfile(t *testing.T) {
	path := writeProfile(t, `
schema_version: 1
roots:
  " ark ": " ./agent-repo-kit/skills "
skills:
  - name: " skill-builder "
    tier: " USER "
    owner: first_party
    source: "local:ark//skill-builder "
    agents:
      - codex
      - CODEX
      - claude-code
  - name: browser-use
    tier: system
    owner: system
    source: system:codex/browser-use
`)

	p, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	normalized := p.Normalized()
	if got := normalized.Skills[0].Name; got != "skill-builder" {
		t.Fatalf("expected profile order to be preserved, got first skill %q", got)
	}
	if got := normalized.Skills[0].Tier; got != TierUser {
		t.Fatalf("expected normalized tier user, got %q", got)
	}
	if got := normalized.Roots["ark"]; got != "agent-repo-kit/skills" {
		t.Fatalf("expected normalized root path, got %q", got)
	}
	if len(normalized.Skills[0].Agents) != 2 {
		t.Fatalf("expected duplicate agents removed, got %#v", normalized.Skills[0].Agents)
	}
	if got := normalized.Skills[1].Agents; len(got) != 1 || got[0] != AgentCodex {
		t.Fatalf("expected system source agent inferred, got %#v", got)
	}
	if result := normalized.Validate(); !result.Valid {
		t.Fatalf("expected normalized profile to validate, got %#v", result.Errors)
	}
}

func TestValidateRoots(t *testing.T) {
	p := Profile{
		SchemaVersion: CurrentSchemaVersion,
		Roots: map[string]string{
			"ark":      "../agent-repo-kit/skills",
			"ark_copy": "../agent-repo-kit/skills/../skills",
			"Bad":      "/tmp/skills",
			"empty":    "",
		},
		Skills: []Skill{
			{Name: "skill-builder", Tier: TierUser, Owner: OwnerFirstParty, Source: "local:ark//skill-builder", Agents: []Agent{AgentCodex}},
		},
	}

	result := p.Validate()
	if result.Valid {
		t.Fatalf("expected invalid roots to fail validation")
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected invalid name and empty path errors, got %#v", result.Errors)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("expected duplicate normalized path and absolute path warnings, got %#v", result.Warnings)
	}
}

func TestValidateRootsWarnsForProfileRelativeAndAbsoluteDuplicate(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "profile")
	sourceRoot := filepath.Join(root, "shared", "skills")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir: %v", err)
	}
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("mkdir source root: %v", err)
	}
	p := Profile{
		SchemaVersion: CurrentSchemaVersion,
		Roots: map[string]string{
			"absolute": sourceRoot,
			"relative": "../shared/skills",
		},
		Skills: []Skill{},
	}

	result := p.ValidateForProfile(filepath.Join(profileDir, "skills.profile.yaml"))
	if !result.Valid {
		t.Fatalf("expected valid roots, got %#v", result.Errors)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("expected absolute path and duplicate real path warnings, got %#v", result.Warnings)
	}
}

func TestValidateRejectsDuplicateAgentTierName(t *testing.T) {
	p := Profile{
		SchemaVersion: CurrentSchemaVersion,
		Skills: []Skill{
			{Name: "dup", Tier: TierUser, Owner: OwnerUpstream, Source: "github:one/repo//skills/dup", Agents: []Agent{AgentCodex}},
			{Name: "dup", Tier: TierUser, Owner: OwnerPrivate, Source: "local:private//skills/dup", Agents: []Agent{AgentCodex}},
		},
	}
	result := p.Validate()
	if result.Valid {
		t.Fatalf("expected duplicate profile to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected duplicate error")
	}
}

func TestParseSourceSchemes(t *testing.T) {
	cases := map[string]SourceScheme{
		"github:gh-xj/agent-repo-kit//skills/skill-builder": SourceGitHub,
		"local:agent-repo-kit//skills/skill-builder":        SourceLocal,
		"system:codex/browser-use":                          SourceSystem,
	}
	for raw, want := range cases {
		source, err := ParseSource(raw)
		if err != nil {
			t.Fatalf("ParseSource(%q) error = %v", raw, err)
		}
		if source.Scheme != want {
			t.Fatalf("ParseSource(%q) scheme = %q, want %q", raw, source.Scheme, want)
		}
	}
}

func TestParseSourcePopulatesTypedSourceData(t *testing.T) {
	github, err := ParseSource("github:gh-xj/agent-repo-kit//skills/skill-builder")
	if err != nil {
		t.Fatalf("ParseSource(github) error = %v", err)
	}
	if github.GitHub == nil || github.GitHub.Owner != "gh-xj" || github.GitHub.Repo != "agent-repo-kit" || github.GitHub.SkillDir != "skills/skill-builder" {
		t.Fatalf("unexpected github source data: %#v", github)
	}
	local, err := ParseSource("local:ark//skill-builder")
	if err != nil {
		t.Fatalf("ParseSource(local) error = %v", err)
	}
	if local.Local == nil || local.Local.Root != "ark" || local.Local.SkillDir != "skill-builder" {
		t.Fatalf("unexpected local source data: %#v", local)
	}
	system, err := ParseSource("system:codex/browser-use")
	if err != nil {
		t.Fatalf("ParseSource(system) error = %v", err)
	}
	if system.System == nil || system.System.Agent != AgentCodex || system.System.Skill != "browser-use" {
		t.Fatalf("unexpected system source data: %#v", system)
	}
}

func TestValidateRejectsIncoherentOwnerSourcePairs(t *testing.T) {
	p := Profile{
		SchemaVersion: CurrentSchemaVersion,
		Skills: []Skill{
			{Name: "private-gh", Tier: TierUser, Owner: OwnerPrivate, Source: "github:owner/repo//skills/private-gh", Agents: []Agent{AgentCodex}},
			{Name: "repo-gh", Tier: TierRepo, Owner: OwnerRepo, Source: "github:owner/repo//skills/repo-gh", Agents: []Agent{AgentCodex}},
			{Name: "system-owner", Tier: TierUser, Owner: OwnerSystem, Source: "local:.//skills/system-owner", Agents: []Agent{AgentCodex}},
		},
	}
	result := p.Validate()
	if result.Valid {
		t.Fatalf("expected incoherent owner/source pairs to be invalid")
	}
	if len(result.Errors) != 4 {
		t.Fatalf("expected four validation errors, got %#v", result.Errors)
	}
}

func TestParseSourceRejectsGitHubRefPinning(t *testing.T) {
	if _, err := ParseSource("github:owner/repo@v1//skills/foo"); err == nil {
		t.Fatalf("expected ref pinning to be rejected")
	}
}

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "skills.profile.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile fixture: %v", err)
	}
	return path
}
