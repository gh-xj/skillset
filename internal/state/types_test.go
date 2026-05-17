package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gh-xj/skillset/internal/profile"
)

func TestStatePathsResolveBesideProfile(t *testing.T) {
	profilePath := filepath.Join("configs", "skills.profile.yaml")
	if got := DirForProfile(profilePath); got != filepath.Join("configs", ".skillset") {
		t.Fatalf("DirForProfile() = %q", got)
	}
	if got := StatePathForProfile(profilePath); got != filepath.Join("configs", ".skillset", "state.yaml") {
		t.Fatalf("StatePathForProfile() = %q", got)
	}
	if got := EventsPathForProfile(profilePath); got != filepath.Join("configs", ".skillset", "events.ndjson") {
		t.Fatalf("EventsPathForProfile() = %q", got)
	}
}

func TestSaveLoadMergeAndEvents(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, ".skillset", "state.yaml")
	eventsPath := filepath.Join(root, ".skillset", "events.ndjson")
	now := time.Date(2026, 5, 17, 6, 40, 0, 0, time.UTC)
	first := ManagedEntry{
		Agent:         profile.AgentCodex,
		Tier:          profile.TierUser,
		Name:          "skill-builder",
		Source:        "local:.//skills/skill-builder",
		SourceScheme:  profile.SourceLocal,
		TargetRel:     "skill-builder",
		TargetPath:    filepath.Join(root, "skill-builder"),
		TargetKind:    "symlink",
		RecordedBy:    "skillset",
		RecordedAt:    now,
		LastSeenAt:    now,
		PruneEligible: true,
	}
	store := MergeManaged(EmptyStore(), []ManagedEntry{first})
	if err := Save(statePath, store); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Managed) != 1 || loaded.Managed[0].Name != "skill-builder" {
		t.Fatalf("unexpected loaded store: %#v", loaded)
	}
	if loaded.Managed[0].TargetRel != "skill-builder" || loaded.Managed[0].TargetPath != "" {
		t.Fatalf("expected saved state to prefer target_rel, got %#v", loaded.Managed[0])
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if strings.Contains(string(data), "target_path:") {
		t.Fatalf("expected state file to omit target_path when target_rel exists:\n%s", data)
	}
	first.TargetPath = filepath.Join(root, "skill-builder")
	first.LastSeenAt = now.Add(time.Hour)
	merged := MergeManaged(loaded, []ManagedEntry{first})
	if len(merged.Managed) != 1 || !merged.Managed[0].LastSeenAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expected idempotent merge update, got %#v", merged.Managed)
	}
	event := Event{ID: "event-1", Operation: "adopt", Status: "adopted", Name: "skill-builder", Timestamp: now}
	if err := AppendEvent(eventsPath, event); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	if _, err := os.Stat(eventsPath); err != nil {
		t.Fatalf("expected events path to exist: %v", err)
	}
	events, err := LoadEvents(eventsPath)
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 1 || events[0].SchemaVersion != CurrentSchemaVersion || events[0].ID != "event-1" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestManagedEntryKeyIncludesSourceAndTarget(t *testing.T) {
	base := ManagedEntry{
		Agent:      profile.AgentCodex,
		Tier:       profile.TierUser,
		Name:       "skill-builder",
		Source:     "local:.//skills/skill-builder",
		TargetPath: "/tmp/skills/skill-builder",
	}
	sameIdentity := base
	if base.Key() != sameIdentity.Key() {
		t.Fatalf("expected identical entries to share a key")
	}
	changedSource := base
	changedSource.Source = "local:ark//skill-builder"
	if base.Key() == changedSource.Key() {
		t.Fatalf("expected source to participate in managed key")
	}
	changedTarget := base
	changedTarget.TargetPath = "/tmp/other/skill-builder"
	if base.Key() == changedTarget.Key() {
		t.Fatalf("expected target_path to participate in managed key")
	}
	root := "/tmp/skills"
	relative := base
	relative.TargetRel = "skill-builder"
	relative.TargetPath = ""
	if relative.ResolvedTargetPath(root) != filepath.Join(root, "skill-builder") {
		t.Fatalf("expected target_rel to resolve under root, got %q", relative.ResolvedTargetPath(root))
	}
}
