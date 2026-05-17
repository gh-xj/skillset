package skillsetcli

import (
	"github.com/gh-xj/skillset/internal/appctx"
)

type DiffCmd struct{}

func (c *DiffCmd) Run(globals *CLI) error {
	plan, result, _ := globals.buildPlan()
	if !result.Valid {
		if globals.JSON {
			if err := emitValidationCommandJSON(globals.stdout(), "diff", globals.profilePath(), result); err != nil {
				return err
			}
		} else if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	creates := plan.Creates()
	if globals.JSON {
		errors := plan.Errors()
		ignored := plan.Ignored()
		ok := len(errors) == 0
		return emitCommandJSON(globals.stdout(), "diff", ok, plan.ProfilePath, plan.Summary, map[string]any{
			"creates": creates,
			"errors":  errors,
			"ignored": ignored,
			"items":   plan.Items,
		}, result.Warnings, errors, map[string]any{
			"profile_path": plan.ProfilePath,
			"summary":      plan.Summary,
			"creates":      creates,
			"errors":       errors,
			"ignored":      ignored,
			"changes":      creates,
			"items":        plan.Items,
		})
	}
	return printPlanItems(globals.stdout(), creates, "no changes")
}
