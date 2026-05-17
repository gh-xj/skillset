package state

import (
	"path/filepath"
	"time"

	"github.com/gh-xj/skillset/internal/profile"
)

const CurrentSchemaVersion = 1

type Store struct {
	SchemaVersion int            `yaml:"schema_version" json:"schema_version"`
	Managed       []ManagedEntry `yaml:"managed" json:"managed"`
}

type ManagedEntry struct {
	Agent            profile.Agent        `yaml:"agent" json:"agent"`
	Tier             profile.Tier         `yaml:"tier" json:"tier"`
	Name             string               `yaml:"name" json:"name"`
	Source           string               `yaml:"source" json:"source"`
	SourceScheme     profile.SourceScheme `yaml:"source_scheme" json:"source_scheme"`
	TargetPath       string               `yaml:"target_path" json:"target_path"`
	TargetKind       string               `yaml:"target_kind" json:"target_kind"`
	RecordedBy       string               `yaml:"recorded_by" json:"recorded_by"`
	RecordedAt       time.Time            `yaml:"recorded_at" json:"recorded_at"`
	LastSeenAt       time.Time            `yaml:"last_seen_at" json:"last_seen_at"`
	LastOperationID  string               `yaml:"last_operation_id,omitempty" json:"last_operation_id,omitempty"`
	SkillFolderHash  string               `yaml:"skill_folder_hash,omitempty" json:"skill_folder_hash,omitempty"`
	SymlinkTarget    string               `yaml:"symlink_target,omitempty" json:"symlink_target,omitempty"`
	InstallCommand   []string             `yaml:"install_command,omitempty" json:"install_command,omitempty"`
	PruneEligible    bool                 `yaml:"prune_eligible" json:"prune_eligible"`
	PruneSafetyNotes []string             `yaml:"prune_safety_notes,omitempty" json:"prune_safety_notes,omitempty"`
}

type Event struct {
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	Operation     string        `json:"operation"`
	Status        string        `json:"status"`
	Agent         profile.Agent `json:"agent,omitempty"`
	Tier          profile.Tier  `json:"tier,omitempty"`
	Name          string        `json:"name,omitempty"`
	TargetPath    string        `json:"target_path,omitempty"`
	Source        string        `json:"source,omitempty"`
	Message       string        `json:"message,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
}

func DirForProfile(profilePath string) string {
	if profilePath == "" {
		profilePath = "skills.profile.yaml"
	}
	return filepath.Join(filepath.Dir(filepath.Clean(profilePath)), ".skillset")
}

func StatePathForProfile(profilePath string) string {
	return filepath.Join(DirForProfile(profilePath), "state.yaml")
}

func EventsPathForProfile(profilePath string) string {
	return filepath.Join(DirForProfile(profilePath), "events.ndjson")
}
