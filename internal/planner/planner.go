package planner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gh-xj/skillset/internal/placement"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/skillfs"
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

type DesiredPlacement = Item
type PlacementDecision = Item
type PlacementKey = placement.Key

type PlacementObservation struct {
	Placement DesiredPlacement
	Source    profile.Source

	Root    Root
	HasRoot bool

	SourceObservation skillfs.SkillDirObservation
	TargetObservation skillfs.PathObservation
	TargetError       error
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

func (i Item) Key() PlacementKey {
	return PlacementKey{
		Agent:      i.Agent,
		Tier:       i.Tier,
		Name:       i.Name,
		Source:     i.Source,
		TargetPath: i.TargetPath,
	}
}

func (i Item) IsCreateAction() bool {
	return i.Status == StatusMissingTarget && (i.Action == ActionInstallGitHub || i.Action == ActionLinkLocal)
}

func (i Item) IsError() bool {
	return isErrorStatus(i.Status)
}

func (i Item) IsIgnored() bool {
	return i.Action == ActionAudit || i.Action == ActionIgnore
}

func (i Item) IsAdoptable() bool {
	return i.Status == StatusPresent && i.Tier == profile.TierUser && i.TargetPath != ""
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
			completeItem(&item, source, profileDir, p.Roots, roots)
			plan.Items = append(plan.Items, item)
		}
	}
	plan.Summary = summarize(plan.Items)
	return plan, nil
}

func (p Plan) Changes() []Item {
	return p.Creates()
}

func (p Plan) Creates() []Item {
	out := make([]Item, 0, len(p.Items))
	for _, item := range p.Items {
		if item.IsCreateAction() {
			out = append(out, item)
		}
	}
	return out
}

func (p Plan) ErrorItems() []Item {
	return p.Errors()
}

func (p Plan) Errors() []Item {
	out := make([]Item, 0, len(p.Items))
	for _, item := range p.Items {
		if item.IsError() {
			out = append(out, item)
		}
	}
	return out
}

func (p Plan) Ignored() []Item {
	out := make([]Item, 0, len(p.Items))
	for _, item := range p.Items {
		if item.IsIgnored() {
			out = append(out, item)
		}
	}
	return out
}

func completeItem(item *Item, source profile.Source, profileDir string, namedRoots map[string]string, roots []Root) {
	observation := observePlacement(*item, source, profileDir, namedRoots, roots)
	*item = decidePlacement(observation)
}

func observePlacement(item DesiredPlacement, source profile.Source, profileDir string, namedRoots map[string]string, roots []Root) PlacementObservation {
	observation := PlacementObservation{
		Placement: item,
		Source:    source,
	}
	if source.Scheme == profile.SourceLocal {
		observation.Placement.SourcePath = resolveSourcePath(profileDir, namedRoots, source)
	}
	if observation.Placement.Tier == profile.TierSystem {
		return observation
	}
	root, ok := findRoot(roots, observation.Placement.Agent, observation.Placement.Tier)
	if !ok || root.Path == "" {
		return observation
	}
	observation.Root = root
	observation.HasRoot = true
	observation.Placement.TargetPath = filepath.Join(root.Path, observation.Placement.Name)
	if source.Scheme == profile.SourceLocal {
		observation.SourceObservation = skillfs.InspectSkillDir(observation.Placement.SourcePath, observation.Placement.Name)
	}
	target, err := skillfs.InspectPath(observation.Placement.TargetPath)
	if err != nil {
		observation.TargetError = err
	} else {
		observation.TargetObservation = target
	}
	return observation
}

func decidePlacement(observation PlacementObservation) PlacementDecision {
	item := observation.Placement
	source := observation.Source
	if item.Tier == profile.TierSystem {
		item.Status = StatusSystemIgnored
		item.Action = ActionIgnore
		item.Reason = "system tier is check/ignore-only"
		return item
	}

	if !observation.HasRoot {
		item.Status = StatusWrongKind
		item.Action = ActionNone
		item.Reason = fmt.Sprintf("no root configured for %s/%s", item.Agent, item.Tier)
		return item
	}

	if source.Scheme == profile.SourceLocal {
		if !observation.SourceObservation.Exists {
			item.Status = StatusMissingSource
			item.Action = ActionNone
			item.Reason = "local source path does not exist"
			return item
		}
		if !observation.SourceObservation.Valid {
			item.Status = StatusMissingSource
			item.Action = ActionNone
			item.Reason = "local source is not a valid skill: " + observation.SourceObservation.Reason
			return item
		}
	}
	if item.Tier == profile.TierRepo {
		if observation.TargetError != nil {
			item.Status = StatusWrongKind
			item.Action = ActionAudit
			item.Reason = observation.TargetError.Error()
			return item
		}
		if !observation.TargetObservation.Exists {
			item.Status = StatusMissingTarget
			item.Action = ActionAudit
			item.Reason = "repo tier is audit/check-only in v1"
			return item
		}
		if err := validateInstalledTarget(item, source, observation.Root); err != nil {
			item.Status = StatusWrongKind
			item.Action = ActionAudit
			item.Reason = err.Error()
			return item
		}
		item.Status = StatusRepoAuditOnly
		item.Action = ActionAudit
		item.Reason = "repo tier is audit/check-only in v1"
		return item
	}

	if observation.TargetError != nil {
		item.Status = StatusWrongKind
		item.Action = ActionNone
		item.Reason = observation.TargetError.Error()
		return item
	}
	if !observation.TargetObservation.Exists {
		item.Status = StatusMissingTarget
		item.Action = actionForSource(source.Scheme)
		item.Reason = "target skill is missing"
		return item
	}
	if source.Scheme == profile.SourceLocal {
		validateLocalTarget(&item, observation.TargetObservation)
		return item
	}
	if source.Scheme == profile.SourceGitHub && observation.TargetObservation.Kind == skillfs.KindSymlink {
		item.Status = StatusWrongKind
		item.Action = ActionNone
		item.Reason = "github sources use copy mode in v1; target is a symlink"
		return item
	}
	if source.Scheme == profile.SourceGitHub {
		if err := validateInstalledTarget(item, source, observation.Root); err != nil {
			item.Status = StatusWrongKind
			item.Action = ActionNone
			item.Reason = err.Error()
			return item
		}
	}
	item.Status = StatusPresent
	item.Action = ActionNone
	return item
}

func validateLocalTarget(item *Item, target skillfs.PathObservation) {
	if target.Kind != skillfs.KindSymlink {
		item.Status = StatusWrongKind
		item.Action = ActionNone
		item.Reason = "local sources use symlink installs in v1"
		return
	}
	targetReal, err := skillfs.RealPath(item.TargetPath)
	if err != nil {
		item.Status = StatusWrongTarget
		item.Action = ActionLinkLocal
		item.Reason = fmt.Sprintf("target symlink cannot be resolved: %v", err)
		return
	}
	sourceReal, err := skillfs.RealPath(item.SourcePath)
	if err != nil {
		item.Status = StatusMissingSource
		item.Action = ActionNone
		item.Reason = fmt.Sprintf("source cannot be resolved: %v", err)
		return
	}
	if targetReal != sourceReal {
		item.Status = StatusWrongTarget
		item.Action = ActionNone
		item.Reason = "target symlink points at a different source"
		return
	}
	item.Status = StatusPresent
	item.Action = ActionNone
}

func validateInstalledTarget(item Item, source profile.Source, root Root) error {
	switch source.Scheme {
	case profile.SourceLocal:
		return skillfs.ValidateSkillDir(item.TargetPath, item.Name)
	case profile.SourceGitHub:
		if err := skillfs.ValidateGitHubInstall(root.Path, item.TargetPath, item.Name, source); err != nil {
			return fmt.Errorf("github target is not a valid npx skills install: %w", err)
		}
		return nil
	default:
		return nil
	}
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
		if item.IsCreateAction() {
			summary.Changes++
		}
		if item.IsError() {
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

func resolveSourcePath(profileDir string, namedRoots map[string]string, source profile.Source) string {
	if source.Local == nil {
		return ""
	}
	root := source.Local.Root
	if namedRoot, ok := namedRoots[root]; ok {
		root = namedRoot
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(profileDir, root)
	}
	return filepath.Clean(filepath.Join(root, source.Local.SkillDir))
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
