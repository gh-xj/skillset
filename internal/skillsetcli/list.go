package skillsetcli

import (
	"fmt"
	"strings"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/profile"
)

type ListCmd struct {
	Agent string `help:"filter by agent: codex or claude-code"`
	Tier  string `help:"filter by tier: system, user, or repo"`
}

type listSkill struct {
	Name         string   `json:"name"`
	Tier         string   `json:"tier"`
	Owner        string   `json:"owner"`
	Source       string   `json:"source"`
	SourceScheme string   `json:"source_scheme"`
	Agents       []string `json:"agents"`
}

func (c *ListCmd) Run(globals *CLI) error {
	p, path, err := globals.loadProfile()
	if err != nil {
		if globals.JSON {
			result := profileError(err)
			if writeErr := emitJSON(globals.stdout(), validationPayload(path, result)); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	result := p.Validate()
	if !result.Valid {
		if globals.JSON {
			if err := emitJSON(globals.stdout(), validationPayload(path, result)); err != nil {
				return err
			}
		} else if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}

	skills, err := c.filteredSkills(p)
	if err != nil {
		if globals.JSON {
			result := profile.ValidationResult{
				Valid:  false,
				Errors: []profile.Diagnostic{{Path: "list", Message: err.Error()}},
			}
			if writeErr := emitJSON(globals.stdout(), validationPayload(path, result)); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	if globals.JSON {
		return emitJSON(globals.stdout(), map[string]any{
			"profile_path": path,
			"count":        len(skills),
			"skills":       skills,
		})
	}
	if len(skills) == 0 {
		_, err := fmt.Fprintln(globals.stdout(), "no skills")
		return err
	}
	for _, skill := range skills {
		if _, err := fmt.Fprintf(globals.stdout(), "%s\t%s\t%s\t%s\t%s\n", skill.Name, skill.Tier, skill.Owner, strings.Join(skill.Agents, ","), skill.Source); err != nil {
			return err
		}
	}
	return nil
}

func (c *ListCmd) filteredSkills(p profile.Profile) ([]listSkill, error) {
	if c.Agent != "" && c.Agent != string(profile.AgentCodex) && c.Agent != string(profile.AgentClaudeCode) {
		return nil, fmt.Errorf("agent must be one of: codex, claude-code")
	}
	if c.Tier != "" && c.Tier != string(profile.TierSystem) && c.Tier != string(profile.TierUser) && c.Tier != string(profile.TierRepo) {
		return nil, fmt.Errorf("tier must be one of: system, user, repo")
	}
	var out []listSkill
	for _, skill := range p.Skills {
		if c.Tier != "" && string(skill.Tier) != c.Tier {
			continue
		}
		if c.Agent != "" && !hasAgent(skill.Agents, profile.Agent(c.Agent)) {
			continue
		}
		source, err := profile.ParseSource(skill.Source)
		if err != nil {
			return nil, err
		}
		out = append(out, listSkill{
			Name:         skill.Name,
			Tier:         string(skill.Tier),
			Owner:        string(skill.Owner),
			Source:       skill.Source,
			SourceScheme: string(source.Scheme),
			Agents:       agentStrings(skill.Agents),
		})
	}
	return out, nil
}

func hasAgent(agents []profile.Agent, needle profile.Agent) bool {
	for _, agent := range agents {
		if agent == needle {
			return true
		}
	}
	return false
}
