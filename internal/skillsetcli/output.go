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

func emitCommandJSON(w io.Writer, command string, ok bool, profilePath string, summary any, result any, warnings any, errors any, legacy map[string]any) error {
	payload := map[string]any{
		"ok":       ok,
		"command":  command,
		"warnings": emptyIfNil(warnings),
		"errors":   emptyIfNil(errors),
	}
	if profilePath != "" {
		payload["profile_path"] = profilePath
	}
	if summary != nil {
		payload["summary"] = summary
	}
	if result != nil {
		payload["result"] = result
	}
	for key, value := range legacy {
		payload[key] = value
	}
	return emitJSON(w, payload)
}

func emitCommandErrorJSON(w io.Writer, command, path, message string) error {
	errors := []profile.Diagnostic{{Path: path, Message: message}}
	return emitCommandJSON(w, command, false, "", nil, nil, nil, errors, map[string]any{
		"ok":     false,
		"errors": errors,
	})
}

func emitValidationCommandJSON(w io.Writer, command, path string, result profile.ValidationResult) error {
	return emitCommandJSON(w, command, result.Valid, path, map[string]any{"valid": result.Valid}, nil, result.Warnings, result.Errors, validationPayload(path, result))
}

func emptyIfNil(value any) any {
	if value == nil {
		return []any{}
	}
	return value
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
