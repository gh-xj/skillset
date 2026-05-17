package skillsetcli

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/gh-xj/skillset/internal/cliruntime"
	"github.com/gh-xj/skillset/internal/log"
	"github.com/gh-xj/skillset/internal/profile"
)

const binaryName = "skillset"

var (
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

type CLI struct {
	Verbose     bool             `short:"v" help:"enable debug logs"`
	NoColor     bool             `name:"no-color" help:"disable colorized output"`
	JSON        bool             `help:"emit machine-readable JSON output"`
	Profile     string           `help:"path to skills.profile.yaml" default:"skills.profile.yaml" type:"path"`
	Home        string           `help:"home directory for user skill roots" type:"path"`
	Repo        string           `help:"repo directory for repo skill roots" default:"." type:"path"`
	VersionFlag kong.VersionFlag `name:"version" help:"print version and exit"`

	Validate  ValidateCmd  `cmd:"" help:"validate a skills profile"`
	Normalize NormalizeCmd `cmd:"" help:"emit a normalized skills profile"`
	List      ListCmd      `cmd:"" help:"list skills declared by a profile"`
	Roots     RootsCmd     `cmd:"" help:"show resolved skill roots"`
	Check     CheckCmd     `cmd:"" help:"check profile and current skill state"`
	Diff      DiffCmd      `cmd:"" help:"show planned read-only skill changes"`
	Doctor    DoctorCmd    `cmd:"" help:"diagnose profile and environment"`
	Discover  DiscoverCmd  `cmd:"" help:"discover existing skills and suggest profile entries"`
	Adopt     AdoptCmd     `cmd:"" help:"record matching existing skills as managed"`
	Managed   ManagedCmd   `cmd:"" help:"list skillset-managed entries"`
	Apply     ApplyCmd     `cmd:"" help:"apply missing desired user skills non-destructively"`
	Prune     PruneCmd     `cmd:"" help:"delete managed entries no longer desired"`
	Version   VersionCmd   `cmd:"" help:"print build metadata"`

	out io.Writer
	err io.Writer
}

func (c *CLI) stdout() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

func (c *CLI) stderr() io.Writer {
	if c.err != nil {
		return c.err
	}
	return os.Stderr
}

func (c *CLI) profilePath() string {
	path := strings.TrimSpace(c.Profile)
	if path == "" {
		path = "skills.profile.yaml"
	}
	return filepath.Clean(path)
}

func (c *CLI) loadProfile() (profile.Profile, string, error) {
	path := c.profilePath()
	p, err := profile.LoadFile(path)
	if err != nil {
		return profile.Profile{}, path, err
	}
	return p.Normalized(), path, nil
}

func Execute(args []string) int {
	return execWriters(args, os.Stdout, os.Stderr)
}

func execWriters(args []string, stdout, stderr io.Writer) int {
	cli := CLI{out: stdout, err: stderr}
	return cliruntime.Execute(cliruntime.Options{
		Meta: cliruntime.Meta{
			Name:        binaryName,
			Description: "desired-state manager for agent skills",
			Version:     effectiveAppVersion(),
			Commit:      appCommit,
			Date:        appDate,
		},
		Root:   &cli,
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
		BeforeRun: func() {
			log.Setup(log.Options{Verbose: cli.Verbose, NoColor: cli.NoColor, Writer: stderr})
		},
	})
}
