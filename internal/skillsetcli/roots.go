package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/planner"
)

type RootsCmd struct {
	Agent string `help:"filter by agent: codex or claude-code"`
	Tier  string `help:"filter by tier: system, user, or repo"`
}

func (c *RootsCmd) Run(globals *CLI) error {
	roots, err := planner.Roots(globals.plannerOptions())
	if err != nil {
		if globals.JSON {
			if writeErr := emitCommandErrorJSON(globals.stdout(), "roots", "roots", err.Error()); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	filtered, err := c.filteredRoots(roots)
	if err != nil {
		if globals.JSON {
			if writeErr := emitCommandErrorJSON(globals.stdout(), "roots", "roots", err.Error()); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	if globals.JSON {
		return emitCommandJSON(globals.stdout(), "roots", true, "", map[string]any{"count": len(filtered)}, map[string]any{
			"roots": filtered,
		}, nil, nil, map[string]any{
			"ok":    true,
			"count": len(filtered),
			"roots": filtered,
		})
	}
	for _, root := range filtered {
		path := root.Path
		if path == "" {
			path = "-"
		}
		if _, err := fmt.Fprintf(globals.stdout(), "%s\t%s\t%s\t%s\texists=%t\n", root.Agent, root.Tier, root.Mode, path, root.Exists); err != nil {
			return err
		}
	}
	return nil
}

func (c *RootsCmd) filteredRoots(roots []planner.Root) ([]planner.Root, error) {
	if c.Agent != "" && c.Agent != "codex" && c.Agent != "claude-code" {
		return nil, fmt.Errorf("agent must be one of: codex, claude-code")
	}
	if c.Tier != "" && c.Tier != "system" && c.Tier != "user" && c.Tier != "repo" {
		return nil, fmt.Errorf("tier must be one of: system, user, repo")
	}
	out := make([]planner.Root, 0, len(roots))
	for _, root := range roots {
		if c.Agent != "" && string(root.Agent) != c.Agent {
			continue
		}
		if c.Tier != "" && string(root.Tier) != c.Tier {
			continue
		}
		out = append(out, root)
	}
	return out, nil
}
