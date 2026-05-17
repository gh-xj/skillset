package skillsetcli

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/discover"
)

type DiscoverCmd struct {
	SuggestedProfile bool `name:"suggested-profile" help:"emit YAML for suggested skills.profile.yaml entries"`
}

func (c *DiscoverCmd) Run(globals *CLI) error {
	result, err := discover.Run(discover.Options{
		ProfilePath: globals.profilePath(),
		HomeDir:     globals.Home,
		RepoDir:     globals.Repo,
	})
	if err != nil {
		if globals.JSON {
			if writeErr := emitJSON(globals.stdout(), map[string]any{
				"ok":     false,
				"errors": []map[string]string{{"path": "discover", "message": err.Error()}},
			}); writeErr != nil {
				return writeErr
			}
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return err
	}
	if globals.JSON {
		return emitJSON(globals.stdout(), map[string]any{
			"ok":                true,
			"profile_path":      result.ProfilePath,
			"summary":           result.Summary,
			"entries":           result.Entries,
			"suggested_profile": result.SuggestedProfile,
		})
	}
	if c.SuggestedProfile {
		data, err := yaml.Marshal(result.SuggestedProfile)
		if err != nil {
			return err
		}
		_, err = globals.stdout().Write(data)
		return err
	}
	if len(result.Entries) == 0 {
		_, err := fmt.Fprintln(globals.stdout(), "no skills discovered")
		return err
	}
	for _, entry := range result.Entries {
		source := entry.Source
		if source == "" {
			source = "-"
		}
		if _, err := fmt.Fprintf(globals.stdout(), "%s\t%s\t%s\t%s\t%s\t%s\n", entry.Agent, entry.Tier, entry.Name, entry.Kind, entry.Status, source); err != nil {
			return err
		}
	}
	return nil
}
