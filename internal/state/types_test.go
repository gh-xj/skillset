package state

import (
	"os"
	"path/filepath"
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
