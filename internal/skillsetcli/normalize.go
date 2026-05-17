package skillsetcli

import (
	"github.com/gh-xj/skillset/internal/appctx"
	"gopkg.in/yaml.v3"
)

type NormalizeCmd struct{}

func (c *NormalizeCmd) Run(globals *CLI) error {
	p, path, err := globals.loadProfile()
	if err != nil {
		if globals.JSON {
			result := profileError(err)
			if writeErr := emitValidationCommandJSON(globals.stdout(), "normalize", path, result); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	result := p.ValidateForProfile(path)
	if !result.Valid {
		if globals.JSON {
			if err := emitValidationCommandJSON(globals.stdout(), "normalize", path, result); err != nil {
				return err
			}
		} else if err := printValidationErrors(globals.stderr(), result); err != nil {
			return err
		}
		return appctx.NewExitError(appctx.ExitError, "")
	}
	if globals.JSON {
		return emitCommandJSON(globals.stdout(), "normalize", true, path, nil, map[string]any{
			"profile": p,
		}, result.Warnings, result.Errors, map[string]any{
			"profile_path": path,
			"profile":      p,
		})
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	_, err = globals.stdout().Write(data)
	return err
}
