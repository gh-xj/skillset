package prune

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/skillfs"
	"github.com/gh-xj/skillset/internal/state"
)

type Options struct {
	Apply       bool
	ProfilePath string
	Now         func() time.Time
}

type Result struct {
	DryRun      bool                 `json:"dry_run"`
	ProfilePath string               `json:"profile_path"`
	StatePath   string               `json:"state_path"`
	EventsPath  string               `json:"events_path"`
	Planned     []state.ManagedEntry `json:"planned"`
	Deleted     []state.ManagedEntry `json:"deleted"`
	Skipped     []SkippedEntry       `json:"skipped"`
	Failed      []SkippedEntry       `json:"failed"`
	Summary     Summary              `json:"summary"`
}

type SkippedEntry struct {
	Entry  state.ManagedEntry `json:"entry"`
	Reason string             `json:"reason"`
}

type Summary struct {
	Planned int `json:"planned"`
	Deleted int `json:"deleted"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Written int `json:"written"`
}

func Run(plan planner.Plan, opts Options) (Result, error) {
	now := time.Now().UTC()
	if opts.Now != nil {
		now = opts.Now().UTC()
	}
	if opts.ProfilePath == "" {
		opts.ProfilePath = plan.ProfilePath
	}
	result := Result{
		DryRun:      !opts.Apply,
		ProfilePath: opts.ProfilePath,
		StatePath:   state.StatePathForProfile(opts.ProfilePath),
		EventsPath:  state.EventsPathForProfile(opts.ProfilePath),
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		return Result{}, err
	}
	desired := desiredKeys(plan)
	for _, entry := range store.Managed {
		root, hasRoot := rootForEntry(plan.Roots, entry)
		if hasRoot {
			entry = entry.WithResolvedTargetPath(root.Path)
		}
		if hasRoot && (desired[entry.KeyForRoot(root.Path)] || semanticallyDesired(plan, entry, root)) {
			result.Skipped = append(result.Skipped, SkippedEntry{Entry: entry, Reason: "still desired by profile"})
			continue
		}
		if !entry.PruneEligible {
			result.Skipped = append(result.Skipped, SkippedEntry{Entry: entry, Reason: "state entry is not prune eligible"})
			continue
		}
		if entry.Tier != profile.TierUser {
			result.Skipped = append(result.Skipped, SkippedEntry{Entry: entry, Reason: "only user-tier entries can be pruned in v1"})
			continue
		}
		result.Planned = append(result.Planned, entry)
	}
	result.Summary.Planned = len(result.Planned)
	result.Summary.Skipped = len(result.Skipped)
	if !opts.Apply || len(result.Planned) == 0 {
		return result, nil
	}
	remaining := append([]state.ManagedEntry(nil), store.Managed...)
	for _, entry := range result.Planned {
		root, ok := rootForEntry(plan.Roots, entry)
		if !ok {
			result.Failed = append(result.Failed, SkippedEntry{Entry: entry, Reason: "no configured root for managed entry"})
			continue
		}
		if err := deleteManaged(entry, root.Path); err != nil {
			result.Failed = append(result.Failed, SkippedEntry{Entry: entry, Reason: err.Error()})
			continue
		}
		result.Deleted = append(result.Deleted, entry)
		remaining = removeEntry(remaining, entry)
	}
	result.Summary.Deleted = len(result.Deleted)
	result.Summary.Failed = len(result.Failed)
	if len(result.Deleted) == 0 {
		return result, nil
	}
	store.Managed = remaining
	if err := state.Save(result.StatePath, store); err != nil {
		return Result{}, err
	}
	for _, entry := range result.Deleted {
		if err := state.AppendEvent(result.EventsPath, state.Event{
			ID:         eventID("prune", entry, now),
			Operation:  "prune",
			Status:     "deleted",
			Agent:      entry.Agent,
			Tier:       entry.Tier,
			Name:       entry.Name,
			TargetPath: entry.TargetPath,
			Source:     entry.Source,
			Message:    "deleted skillset-managed entry no longer desired by profile",
			Timestamp:  now,
		}); err != nil {
			return Result{}, err
		}
		result.Summary.Written++
	}
	return result, nil
}

func desiredKeys(plan planner.Plan) map[planner.PlacementKey]bool {
	out := map[planner.PlacementKey]bool{}
	for _, item := range plan.Items {
		if item.TargetPath == "" {
			continue
		}
		out[item.Key()] = true
	}
	return out
}

func semanticallyDesired(plan planner.Plan, entry state.ManagedEntry, root planner.Root) bool {
	entry = entry.WithResolvedTargetPath(root.Path)
	for _, item := range plan.Items {
		if item.Agent != entry.Agent || item.Tier != entry.Tier || item.Name != entry.Name || item.TargetPath != entry.TargetPath {
			continue
		}
		if item.Source == entry.Source {
			return true
		}
		if localSourcesMatch(plan.ProfileDir, item, entry) {
			return true
		}
		if githubSourcesMatch(item.Source, entry.Source) {
			return true
		}
	}
	return false
}

func localSourcesMatch(profileDir string, item planner.Item, entry state.ManagedEntry) bool {
	desired, err := profile.ParseSource(item.Source)
	if err != nil || desired.Scheme != profile.SourceLocal || item.SourcePath == "" {
		return false
	}
	recorded, err := profile.ParseSource(entry.Source)
	if err != nil || recorded.Scheme != profile.SourceLocal {
		return false
	}
	if sameRealPath(item.SourcePath, resolveRecordedLocalSource(profileDir, recorded)) {
		return true
	}
	if entry.TargetPath == "" {
		return false
	}
	targetReal, err := skillfs.RealPath(entry.TargetPath)
	if err != nil {
		return false
	}
	return sameRealPath(item.SourcePath, targetReal)
}

func resolveRecordedLocalSource(profileDir string, source profile.Source) string {
	if source.Local == nil {
		return ""
	}
	root := source.Local.Root
	if !filepath.IsAbs(root) {
		root = filepath.Join(profileDir, root)
	}
	return filepath.Clean(filepath.Join(root, source.Local.SkillDir))
}

func sameRealPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aReal, err := skillfs.RealPath(a)
	if err != nil {
		return false
	}
	bReal, err := skillfs.RealPath(b)
	if err != nil {
		return false
	}
	return aReal == bReal
}

func githubSourcesMatch(a, b string) bool {
	left, err := profile.ParseSource(a)
	if err != nil || left.GitHub == nil {
		return false
	}
	right, err := profile.ParseSource(b)
	if err != nil || right.GitHub == nil {
		return false
	}
	return *left.GitHub == *right.GitHub
}

func removeEntry(entries []state.ManagedEntry, needle state.ManagedEntry) []state.ManagedEntry {
	key := needle.Key()
	out := entries[:0]
	for _, entry := range entries {
		if entry.Key() == key || sameManagedEntry(entry, needle) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func sameManagedEntry(a, b state.ManagedEntry) bool {
	return a.Agent == b.Agent &&
		a.Tier == b.Tier &&
		a.Name == b.Name &&
		a.Source == b.Source &&
		a.TargetRel != "" &&
		a.TargetRel == b.TargetRel
}

func rootForEntry(roots []planner.Root, entry state.ManagedEntry) (planner.Root, bool) {
	for _, root := range roots {
		if root.Agent == entry.Agent && root.Tier == entry.Tier {
			return root, true
		}
	}
	return planner.Root{}, false
}

func deleteManaged(entry state.ManagedEntry, rootPath string) error {
	if entry.TargetPath == "" || entry.TargetPath == "/" || entry.TargetPath == "." {
		return fmt.Errorf("refusing unsafe target path %q", entry.TargetPath)
	}
	if err := skillfs.ValidatePathUnderRoot(entry.TargetPath, rootPath); err != nil {
		return err
	}
	info, err := os.Lstat(entry.TargetPath)
	if err != nil {
		return fmt.Errorf("inspect target %s: %w", entry.TargetPath, err)
	}
	switch entry.TargetKind {
	case "symlink":
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("target is no longer a symlink")
		}
		if entry.SymlinkTarget != "" {
			target, err := os.Readlink(entry.TargetPath)
			if err != nil {
				return fmt.Errorf("read symlink target: %w", err)
			}
			if target != entry.SymlinkTarget {
				return fmt.Errorf("symlink target changed from %q to %q", entry.SymlinkTarget, target)
			}
		}
		return os.Remove(entry.TargetPath)
	case "directory":
		if !info.IsDir() {
			return fmt.Errorf("target is no longer a directory")
		}
		if _, err := os.Stat(filepath.Join(entry.TargetPath, "SKILL.md")); err != nil {
			return fmt.Errorf("refusing to delete directory without SKILL.md: %w", err)
		}
		return os.RemoveAll(entry.TargetPath)
	default:
		return fmt.Errorf("unsupported managed target kind %q", entry.TargetKind)
	}
}

func eventID(operation string, entry state.ManagedEntry, ts time.Time) string {
	return fmt.Sprintf("%s-%s-%s-%s-%d", operation, entry.Agent, entry.Tier, filepath.Base(entry.TargetPath), ts.UnixNano())
}
