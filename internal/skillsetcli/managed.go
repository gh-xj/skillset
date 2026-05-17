package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/state"
)

type ManagedCmd struct{}

func (c *ManagedCmd) Run(globals *CLI) error {
	statePath := state.StatePathForProfile(globals.profilePath())
	store, err := state.Load(statePath)
	if err != nil {
		if globals.JSON {
			if writeErr := emitCommandErrorJSON(globals.stdout(), "managed", statePath, err.Error()); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	if globals.JSON {
		return emitCommandJSON(globals.stdout(), "managed", true, globals.profilePath(), map[string]any{"count": len(store.Managed)}, map[string]any{
			"managed": store.Managed,
		}, nil, nil, map[string]any{
			"ok":         true,
			"state_path": statePath,
			"count":      len(store.Managed),
			"managed":    store.Managed,
		})
	}
	if len(store.Managed) == 0 {
		_, err := fmt.Fprintln(globals.stdout(), "no managed entries")
		return err
	}
	for _, entry := range store.Managed {
		target := entry.TargetPath
		if target == "" {
			target = entry.TargetRel
		}
		if _, err := fmt.Fprintf(globals.stdout(), "%s\t%s\t%s\t%s\t%s\n", entry.Agent, entry.Tier, entry.Name, entry.SourceScheme, target); err != nil {
			return err
		}
	}
	return nil
}
