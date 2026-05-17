package apply

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gh-xj/skillset/internal/planner"
	"github.com/gh-xj/skillset/internal/profile"
	"github.com/gh-xj/skillset/internal/skillfs"
	"github.com/gh-xj/skillset/internal/state"
)

type Runner func(Command) CommandResult

type Options struct {
	Apply       bool
	ProfilePath string
	ToolName    string
	Now         func() time.Time
	Runner      Runner
}

type Command struct {
	Args []string `json:"args"`
}

type CommandResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Err      string `json:"error,omitempty"`
}

type Result struct {
	DryRun      bool        `json:"dry_run"`
	ProfilePath string      `json:"profile_path"`
	StatePath   string      `json:"state_path"`
	EventsPath  string      `json:"events_path"`
	Planned     []Operation `json:"planned"`
	Applied     []Operation `json:"applied"`
	Skipped     []Operation `json:"skipped"`
	Failed      []Operation `json:"failed"`
	Summary     Summary     `json:"summary"`
}

type Operation struct {
	Name          string         `json:"name"`
	Agent         profile.Agent  `json:"agent"`
	Tier          profile.Tier   `json:"tier"`
	Action        string         `json:"action"`
	Source        string         `json:"source"`
	SourcePath    string         `json:"source_path,omitempty"`
	TargetPath    string         `json:"target_path,omitempty"`
	Command       []string       `json:"command,omitempty"`
	Status        string         `json:"status"`
	Reason        string         `json:"reason,omitempty"`
	CommandResult *CommandResult `json:"command_result,omitempty"`
}

type Summary struct {
	Planned int `json:"planned"`
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
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
	if opts.Runner == nil {
		opts.Runner = ExecRunner
	}
	result := Result{
		DryRun:      !opts.Apply,
		ProfilePath: opts.ProfilePath,
		StatePath:   state.StatePathForProfile(opts.ProfilePath),
		EventsPath:  state.EventsPathForProfile(opts.ProfilePath),
	}
	for _, item := range plan.Items {
		op, ok := operationForItem(item)
		if !ok {
			result.Skipped = append(result.Skipped, skippedOperation(item))
			continue
		}
		result.Planned = append(result.Planned, op)
	}
	result.Summary.Planned = len(result.Planned)
	result.Summary.Skipped = len(result.Skipped)
	if !opts.Apply {
		return result, nil
	}

	var managed []state.ManagedEntry
	for _, op := range result.Planned {
		applied, entry, err := applyOperation(op, opts, now)
		if err != nil {
			applied.Status = "failed"
			applied.Reason = err.Error()
			result.Failed = append(result.Failed, applied)
			continue
		}
		applied.Status = "applied"
		result.Applied = append(result.Applied, applied)
		managed = append(managed, entry)
	}
	result.Summary.Applied = len(result.Applied)
	result.Summary.Failed = len(result.Failed)
	if len(managed) == 0 {
		return result, nil
	}
	store, err := state.Load(result.StatePath)
	if err != nil {
		return Result{}, err
	}
	store = state.MergeManaged(store, managed)
	if err := state.Save(result.StatePath, store); err != nil {
		return Result{}, err
	}
	for _, entry := range managed {
		if err := state.AppendEvent(result.EventsPath, state.Event{
			ID:         eventID("apply", entry, now),
			Operation:  "apply",
			Status:     "applied",
			Agent:      entry.Agent,
			Tier:       entry.Tier,
			Name:       entry.Name,
			TargetPath: entry.TargetPath,
			Source:     entry.Source,
			Message:    "applied desired skill and recorded it as skillset-managed",
			Timestamp:  now,
		}); err != nil {
			return Result{}, err
		}
		result.Summary.Written++
	}
	return result, nil
}

func ExecRunner(cmd Command) CommandResult {
	if len(cmd.Args) == 0 {
		return CommandResult{ExitCode: 1, Err: "empty command"}
	}
	var stdout, stderr bytes.Buffer
	command := exec.Command(cmd.Args[0], cmd.Args[1:]...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result
	}
	result.ExitCode = 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	}
	result.Err = err.Error()
	return result
}

func operationForItem(item planner.Item) (Operation, bool) {
	if item.Status != planner.StatusMissingTarget || item.Tier != profile.TierUser {
		return Operation{}, false
	}
	op := Operation{
		Name:       item.Name,
		Agent:      item.Agent,
		Tier:       item.Tier,
		Action:     item.Action,
		Source:     item.Source,
		SourcePath: item.SourcePath,
		TargetPath: item.TargetPath,
		Status:     "planned",
		Reason:     item.Reason,
	}
	if item.Action == planner.ActionInstallGitHub {
		source, err := profile.ParseSource(item.Source)
		if err != nil {
			op.Status = "skipped"
			op.Reason = err.Error()
			return op, false
		}
		op.Command = githubInstallCommand(item, source)
	}
	return op, item.Action == planner.ActionLinkLocal || item.Action == planner.ActionInstallGitHub
}

func skippedOperation(item planner.Item) Operation {
	reason := item.Reason
	if reason == "" {
		reason = "not a missing user-tier target"
	}
	return Operation{
		Name:       item.Name,
		Agent:      item.Agent,
		Tier:       item.Tier,
		Action:     item.Action,
		Source:     item.Source,
		SourcePath: item.SourcePath,
		TargetPath: item.TargetPath,
		Status:     "skipped",
		Reason:     reason,
	}
}

func applyOperation(op Operation, opts Options, now time.Time) (Operation, state.ManagedEntry, error) {
	switch op.Action {
	case planner.ActionLinkLocal:
		return applyLocal(op, opts, now)
	case planner.ActionInstallGitHub:
		return applyGitHub(op, opts, now)
	default:
		return op, state.ManagedEntry{}, fmt.Errorf("unsupported apply action %q", op.Action)
	}
}

func applyLocal(op Operation, opts Options, now time.Time) (Operation, state.ManagedEntry, error) {
	if op.SourcePath == "" {
		return op, state.ManagedEntry{}, fmt.Errorf("local source path is empty")
	}
	if err := skillfs.ValidateSkillDir(op.SourcePath, op.Name); err != nil {
		return op, state.ManagedEntry{}, fmt.Errorf("local source path is not a valid skill: %w", err)
	}
	if _, err := os.Lstat(op.TargetPath); err == nil {
		return op, state.ManagedEntry{}, fmt.Errorf("target already exists: %s", op.TargetPath)
	} else if !os.IsNotExist(err) {
		return op, state.ManagedEntry{}, fmt.Errorf("inspect target %s: %w", op.TargetPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(op.TargetPath), 0o755); err != nil {
		return op, state.ManagedEntry{}, fmt.Errorf("ensure target parent: %w", err)
	}
	target, err := filepath.Rel(filepath.Dir(op.TargetPath), op.SourcePath)
	if err != nil {
		target = op.SourcePath
	}
	if err := os.Symlink(target, op.TargetPath); err != nil {
		return op, state.ManagedEntry{}, fmt.Errorf("symlink %s -> %s: %w", op.TargetPath, target, err)
	}
	entry, err := managedEntry(op, opts.ToolName, now)
	return op, entry, err
}

func applyGitHub(op Operation, opts Options, now time.Time) (Operation, state.ManagedEntry, error) {
	command := Command{Args: op.Command}
	result := opts.Runner(command)
	op.CommandResult = &result
	if result.ExitCode != 0 || result.Err != "" {
		return op, state.ManagedEntry{}, fmt.Errorf("github install failed: %s", result.Err)
	}
	if _, err := os.Lstat(op.TargetPath); err != nil {
		return op, state.ManagedEntry{}, fmt.Errorf("github install did not create target %s: %w", op.TargetPath, err)
	}
	source, err := profile.ParseSource(op.Source)
	if err != nil {
		return op, state.ManagedEntry{}, err
	}
	if err := skillfs.ValidateGitHubInstall(filepath.Dir(op.TargetPath), op.TargetPath, op.Name, source); err != nil {
		return op, state.ManagedEntry{}, fmt.Errorf("github install did not create a valid npx skills target: %w", err)
	}
	entry, err := managedEntry(op, opts.ToolName, now)
	return op, entry, err
}

func managedEntry(op Operation, toolName string, now time.Time) (state.ManagedEntry, error) {
	info, err := os.Lstat(op.TargetPath)
	if err != nil {
		return state.ManagedEntry{}, fmt.Errorf("inspect target %s: %w", op.TargetPath, err)
	}
	source, err := profile.ParseSource(op.Source)
	if err != nil {
		return state.ManagedEntry{}, err
	}
	entry := state.ManagedEntry{
		Agent:            op.Agent,
		Tier:             op.Tier,
		Name:             op.Name,
		Source:           op.Source,
		SourceScheme:     source.Scheme,
		TargetPath:       op.TargetPath,
		TargetKind:       targetKind(info),
		RecordedBy:       toolName,
		RecordedAt:       now,
		LastSeenAt:       now,
		PruneEligible:    true,
		PruneSafetyNotes: []string{"created by skillset apply"},
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(op.TargetPath); err == nil {
			entry.SymlinkTarget = target
		}
	}
	if source.Scheme == profile.SourceGitHub {
		entry.InstallCommand = append([]string(nil), op.Command...)
	}
	return entry, nil
}

func githubInstallCommand(item planner.Item, source profile.Source) []string {
	return []string{"npx", "skills", "add", source.Owner + "/" + source.Repo, "-g", "-s", item.Name, "-a", string(item.Agent), "-y", "--copy"}
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
