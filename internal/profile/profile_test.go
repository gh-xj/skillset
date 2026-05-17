package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNormalizeValidateProfile(t *testing.T) {
	path := writeProfile(t, `
schema_version: 1
skills:
  - name: " skill-builder "
    tier: " USER "
    owner: first_party
    source: "local:agent-repo-kit//skills/skill-builder "
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
	if got := normalized.Skills[0].Name; got != "browser-use" {
		t.Fatalf("expected skills sorted by name, got %q", got)
	}
	if got := normalized.Skills[1].Tier; got != TierUser {
		t.Fatalf("expected normalized tier user, got %q", got)
	}
	if len(normalized.Skills[1].Agents) != 2 {
		t.Fatalf("expected duplicate agents removed, got %#v", normalized.Skills[1].Agents)
	}
	if got := normalized.Skills[0].Agents; len(got) != 1 || got[0] != AgentCodex {
		t.Fatalf("expected system source agent inferred, got %#v", got)
	}
	if result := normalized.Validate(); !result.Valid {
		t.Fatalf("expected normalized profile to validate, got %#v", result.Errors)
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
