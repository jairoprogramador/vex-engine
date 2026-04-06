package main

import (
	"github.com/jairoprogramador/vex-engine/cmd/vex/cmd"
)

var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.Execute(version)
}
