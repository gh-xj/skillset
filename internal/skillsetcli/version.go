package skillsetcli

import (
	"fmt"
	"runtime/debug"
	"strings"

	appio "github.com/gh-xj/skillset/internal/io"
)

type VersionCmd struct{}

func (c *VersionCmd) Run(globals *CLI) error {
	version, source := detectAppVersion()
	data := versionPayload{
		SchemaVersion: "v1",
		Name:          binaryName,
		Version:       version,
		VersionSource: source,
		Commit:        appCommit,
		Date:          appDate,
	}
	if globals.JSON {
		return appio.WriteJSON(globals.stdout(), data)
	}
	_, err := fmt.Fprintf(globals.stdout(), "%s %s\n", data.Name, data.Version)
	return err
}

type versionPayload struct {
	SchemaVersion string `json:"schema_version"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	VersionSource string `json:"version_source"`
	Commit        string `json:"commit"`
	Date          string `json:"date"`
}

func effectiveAppVersion() string {
	version, _ := detectAppVersion()
	return version
}

func detectAppVersion() (string, string) {
	if appVersion != "" && appVersion != "dev" {
		return appVersion, "ldflags"
	}
	if version := goBuildInfoVersion(); version != "" {
		return version, "go_build_info"
	}
	if appVersion != "" {
		return appVersion, "default"
	}
	return "dev", "default"
}

func goBuildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	version := strings.TrimSpace(info.Main.Version)
	if version == "" || version == "(devel)" {
		return ""
	}
	return version
}
