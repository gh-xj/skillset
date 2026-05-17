package skillsetcli

import (
	"github.com/gh-xj/skillset/internal/appctx"
)

type DiffCmd struct{}

func (c *DiffCmd) Run(globals *CLI) error {
	plan, result, _ := globals.buildPlan()
	if !result.Valid {
		if globals.JSON {
			if err := emitJSON(globals.stdout(), validationPayload(globals.profilePath(), result)); err != nil {
				return err
			}
		} else if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	changes := plan.Changes()
	if globals.JSON {
		return emitJSON(globals.stdout(), map[string]any{
			"profile_path": plan.ProfilePath,
			"summary":      plan.Summary,
			"changes":      changes,
			"items":        plan.Items,
		})
	}
	return printPlanItems(globals.stdout(), changes, "no changes")
}
