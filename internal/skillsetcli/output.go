package skillsetcli

import (
	"fmt"
	"io"
	"strings"

	appio "github.com/gh-xj/skillset/internal/io"
	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
)

func emitJSON(w io.Writer, payload map[string]any) error {
	payload["schema_version"] = "v1"
	return appio.WriteJSON(w, payload)
}

func validationPayload(path string, result profile.ValidationResult) map[string]any {
	if result.Errors == nil {
		result.Errors = []profile.Diagnostic{}
	}
	if result.Warnings == nil {
		result.Warnings = []profile.Diagnostic{}
	}
	return map[string]any{
		"profile_path": path,
		"valid":        result.Valid,
		"errors":       result.Errors,
		"warnings":     result.Warnings,
	}
}

func printValidationErrors(w io.Writer, result profile.ValidationResult) error {
	for _, diag := range result.Errors {
		if _, err := fmt.Fprintf(w, "%s: %s\n", diag.Path, diag.Message); err != nil {
			return err
		}
	}
	return nil
}

func profileError(err error) profile.ValidationResult {
	return profile.ValidationResult{
		Valid:  false,
		Errors: []profile.Diagnostic{{Path: "profile", Message: err.Error()}},
	}
}

func agentStrings(agents []profile.Agent) []string {
	out := make([]string, 0, len(agents))
	for _, agent := range agents {
		out = append(out, string(agent))
	}
	return out
}

func joinAgents(agents []profile.Agent) string {
	return strings.Join(agentStrings(agents), ",")
}

func printPlanItems(w io.Writer, items []planner.Item, empty string) error {
	if len(items) == 0 {
		_, err := fmt.Fprintln(w, empty)
		return err
	}
	for _, item := range items {
		target := item.TargetPath
		if target == "" {
			target = "-"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", item.Agent, item.Tier, item.Name, item.Status, target); err != nil {
			return err
		}
	}
	return nil
}
