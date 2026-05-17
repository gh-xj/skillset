package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func EmptyStore() Store {
	return Store{SchemaVersion: CurrentSchemaVersion, Managed: []ManagedEntry{}}
}

func Load(path string) (Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EmptyStore(), nil
		}
		return Store{}, fmt.Errorf("read state %s: %w", path, err)
	}
	var store Store
	if err := yaml.Unmarshal(data, &store); err != nil {
		return Store{}, fmt.Errorf("decode state %s: %w", path, err)
	}
	if store.SchemaVersion == 0 {
		store.SchemaVersion = CurrentSchemaVersion
	}
	if store.SchemaVersion != CurrentSchemaVersion {
		return Store{}, fmt.Errorf("unsupported state schema_version %d", store.SchemaVersion)
	}
	sortManaged(store.Managed)
	return store, nil
}

func Save(path string, store Store) error {
	store.SchemaVersion = CurrentSchemaVersion
	sortManaged(store.Managed)
	data, err := yaml.Marshal(storeForSave(store))
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure state dir %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace state %s: %w", path, err)
	}
	return nil
}

func storeForSave(store Store) Store {
	out := Store{
		SchemaVersion: store.SchemaVersion,
		Managed:       append([]ManagedEntry(nil), store.Managed...),
	}
	for i := range out.Managed {
		if out.Managed[i].TargetRel != "" {
			out.Managed[i].TargetPath = ""
		}
	}
	return out
}

func MergeManaged(store Store, entries []ManagedEntry) Store {
	store.SchemaVersion = CurrentSchemaVersion
	byKey := map[PlacementKey]ManagedEntry{}
	for _, entry := range store.Managed {
		byKey[entry.Key()] = entry
	}
	for _, entry := range entries {
		byKey[entry.Key()] = entry
	}
	store.Managed = store.Managed[:0]
	for _, entry := range byKey {
		store.Managed = append(store.Managed, entry)
	}
	sortManaged(store.Managed)
	return store
}

func AppendEvent(path string, event Event) error {
	event.SchemaVersion = CurrentSchemaVersion
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure events dir %s: %w", filepath.Dir(path), err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events %s: %w", path, err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	if err := enc.Encode(event); err != nil {
		return fmt.Errorf("append event %s: %w", path, err)
	}
	return nil
}

func LoadEvents(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read events %s: %w", path, err)
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode event %s: %w", path, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events %s: %w", path, err)
	}
	return events, nil
}

func sortManaged(entries []ManagedEntry) {
	slices.SortFunc(entries, func(a, b ManagedEntry) int {
		for _, cmp := range []int{
			cmpString(string(a.Agent), string(b.Agent)),
			cmpString(string(a.Tier), string(b.Tier)),
			cmpString(a.Name, b.Name),
			cmpString(a.TargetPath, b.TargetPath),
		} {
			if cmp != 0 {
				return cmp
			}
		}
		return 0
	})
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
