package main

import (
	"os"

	"github.com/gh-xj/skillset/internal/skillsetcli"
)

func main() {
	os.Exit(skillsetcli.Execute(os.Args[1:]))
}
