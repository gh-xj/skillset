package skillsetcli

import (
	"fmt"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/profile"
)

type ValidateCmd struct{}

func (c *ValidateCmd) Run(globals *CLI) error {
	p, path, err := globals.loadProfile()
	if err != nil {
		if globals.JSON {
			result := profile.ValidationResult{
				Valid:  false,
				Errors: []profile.Diagnostic{{Path: "profile", Message: err.Error()}},
			}
			if writeErr := emitValidationCommandJSON(globals.stdout(), "validate", path, result); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	result := p.ValidateForProfile(path)
	if globals.JSON {
		if err := emitValidationCommandJSON(globals.stdout(), "validate", path, result); err != nil {
			return err
		}
		if !result.Valid {
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return nil
	}
	if !result.Valid {
		if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	_, err = fmt.Fprintf(globals.stdout(), "profile valid: %s\n", path)
	return err
}
