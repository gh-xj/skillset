package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gh-xj/skillset/internal/profile"
)

const (
	StatusPresent       = "present"
	StatusMissingTarget = "missing_target"
	StatusMissingSource = "missing_source"
	StatusWrongKind     = "wrong_kind"
	StatusWrongTarget   = "wrong_target"
	StatusSystemIgnored = "system_ignored"
	StatusRepoAuditOnly = "repo_audit_only"

	ActionNone          = "none"
	ActionInstallGitHub = "install_github"
	ActionLinkLocal     = "link_local"
	ActionAudit         = "audit"
	ActionIgnore        = "ignore"
)

type Options struct {
	ProfilePath string
	HomeDir     string
	RepoDir     string
}

type Root struct {
	Agent  profile.Agent `json:"agent"`
	Tier   profile.Tier  `json:"tier"`
	Path   string        `json:"path,omitempty"`
	Mode   string        `json:"mode"`
	Exists bool          `json:"exists"`
}

type Item struct {
	Name         string               `json:"name"`
	Agent        profile.Agent        `json:"agent"`
	Tier         profile.Tier         `json:"tier"`
	Owner        profile.Owner        `json:"owner"`
	Source       string               `json:"source"`
	SourceScheme profile.SourceScheme `json:"source_scheme"`
	SourcePath   string               `json:"source_path,omitempty"`
	TargetPath   string               `json:"target_path,omitempty"`
	Status       string               `json:"status"`
	Action       string               `json:"action"`
	Reason       string               `json:"reason,omitempty"`
}

type Summary struct {
	Total         int `json:"total"`
	Present       int `json:"present"`
	MissingTarget int `json:"missing_target"`
	MissingSource int `json:"missing_source"`
	WrongKind     int `json:"wrong_kind"`
	WrongTarget   int `json:"wrong_target"`
	SystemIgnored int `json:"system_ignored"`
	RepoAuditOnly int `json:"repo_audit_only"`
	Changes       int `json:"changes"`
	Errors        int `json:"errors"`
}

type Plan struct {
	ProfilePath string  `json:"profile_path"`
	ProfileDir  string  `json:"profile_dir"`
	Roots       []Root  `json:"roots"`
	Items       []Item  `json:"items"`
	Summary     Summary `json:"summary"`
}

func Roots(opts Options) ([]Root, error) {
	home, err := resolveHome(opts.HomeDir)
	if err != nil {
		return nil, err
	}
	repo := cleanOrDefault(opts.RepoDir, ".")
	roots := []Root{
		{Agent: profile.AgentCodex, Tier: profile.TierUser, Path: filepath.Join(home, ".agents", "skills"), Mode: "managed"},
		{Agent: profile.AgentCodex, Tier: profile.TierRepo, Path: filepath.Join(repo, ".codex", "skills"), Mode: "audit_only"},
		{Agent: profile.AgentCodex, Tier: profile.TierSystem, Mode: "ignore"},
		{Agent: profile.AgentClaudeCode, Tier: profile.TierUser, Path: filepath.Join(home, ".claude", "skills"), Mode: "managed"},
		{Agent: profile.AgentClaudeCode, Tier: profile.TierRepo, Path: filepath.Join(repo, ".claude", "skills"), Mode: "audit_only"},
		{Agent: profile.AgentClaudeCode, Tier: profile.TierSystem, Mode: "ignore"},
	}
	for i := range roots {
		roots[i].Exists = pathExists(roots[i].Path)
	}
	return roots, nil
}

func Build(p profile.Profile, opts Options) (Plan, error) {
	roots, err := Roots(opts)
	if err != nil {
		return Plan{}, err
	}
	profilePath := cleanOrDefault(opts.ProfilePath, "skills.profile.yaml")
	profileDir := filepath.Dir(profilePath)
	plan := Plan{
		ProfilePath: profilePath,
		ProfileDir:  profileDir,
		Roots:       roots,
	}

	for _, skill := range p.Skills {
		source, err := profile.ParseSource(skill.Source)
		if err != nil {
			return Plan{}, err
		}
		for _, agent := range skill.Agents {
			item := Item{
				Name:         skill.Name,
				Agent:        agent,
				Tier:         skill.Tier,
				Owner:        skill.Owner,
				Source:       skill.Source,
				SourceScheme: source.Scheme,
			}
			completeItem(&item, source, profileDir, roots)
			plan.Items = append(plan.Items, item)
		}
	}
	slices.SortFunc(plan.Items, func(a, b Item) int {
		for _, cmp := range []int{
			cmpString(string(a.Agent), string(b.Agent)),
			cmpString(string(a.Tier), string(b.Tier)),
			cmpString(a.Name, b.Name),
			cmpString(a.Source, b.Source),
		} {
			if cmp != 0 {
				return cmp
			}
		}
		return 0
	})
	plan.Summary = summarize(plan.Items)
	return plan, nil
}

func (p Plan) Changes() []Item {
	out := make([]Item, 0, len(p.Items))
	for _, item := range p.Items {
		if item.Action == ActionInstallGitHub || item.Action == ActionLinkLocal {
			out = append(out, item)
		}
	}
	return out
}

func (p Plan) ErrorItems() []Item {
	out := make([]Item, 0, len(p.Items))
	for _, item := range p.Items {
		if isErrorStatus(item.Status) {
			out = append(out, item)
		}
	}
	return out
}

func completeItem(item *Item, source profile.Source, profileDir string, roots []Root) {
	if source.Scheme == profile.SourceLocal {
		item.SourcePath = resolveSourcePath(profileDir, source)
	}
	if item.Tier == profile.TierSystem {
		item.Status = StatusSystemIgnored
		item.Action = ActionIgnore
		item.Reason = "system tier is check/ignore-only"
		return
	}

	root, ok := findRoot(roots, item.Agent, item.Tier)
	if !ok || root.Path == "" {
		item.Status = StatusWrongKind
		item.Action = ActionNone
		item.Reason = fmt.Sprintf("no root configured for %s/%s", item.Agent, item.Tier)
		return
	}
	item.TargetPath = filepath.Join(root.Path, item.Name)

	if source.Scheme == profile.SourceLocal && !pathExists(item.SourcePath) {
		item.Status = StatusMissingSource
		item.Action = ActionNone
		item.Reason = "local source path does not exist"
		return
	}
	if item.Tier == profile.TierRepo {
		if !pathExists(item.TargetPath) {
			item.Status = StatusMissingTarget
			item.Action = ActionAudit
			item.Reason = "repo tier is audit/check-only in v1"
			return
		}
		item.Status = StatusRepoAuditOnly
		item.Action = ActionAudit
		item.Reason = "repo tier is audit/check-only in v1"
		return
	}

	targetInfo, err := os.Lstat(item.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			item.Status = StatusMissingTarget
			item.Action = actionForSource(source.Scheme)
			item.Reason = "target skill is missing"
			return
		}
		item.Status = StatusWrongKind
		item.Action = ActionNone
		item.Reason = err.Error()
		return
	}
	if source.Scheme == profile.SourceLocal {
		validateLocalTarget(item, targetInfo)
		return
	}
	if source.Scheme == profile.SourceGitHub && targetInfo.Mode()&os.ModeSymlink != 0 {
		item.Status = StatusWrongKind
		item.Action = ActionInstallGitHub
		item.Reason = "github sources use copy mode in v1; target is a symlink"
		return
	}
	item.Status = StatusPresent
	item.Action = ActionNone
}

func validateLocalTarget(item *Item, targetInfo os.FileInfo) {
	if targetInfo.Mode()&os.ModeSymlink == 0 {
		item.Status = StatusWrongKind
		item.Action = ActionLinkLocal
		item.Reason = "local sources use symlink installs in v1"
		return
	}
	target, err := filepath.EvalSymlinks(item.TargetPath)
	if err != nil {
		item.Status = StatusWrongTarget
		item.Action = ActionLinkLocal
		item.Reason = fmt.Sprintf("target symlink cannot be resolved: %v", err)
		return
	}
	source, err := filepath.EvalSymlinks(item.SourcePath)
	if err != nil {
		item.Status = StatusMissingSource
		item.Action = ActionNone
		item.Reason = fmt.Sprintf("source cannot be resolved: %v", err)
		return
	}
	if target != source {
		item.Status = StatusWrongTarget
		item.Action = ActionLinkLocal
		item.Reason = "target symlink points at a different source"
		return
	}
	item.Status = StatusPresent
	item.Action = ActionNone
}

func summarize(items []Item) Summary {
	var summary Summary
	summary.Total = len(items)
	for _, item := range items {
		switch item.Status {
		case StatusPresent:
			summary.Present++
		case StatusMissingTarget:
			summary.MissingTarget++
		case StatusMissingSource:
			summary.MissingSource++
		case StatusWrongKind:
			summary.WrongKind++
		case StatusWrongTarget:
			summary.WrongTarget++
		case StatusSystemIgnored:
			summary.SystemIgnored++
		case StatusRepoAuditOnly:
			summary.RepoAuditOnly++
		}
		if item.Action == ActionInstallGitHub || item.Action == ActionLinkLocal {
			summary.Changes++
		}
		if isErrorStatus(item.Status) {
			summary.Errors++
		}
	}
	return summary
}

func isErrorStatus(status string) bool {
	switch status {
	case StatusMissingSource, StatusMissingTarget, StatusWrongKind, StatusWrongTarget:
		return true
	default:
		return false
	}
}

func actionForSource(scheme profile.SourceScheme) string {
	switch scheme {
	case profile.SourceGitHub:
		return ActionInstallGitHub
	case profile.SourceLocal:
		return ActionLinkLocal
	default:
		return ActionNone
	}
}

func findRoot(roots []Root, agent profile.Agent, tier profile.Tier) (Root, bool) {
	for _, root := range roots {
		if root.Agent == agent && root.Tier == tier {
			return root, true
		}
	}
	return Root{}, false
}

func resolveSourcePath(profileDir string, source profile.Source) string {
	root := source.Root
	if !filepath.IsAbs(root) {
		root = filepath.Join(profileDir, root)
	}
	return filepath.Clean(filepath.Join(root, source.SkillDir))
}

func resolveHome(home string) (string, error) {
	if home != "" {
		return filepath.Clean(home), nil
	}
	resolved, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func cleanOrDefault(path, fallback string) string {
	if path == "" {
		path = fallback
	}
	return filepath.Clean(path)
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func cmpString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
