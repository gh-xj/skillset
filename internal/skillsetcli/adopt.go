package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/adopt"
	"github.com/gh-xj/skillset/internal/appctx"
)

type AdoptCmd struct {
	Apply bool `help:"write .skillset/state.yaml and .skillset/events.ndjson"`
}

func (c *AdoptCmd) Run(globals *CLI) error {
	plan, result, _ := globals.buildPlan()
	if !result.Valid {
		if globals.JSON {
			if err := emitValidationCommandJSON(globals.stdout(), "adopt", globals.profilePath(), result); err != nil {
				return err
			}
		} else if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	adoption, err := adopt.Run(plan, adopt.Options{
		Apply:       c.Apply,
		ProfilePath: globals.profilePath(),
		ToolName:    binaryName,
	})
	if err != nil {
		if globals.JSON {
			if writeErr := emitCommandErrorJSON(globals.stdout(), "adopt", "adopt", err.Error()); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	if globals.JSON {
		return emitCommandJSON(globals.stdout(), "adopt", true, adoption.ProfilePath, adoption.Summary, map[string]any{
			"adopted": adoption.Adopted,
			"skipped": adoption.Skipped,
		}, nil, nil, map[string]any{
			"ok":           true,
			"dry_run":      adoption.DryRun,
			"profile_path": adoption.ProfilePath,
			"state_path":   adoption.StatePath,
			"events_path":  adoption.EventsPath,
			"summary":      adoption.Summary,
			"adopted":      adoption.Adopted,
			"skipped":      adoption.Skipped,
		})
	}
	mode := "dry-run"
	if c.Apply {
		mode = "applied"
	}
	if _, err := fmt.Fprintf(globals.stdout(), "adopt %s: adopted=%d skipped=%d written=%d\n", mode, adoption.Summary.Adopted, adoption.Summary.Skipped, adoption.Summary.Written); err != nil {
		return err
	}
	for _, entry := range adoption.Adopted {
		if _, err := fmt.Fprintf(globals.stdout(), "%s\t%s\t%s\t%s\n", entry.Agent, entry.Tier, entry.Name, entry.TargetPath); err != nil {
			return err
		}
	}
	return nil
}
