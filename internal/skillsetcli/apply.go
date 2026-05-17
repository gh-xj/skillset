package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/appctx"
	applyop "github.com/gh-xj/skillset/internal/apply"
)

type ApplyCmd struct {
	Apply bool `help:"create missing skills and record managed state"`
}

var applyRunner applyop.Runner

func (c *ApplyCmd) Run(globals *CLI) error {
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
	applied, err := applyop.Run(plan, applyop.Options{
		Apply:       c.Apply,
		ProfilePath: globals.profilePath(),
		ToolName:    binaryName,
		Runner:      applyRunner,
	})
	if err != nil {
		if globals.JSON {
			if writeErr := emitJSON(globals.stdout(), map[string]any{
				"ok":     false,
				"errors": []map[string]string{{"path": "apply", "message": err.Error()}},
			}); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	ok := len(applied.Failed) == 0
	if globals.JSON {
		if err := emitJSON(globals.stdout(), map[string]any{
			"ok":           ok,
			"dry_run":      applied.DryRun,
			"profile_path": applied.ProfilePath,
			"state_path":   applied.StatePath,
			"events_path":  applied.EventsPath,
			"summary":      applied.Summary,
			"planned":      applied.Planned,
			"applied":      applied.Applied,
			"skipped":      applied.Skipped,
			"failed":       applied.Failed,
		}); err != nil {
			return err
		}
		if !ok {
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return nil
	}
	mode := "dry-run"
	if c.Apply {
		mode = "applied"
	}
	if _, err := fmt.Fprintf(globals.stdout(), "apply %s: planned=%d applied=%d skipped=%d failed=%d written=%d\n", mode, applied.Summary.Planned, applied.Summary.Applied, applied.Summary.Skipped, applied.Summary.Failed, applied.Summary.Written); err != nil {
		return err
	}
	if !ok {
		return appctx.NewExitError(appctx.ExitError, "")
	}
	return nil
}
