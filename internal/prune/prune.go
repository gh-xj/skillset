package prune

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
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
		if desired[entryKey(entry)] {
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
		if err := deleteManaged(entry); err != nil {
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

func desiredKeys(plan planner.Plan) map[string]bool {
	out := map[string]bool{}
	for _, item := range plan.Items {
		if item.TargetPath == "" {
			continue
		}
		out[strings.Join([]string{string(item.Agent), string(item.Tier), item.Name, item.Source, item.TargetPath}, "\x00")] = true
	}
	return out
}

func entryKey(entry state.ManagedEntry) string {
	return strings.Join([]string{string(entry.Agent), string(entry.Tier), entry.Name, entry.Source, entry.TargetPath}, "\x00")
}

func removeEntry(entries []state.ManagedEntry, needle state.ManagedEntry) []state.ManagedEntry {
	key := entryKey(needle)
	out := entries[:0]
	for _, entry := range entries {
		if entryKey(entry) == key {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func deleteManaged(entry state.ManagedEntry) error {
	if entry.TargetPath == "" || entry.TargetPath == "/" || entry.TargetPath == "." {
		return fmt.Errorf("refusing unsafe target path %q", entry.TargetPath)
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
