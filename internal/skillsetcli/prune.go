package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/prune"
)

type PruneCmd struct {
	Apply bool `help:"delete managed entries no longer desired"`
}

func (c *PruneCmd) Run(globals *CLI) error {
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
	pruned, err := prune.Run(plan, prune.Options{
		Apply:       c.Apply,
		ProfilePath: globals.profilePath(),
	})
	if err != nil {
		if globals.JSON {
			if writeErr := emitJSON(globals.stdout(), map[string]any{
				"ok":     false,
				"errors": []map[string]string{{"path": "prune", "message": err.Error()}},
			}); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	ok := len(pruned.Failed) == 0
	if globals.JSON {
		if err := emitJSON(globals.stdout(), map[string]any{
			"ok":           ok,
			"dry_run":      pruned.DryRun,
			"profile_path": pruned.ProfilePath,
			"state_path":   pruned.StatePath,
			"events_path":  pruned.EventsPath,
			"summary":      pruned.Summary,
			"planned":      pruned.Planned,
			"deleted":      pruned.Deleted,
			"skipped":      pruned.Skipped,
			"failed":       pruned.Failed,
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
	if _, err := fmt.Fprintf(globals.stdout(), "prune %s: planned=%d deleted=%d skipped=%d failed=%d written=%d\n", mode, pruned.Summary.Planned, pruned.Summary.Deleted, pruned.Summary.Skipped, pruned.Summary.Failed, pruned.Summary.Written); err != nil {
		return err
	}
	if !ok {
		return appctx.NewExitError(appctx.ExitError, "")
	}
	return nil
}
