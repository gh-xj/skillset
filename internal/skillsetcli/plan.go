package skillsetcli

import (
	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
)

func (c *CLI) plannerOptions() planner.Options {
	return planner.Options{
		ProfilePath: c.profilePath(),
		HomeDir:     c.Home,
		RepoDir:     c.Repo,
	}
}

func (c *CLI) buildPlan() (planner.Plan, profile.ValidationResult, error) {
	p, _, err := c.loadProfile()
	if err != nil {
		return planner.Plan{}, profileError(err), err
	}
	result := p.Validate()
	if !result.Valid {
		return planner.Plan{}, result, nil
	}
	plan, err := planner.Build(p, c.plannerOptions())
	if err != nil {
		return planner.Plan{}, profileError(err), err
	}
	return plan, result, nil
}
