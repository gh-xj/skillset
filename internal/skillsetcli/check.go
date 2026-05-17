package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/appctx"
)

type CheckCmd struct{}

func (c *CheckCmd) Run(globals *CLI) error {
	plan, result, _ := globals.buildPlan()
	if !result.Valid {
		if globals.JSON {
			if err := emitValidationCommandJSON(globals.stdout(), "check", globals.profilePath(), result); err != nil {
				return err
			}
		} else if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	ok := plan.Summary.Errors == 0
	if globals.JSON {
		errors := plan.ErrorItems()
		if err := emitCommandJSON(globals.stdout(), "check", ok, plan.ProfilePath, plan.Summary, map[string]any{
			"items": plan.Items,
		}, result.Warnings, errors, map[string]any{
			"ok":           ok,
			"profile_path": plan.ProfilePath,
			"summary":      plan.Summary,
			"errors":       errors,
			"items":        plan.Items,
		}); err != nil {
			return err
		}
		if !ok {
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return nil
	}
	if !ok {
		if err := printPlanItems(globals.stderr(), plan.ErrorItems(), "no skill state errors"); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	_, err := fmt.Fprintf(globals.stdout(), "profile and skill state ok: %s\n", plan.ProfilePath)
	return err
}
