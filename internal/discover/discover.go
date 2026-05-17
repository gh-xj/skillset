package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
)

const (
	KindGitHub  = "github"
	KindLocal   = "local"
	KindUnknown = "unknown"

	StatusSuggested     = "suggested"
	StatusUnknown       = "unknown"
	StatusBrokenSymlink = "broken_symlink"
)

type Options struct {
	ProfilePath string
	HomeDir     string
	RepoDir     string
}

type Result struct {
	ProfilePath      string          `json:"profile_path"`
	Roots            []planner.Root  `json:"roots"`
	Entries          []Entry         `json:"entries"`
	SuggestedProfile profile.Profile `json:"suggested_profile"`
	Summary          Summary         `json:"summary"`
}

type Entry struct {
	Name       string         `json:"name"`
	Agent      profile.Agent  `json:"agent"`
	Tier       profile.Tier   `json:"tier"`
	Path       string         `json:"path"`
	Kind       string         `json:"kind"`
	Status     string         `json:"status"`
	Owner      profile.Owner  `json:"owner,omitempty"`
	Source     string         `json:"source,omitempty"`
	SourcePath string         `json:"source_path,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	Lock       *LockSkillInfo `json:"lock,omitempty"`
	Suggested  *profile.Skill `json:"suggested,omitempty"`
}

type LockSkillInfo struct {
	Source          string `json:"source"`
	SourceType      string `json:"source_type"`
	SourceURL       string `json:"source_url,omitempty"`
	SkillPath       string `json:"skill_path"`
	SkillFolderHash string `json:"skill_folder_hash,omitempty"`
	InstalledAt     string `json:"installed_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type Summary struct {
	Total     int `json:"total"`
	GitHub    int `json:"github"`
	Local     int `json:"local"`
	Unknown   int `json:"unknown"`
	Broken    int `json:"broken"`
	Suggested int `json:"suggested"`
}

type lockFile struct {
	Skills map[string]lockSkill `json:"skills"`
}

type lockSkill struct {
	Source          string `json:"source"`
	SourceType      string `json:"sourceType"`
	SourceURL       string `json:"sourceUrl"`
	SkillPath       string `json:"skillPath"`
	SkillFolderHash string `json:"skillFolderHash"`
	InstalledAt     string `json:"installedAt"`
	UpdatedAt       string `json:"updatedAt"`
}

func Run(opts Options) (Result, error) {
	roots, err := planner.Roots(planner.Options{
		ProfilePath: opts.ProfilePath,
		HomeDir:     opts.HomeDir,
		RepoDir:     opts.RepoDir,
	})
	if err != nil {
		return Result{}, err
	}
	result := Result{
		ProfilePath: cleanOrDefault(opts.ProfilePath, "skills.profile.yaml"),
		Roots:       roots,
	}
	profileDir := filepath.Dir(result.ProfilePath)
	namedRoots := loadProfileRoots(result.ProfilePath)
	for _, root := range roots {
		if root.Path == "" || root.Tier == profile.TierSystem {
			continue
		}
		entries, err := scanRoot(root, profileDir, namedRoots)
		if err != nil {
			return Result{}, err
		}
		result.Entries = append(result.Entries, entries...)
	}
	sortEntries(result.Entries)
	result.SuggestedProfile = suggestedProfile(result.Entries, namedRoots)
	result.Summary = summarize(result.Entries)
	return result, nil
}

func scanRoot(root planner.Root, profileDir string, namedRoots map[string]string) ([]Entry, error) {
	infos, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read root %s: %w", root.Path, err)
	}
	lock, err := readLock(filepath.Join(filepath.Dir(root.Path), ".skill-lock.json"))
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(root.Path, name)
		entry := Entry{
			Name:   name,
			Agent:  root.Agent,
			Tier:   root.Tier,
			Path:   path,
			Kind:   KindUnknown,
			Status: StatusUnknown,
			Reason: "entry is not a symlink and has no recognized lock metadata",
		}
		if classifySymlink(&entry, profileDir, namedRoots) {
			entries = append(entries, entry)
			continue
		}
		if locked, ok := lock[name]; ok && classifyLock(&entry, locked) {
			entries = append(entries, entry)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func classifySymlink(entry *Entry, profileDir string, namedRoots map[string]string) bool {
	info, err := os.Lstat(entry.Path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(entry.Path)
	if err != nil {
		entry.Kind = KindLocal
		entry.Status = StatusBrokenSymlink
		entry.Reason = fmt.Sprintf("read symlink: %v", err)
		return true
	}
	if !filepath.IsAbs(target) {
		parent := filepath.Dir(entry.Path)
		if realParent, err := filepath.EvalSymlinks(parent); err == nil {
			parent = realParent
		}
		target = filepath.Join(parent, target)
	}
	target = filepath.Clean(target)
	entry.SourcePath = target
	if _, err := os.Stat(target); err != nil {
		entry.Kind = KindLocal
		entry.Status = StatusBrokenSymlink
		entry.Reason = fmt.Sprintf("symlink target is not readable: %v", err)
		return true
	}
	entry.Kind = KindLocal
	entry.Status = StatusSuggested
	entry.Owner = ownerForTier(entry.Tier)
	entry.Source = localSourceURI(profileDir, target, namedRoots)
	entry.Reason = ""
	entry.Suggested = &profile.Skill{
		Name:   entry.Name,
		Tier:   entry.Tier,
		Owner:  entry.Owner,
		Source: entry.Source,
		Agents: []profile.Agent{entry.Agent},
	}
	return true
}

func classifyLock(entry *Entry, locked lockSkill) bool {
	if locked.SourceType != "github" || locked.Source == "" || locked.SkillPath == "" {
		return false
	}
	sourceDir := filepath.ToSlash(filepath.Dir(locked.SkillPath))
	if sourceDir == "." || sourceDir == "" {
		return false
	}
	entry.Kind = KindGitHub
	entry.Status = StatusSuggested
	entry.Owner = profile.OwnerUpstream
	entry.Source = fmt.Sprintf("github:%s//%s", locked.Source, sourceDir)
	entry.Reason = ""
	lockInfo := locked.info()
	entry.Lock = &lockInfo
	entry.Suggested = &profile.Skill{
		Name:   entry.Name,
		Tier:   entry.Tier,
		Owner:  entry.Owner,
		Source: entry.Source,
		Agents: []profile.Agent{entry.Agent},
	}
	return true
}

func readLock(path string) (map[string]lockSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]lockSkill{}, nil
		}
		return nil, fmt.Errorf("read lock %s: %w", path, err)
	}
	var lock lockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("decode lock %s: %w", path, err)
	}
	if lock.Skills == nil {
		return map[string]lockSkill{}, nil
	}
	return lock.Skills, nil
}

func suggestedProfile(entries []Entry, namedRoots map[string]string) profile.Profile {
	type key struct {
		name   string
		tier   profile.Tier
		owner  profile.Owner
		source string
	}
	byKey := map[key]*profile.Skill{}
	var keys []key
	for _, entry := range entries {
		if entry.Suggested == nil {
			continue
		}
		k := key{name: entry.Suggested.Name, tier: entry.Suggested.Tier, owner: entry.Suggested.Owner, source: entry.Suggested.Source}
		skill, ok := byKey[k]
		if !ok {
			copy := *entry.Suggested
			copy.Agents = append([]profile.Agent(nil), entry.Suggested.Agents...)
			byKey[k] = &copy
			keys = append(keys, k)
			continue
		}
		if !slices.Contains(skill.Agents, entry.Agent) {
			skill.Agents = append(skill.Agents, entry.Agent)
		}
	}
	out := profile.Profile{SchemaVersion: profile.CurrentSchemaVersion}
	for _, k := range keys {
		skill := byKey[k]
		slices.Sort(skill.Agents)
		out.Skills = append(out.Skills, *skill)
	}
	out.Roots = suggestedRoots(out.Skills, namedRoots)
	return out.Normalized()
}

func summarize(entries []Entry) Summary {
	var summary Summary
	summary.Total = len(entries)
	for _, entry := range entries {
		switch entry.Kind {
		case KindGitHub:
			summary.GitHub++
		case KindLocal:
			summary.Local++
		default:
			summary.Unknown++
		}
		if entry.Status == StatusBrokenSymlink {
			summary.Broken++
		}
		if entry.Suggested != nil {
			summary.Suggested++
		}
	}
	return summary
}

func sortEntries(entries []Entry) {
	slices.SortFunc(entries, func(a, b Entry) int {
		for _, cmp := range []int{
			cmpString(string(a.Agent), string(b.Agent)),
			cmpString(string(a.Tier), string(b.Tier)),
			cmpString(a.Name, b.Name),
			cmpString(a.Path, b.Path),
		} {
			if cmp != 0 {
				return cmp
			}
		}
		return 0
	})
}

func ownerForTier(tier profile.Tier) profile.Owner {
	if tier == profile.TierRepo {
		return profile.OwnerRepo
	}
	return profile.OwnerPrivate
}

func localSourceURI(profileDir, target string, namedRoots map[string]string) string {
	if source, ok := namedRootSourceURI(profileDir, target, namedRoots); ok {
		return source
	}
	if source, ok := inferredProfileRootSourceURI(profileDir, target); ok {
		return source
	}
	if rel, err := filepath.Rel(profileDir, target); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return fmt.Sprintf("local:.//%s", filepath.ToSlash(rel))
	}
	return fmt.Sprintf("local:%s//%s", filepath.ToSlash(filepath.Dir(target)), filepath.ToSlash(filepath.Base(target)))
}

func inferredProfileRootSourceURI(profileDir, target string) (string, bool) {
	parentRel, err := filepath.Rel(profileDir, filepath.Dir(target))
	if err != nil || parentRel == "." || parentRel == ".." || strings.HasPrefix(parentRel, ".."+string(filepath.Separator)) || filepath.IsAbs(parentRel) {
		return "", false
	}
	parentRel = filepath.Clean(parentRel)
	if strings.Contains(parentRel, string(filepath.Separator)) || !profile.ValidRootName(parentRel) {
		return "", false
	}
	return fmt.Sprintf("local:%s//%s", parentRel, filepath.ToSlash(filepath.Base(target))), true
}

func loadProfileRoots(profilePath string) map[string]string {
	p, err := profile.LoadFile(profilePath)
	if err != nil {
		return nil
	}
	return p.Normalized().Roots
}

func suggestedRoots(skills []profile.Skill, namedRoots map[string]string) map[string]string {
	out := map[string]string{}
	for _, skill := range skills {
		source, err := profile.ParseSource(skill.Source)
		if err != nil || source.Scheme != profile.SourceLocal {
			continue
		}
		if rootPath, ok := namedRoots[source.Root]; ok {
			out[source.Root] = rootPath
			continue
		}
		if profile.ValidRootName(source.Root) {
			out[source.Root] = source.Root
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func namedRootSourceURI(profileDir, target string, namedRoots map[string]string) (string, bool) {
	if len(namedRoots) == 0 {
		return "", false
	}
	targetReal, err := realPath(target)
	if err != nil {
		return "", false
	}

	type candidate struct {
		name string
		rel  string
		root string
	}
	var best candidate
	names := make([]string, 0, len(namedRoots))
	for name := range namedRoots {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !profile.ValidRootName(name) {
			continue
		}
		rootPath := strings.TrimSpace(namedRoots[name])
		if rootPath == "" {
			continue
		}
		if !filepath.IsAbs(rootPath) {
			rootPath = filepath.Join(profileDir, rootPath)
		}
		rootReal, err := realPath(rootPath)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootReal, targetReal)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		if best.name == "" || len(rootReal) > len(best.root) {
			best = candidate{name: name, rel: filepath.ToSlash(rel), root: rootReal}
		}
	}
	if best.name == "" {
		return "", false
	}
	return fmt.Sprintf("local:%s//%s", best.name, best.rel), true
}

func realPath(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(real), nil
}

func cleanOrDefault(path, fallback string) string {
	if path == "" {
		path = fallback
	}
	return filepath.Clean(path)
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

func (l lockSkill) info() LockSkillInfo {
	return LockSkillInfo{
		Source:          l.Source,
		SourceType:      l.SourceType,
		SourceURL:       l.SourceURL,
		SkillPath:       l.SkillPath,
		SkillFolderHash: l.SkillFolderHash,
		InstalledAt:     l.InstalledAt,
		UpdatedAt:       l.UpdatedAt,
	}
}
