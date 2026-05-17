package skillsetcli

import (
	"fmt"
	"os/exec"

	"github.com/gh-xj/skillset/internal/appctx"
	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
)

var lookPath = exec.LookPath

type DoctorCmd struct{}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (c *DoctorCmd) Run(globals *CLI) error {
	checks, ok := globals.doctorChecks()
	if globals.JSON {
		if err := emitJSON(globals.stdout(), map[string]any{
			"ok":     ok,
			"checks": checks,
		}); err != nil {
			return err
		}
		if !ok {
			return appctx.NewExitError(appctx.ExitError, "")
		}
		return nil
	}
	for _, check := range checks {
		if _, err := fmt.Fprintf(globals.stdout(), "%s\t%s\t%s\n", check.Status, check.Name, check.Message); err != nil {
			return err
		}
	}
	if !ok {
		return appctx.NewExitError(appctx.ExitError, "")
	}
	return nil
}

func (c *CLI) doctorChecks() ([]doctorCheck, bool) {
	checks := []doctorCheck{}
	ok := true

	plan, result, _ := c.buildPlan()
	if !result.Valid {
		ok = false
		checks = append(checks, doctorCheck{Name: "profile", Status: "error", Message: joinDiagnostics(result.Errors)})
	} else {
		checks = append(checks, doctorCheck{Name: "profile", Status: "ok", Message: c.profilePath()})
	}

	roots, err := planner.Roots(c.plannerOptions())
	if err != nil {
		ok = false
		checks = append(checks, doctorCheck{Name: "roots", Status: "error", Message: err.Error()})
	} else {
		missing := 0
		for _, root := range roots {
			if root.Path != "" && !root.Exists {
				missing++
			}
		}
		status := "ok"
		message := "all configured filesystem roots exist"
		if missing > 0 {
			status = "warn"
			message = fmt.Sprintf("%d configured filesystem roots do not exist yet", missing)
		}
		checks = append(checks, doctorCheck{Name: "roots", Status: status, Message: message})
	}

	if result.Valid {
		if plan.Summary.Errors > 0 {
			ok = false
			checks = append(checks, doctorCheck{Name: "skill_state", Status: "error", Message: fmt.Sprintf("%d skill state errors", plan.Summary.Errors)})
		} else {
			checks = append(checks, doctorCheck{Name: "skill_state", Status: "ok", Message: fmt.Sprintf("%d desired entries checked", plan.Summary.Total)})
		}
		if profileUsesGitHub(plan.Items) {
			if path, err := lookPath("npx"); err == nil {
				checks = append(checks, doctorCheck{Name: "npx", Status: "ok", Message: path})
			} else {
				checks = append(checks, doctorCheck{Name: "npx", Status: "warn", Message: "npx not found; future github: apply/update commands will be unavailable"})
			}
		}
	}
	return checks, ok
}

func profileUsesGitHub(items []planner.Item) bool {
	for _, item := range items {
		if item.SourceScheme == profile.SourceGitHub {
			return true
		}
	}
	return false
}

func joinDiagnostics(diags []profile.Diagnostic) string {
	if len(diags) == 0 {
		return ""
	}
	if len(diags) == 1 {
		return diags[0].Path + ": " + diags[0].Message
	}
	return fmt.Sprintf("%s: %s (+%d more)", diags[0].Path, diags[0].Message, len(diags)-1)
}
