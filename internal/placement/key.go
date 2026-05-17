package placement

import "github.com/gh-xj/skillset/internal/profile"

type Key struct {
	Agent      profile.Agent
	Tier       profile.Tier
	Name       string
	Source     string
	TargetPath string
}
