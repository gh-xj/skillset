package profile

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentSchemaVersion = 1

type Tier string

const (
	TierSystem Tier = "system"
	TierUser   Tier = "user"
	TierRepo   Tier = "repo"
)

type Owner string

const (
	OwnerUpstream   Owner = "upstream"
	OwnerFirstParty Owner = "first_party"
	OwnerPrivate    Owner = "private"
	OwnerRepo       Owner = "repo"
	OwnerSystem     Owner = "system"
)

type Agent string

const (
	AgentCodex      Agent = "codex"
	AgentClaudeCode Agent = "claude-code"
)

type SourceScheme string

const (
	SourceGitHub SourceScheme = "github"
	SourceLocal  SourceScheme = "local"
	SourceSystem SourceScheme = "system"
)

type Profile struct {
	SchemaVersion int     `yaml:"schema_version" json:"schema_version"`
	Skills        []Skill `yaml:"skills" json:"skills"`
}

type Skill struct {
	Name   string  `yaml:"name" json:"name"`
	Tier   Tier    `yaml:"tier" json:"tier"`
	Owner  Owner   `yaml:"owner" json:"owner"`
	Source string  `yaml:"source" json:"source"`
	Agents []Agent `yaml:"agents,omitempty" json:"agents,omitempty"`
}

type Source struct {
	Scheme   SourceScheme `json:"scheme"`
	Raw      string       `json:"raw"`
	Owner    string       `json:"owner,omitempty"`
	Repo     string       `json:"repo,omitempty"`
	Root     string       `json:"root,omitempty"`
	SkillDir string       `json:"skill_dir,omitempty"`
	Agent    Agent        `json:"agent,omitempty"`
	Skill    string       `json:"skill,omitempty"`
}

type Diagnostic struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid    bool         `json:"valid"`
	Errors   []Diagnostic `json:"errors"`
	Warnings []Diagnostic `json:"warnings"`
}

func LoadFile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile %s: %w", path, err)
	}
	var p Profile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("decode profile %s: %w", path, err)
	}
	return p, nil
}

func (p Profile) Normalized() Profile {
	out := Profile{
		SchemaVersion: p.SchemaVersion,
		Skills:        make([]Skill, 0, len(p.Skills)),
	}
	for _, skill := range p.Skills {
		normalized := Skill{
			Name:   strings.TrimSpace(skill.Name),
			Tier:   Tier(strings.ToLower(strings.TrimSpace(string(skill.Tier)))),
			Owner:  Owner(strings.ToLower(strings.TrimSpace(string(skill.Owner)))),
			Source: strings.TrimSpace(skill.Source),
			Agents: normalizeAgents(skill.Agents),
		}
		if len(normalized.Agents) == 0 {
			if source, err := ParseSource(normalized.Source); err == nil && source.Scheme == SourceSystem && source.Agent != "" {
				normalized.Agents = []Agent{source.Agent}
			}
		}
		out.Skills = append(out.Skills, normalized)
	}
	return out
}

func (p Profile) Validate() ValidationResult {
	var result ValidationResult
	if p.SchemaVersion != CurrentSchemaVersion {
		result.addError("schema_version", fmt.Sprintf("must be %d", CurrentSchemaVersion))
	}

	seen := map[string]int{}
	for i, skill := range p.Skills {
		base := fmt.Sprintf("skills[%d]", i)
		if skill.Name == "" {
			result.addError(base+".name", "is required")
		}
		if !validTier(skill.Tier) {
			result.addError(base+".tier", "must be one of: system, user, repo")
		}
		if !validOwner(skill.Owner) {
			result.addError(base+".owner", "must be one of: upstream, first_party, private, repo, system")
		}
		source, sourceErr := ParseSource(skill.Source)
		if sourceErr != nil {
			result.addError(base+".source", sourceErr.Error())
		}
		if len(skill.Agents) == 0 {
			result.addError(base+".agents", "must include at least one agent")
		}
		for j, agent := range skill.Agents {
			if !validAgent(agent) {
				result.addError(fmt.Sprintf("%s.agents[%d]", base, j), "must be one of: codex, claude-code")
				continue
			}
			key := strings.Join([]string{string(agent), string(skill.Tier), skill.Name}, "\x00")
			if previous, ok := seen[key]; ok && skill.Name != "" && skill.Tier != "" {
				result.addError(base, fmt.Sprintf("duplicates skills[%d] for agent %q, tier %q, and name %q", previous, agent, skill.Tier, skill.Name))
			} else {
				seen[key] = i
			}
		}
		if sourceErr == nil {
			validateSourceCoherence(&result, base, skill, source)
		}
	}

	result.Valid = len(result.Errors) == 0
	return result
}

func ParseSource(raw string) (Source, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Source{}, fmt.Errorf("is required")
	}
	scheme, body, ok := strings.Cut(raw, ":")
	if !ok || body == "" {
		return Source{}, fmt.Errorf("must use github:, local:, or system: source scheme")
	}
	source := Source{Scheme: SourceScheme(strings.ToLower(scheme)), Raw: raw}
	switch source.Scheme {
	case SourceGitHub:
		repoPart, skillDir, ok := strings.Cut(body, "//")
		if !ok {
			return Source{}, fmt.Errorf("github source must be github:<owner>/<repo>//<skill-dir>")
		}
		owner, repo, ok := strings.Cut(repoPart, "/")
		if !ok || owner == "" || repo == "" || skillDir == "" {
			return Source{}, fmt.Errorf("github source must be github:<owner>/<repo>//<skill-dir>")
		}
		if strings.Contains(repo, "@") {
			return Source{}, fmt.Errorf("github ref pinning is not supported in v1")
		}
		source.Owner = owner
		source.Repo = repo
		source.SkillDir = strings.Trim(skillDir, "/")
	case SourceLocal:
		root, skillDir, ok := strings.Cut(body, "//")
		if !ok || root == "" || skillDir == "" {
			return Source{}, fmt.Errorf("local source must be local:<root>//<skill-dir>")
		}
		source.Root = root
		source.SkillDir = strings.Trim(skillDir, "/")
	case SourceSystem:
		agent, skill, ok := strings.Cut(body, "/")
		if !ok || agent == "" || skill == "" {
			return Source{}, fmt.Errorf("system source must be system:<agent>/<skill>")
		}
		source.Agent = Agent(strings.ToLower(agent))
		source.Skill = strings.Trim(skill, "/")
	default:
		return Source{}, fmt.Errorf("must use github:, local:, or system: source scheme")
	}
	if source.Scheme != SourceSystem && source.SkillDir == "" {
		return Source{}, fmt.Errorf("%s source skill directory is required", source.Scheme)
	}
	if source.Scheme == SourceSystem && source.Skill == "" {
		return Source{}, fmt.Errorf("system source skill is required")
	}
	return source, nil
}

func normalizeAgents(agents []Agent) []Agent {
	seen := map[Agent]struct{}{}
	out := make([]Agent, 0, len(agents))
	for _, agent := range agents {
		normalized := Agent(strings.ToLower(strings.TrimSpace(string(agent))))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	slices.Sort(out)
	return out
}

func validateSourceCoherence(result *ValidationResult, base string, skill Skill, source Source) {
	if source.Scheme == SourceSystem {
		if skill.Tier != TierSystem {
			result.addError(base+".tier", "must be system when source uses system:")
		}
		if skill.Owner != OwnerSystem {
			result.addError(base+".owner", "must be system when source uses system:")
		}
		for j, agent := range skill.Agents {
			if agent != source.Agent {
				result.addError(fmt.Sprintf("%s.agents[%d]", base, j), fmt.Sprintf("must match system source agent %q", source.Agent))
			}
		}
	}
	if skill.Tier == TierSystem && source.Scheme != SourceSystem {
		result.addError(base+".source", "must use system: when tier is system")
	}
	if skill.Owner == OwnerSystem && source.Scheme != SourceSystem {
		result.addError(base+".source", "must use system: when owner is system")
	}
}

func validTier(tier Tier) bool {
	switch tier {
	case TierSystem, TierUser, TierRepo:
		return true
	default:
		return false
	}
}

func validOwner(owner Owner) bool {
	switch owner {
	case OwnerUpstream, OwnerFirstParty, OwnerPrivate, OwnerRepo, OwnerSystem:
		return true
	default:
		return false
	}
}

func validAgent(agent Agent) bool {
	switch agent {
	case AgentCodex, AgentClaudeCode:
		return true
	default:
		return false
	}
}

func (r *ValidationResult) addError(path, message string) {
	r.Errors = append(r.Errors, Diagnostic{Path: path, Message: message})
}
