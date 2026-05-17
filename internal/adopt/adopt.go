package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/state"
)

type Options struct {
	Apply       bool
	ProfilePath string
	ToolName    string
	Now         func() time.Time
}

type Result struct {
	DryRun      bool                 `json:"dry_run"`
	ProfilePath string               `json:"profile_path"`
	StatePath   string               `json:"state_path"`
	EventsPath  string               `json:"events_path"`
	Adopted     []state.ManagedEntry `json:"adopted"`
	Skipped     []SkippedEntry       `json:"skipped"`
	Summary     Summary              `json:"summary"`
}

type SkippedEntry struct {
	Name       string        `json:"name"`
	Agent      profile.Agent `json:"agent"`
	Tier       profile.Tier  `json:"tier"`
	Status     string        `json:"status"`
	TargetPath string        `json:"target_path,omitempty"`
	Reason     string        `json:"reason"`
}

type Summary struct {
	Adopted int `json:"adopted"`
	Skipped int `json:"skipped"`
	Written int `json:"written"`
}

func Run(plan planner.Plan, opts Options) (Result, error) {
	now := time.Now().UTC()
	if opts.Now != nil {
		now = opts.Now().UTC()
	}
	if opts.ToolName == "" {
		opts.ToolName = "skillset"
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
	for _, item := range plan.Items {
		if adoptable(item) {
			entry, err := managedEntry(item, opts.ToolName, now)
			if err != nil {
				return Result{}, err
			}
			result.Adopted = append(result.Adopted, entry)
			continue
		}
		result.Skipped = append(result.Skipped, skippedEntry(item))
	}
	result.Summary.Adopted = len(result.Adopted)
	result.Summary.Skipped = len(result.Skipped)

	if !opts.Apply || len(result.Adopted) == 0 {
		return result, nil
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		return Result{}, err
	}
	store = state.MergeManaged(store, result.Adopted)
	if err := state.Save(result.StatePath, store); err != nil {
		return Result{}, err
	}
	for _, entry := range result.Adopted {
		if err := state.AppendEvent(result.EventsPath, state.Event{
			ID:         eventID("adopt", entry, now),
			Operation:  "adopt",
			Status:     "adopted",
			Agent:      entry.Agent,
			Tier:       entry.Tier,
			Name:       entry.Name,
			TargetPath: entry.TargetPath,
			Source:     entry.Source,
			Message:    "recorded existing matching skill as skillset-managed",
			Timestamp:  now,
		}); err != nil {
			return Result{}, err
		}
		result.Summary.Written++
	}
	return result, nil
}

func adoptable(item planner.Item) bool {
	return item.Status == planner.StatusPresent && item.Tier == profile.TierUser && item.TargetPath != ""
}

func managedEntry(item planner.Item, toolName string, now time.Time) (state.ManagedEntry, error) {
	info, err := os.Lstat(item.TargetPath)
	if err != nil {
		return state.ManagedEntry{}, fmt.Errorf("inspect target %s: %w", item.TargetPath, err)
	}
	source, err := profile.ParseSource(item.Source)
	if err != nil {
		return state.ManagedEntry{}, err
	}
	entry := state.ManagedEntry{
		Agent:            item.Agent,
		Tier:             item.Tier,
		Name:             item.Name,
		Source:           item.Source,
		SourceScheme:     item.SourceScheme,
		TargetPath:       item.TargetPath,
		TargetKind:       targetKind(info),
		RecordedBy:       toolName,
		RecordedAt:       now,
		LastSeenAt:       now,
		PruneEligible:    true,
		PruneSafetyNotes: []string{"adopted from an existing target that matched skills.profile.yaml"},
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(item.TargetPath); err == nil {
			entry.SymlinkTarget = target
		}
	}
	if source.Scheme == profile.SourceGitHub {
		entry.InstallCommand = []string{"npx", "skills", "add", source.Owner + "/" + source.Repo, "-g", "-s", item.Name, "-a", string(item.Agent), "-y", "--copy"}
	}
	return entry, nil
}

func skippedEntry(item planner.Item) SkippedEntry {
	reason := item.Reason
	if reason == "" {
		switch {
		case item.Tier == profile.TierSystem:
			reason = "system tier is check/ignore-only"
		case item.Tier == profile.TierRepo:
			reason = "repo tier is audit/check-only in v1"
		default:
			reason = "target is not present and matching"
		}
	}
	return SkippedEntry{
		Name:       item.Name,
		Agent:      item.Agent,
		Tier:       item.Tier,
		Status:     item.Status,
		TargetPath: item.TargetPath,
		Reason:     reason,
	}
}

func targetKind(info os.FileInfo) string {
	if info.Mode()&os.ModeSymlink != 0 {
		return "symlink"
	}
	if info.IsDir() {
		return "directory"
	}
	if info.Mode().IsRegular() {
		return "file"
	}
	return "other"
}

func eventID(operation string, entry state.ManagedEntry, ts time.Time) string {
	return fmt.Sprintf("%s-%s-%s-%s-%d", operation, entry.Agent, entry.Tier, filepath.Base(entry.TargetPath), ts.UnixNano())
}
