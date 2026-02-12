package main

import (
	"fmt"
	"os"

	"github.com/smallnest/goclaw/cli"
)

// Version information, populated by goreleaser
var (
	Version   = "dev"
	Commit    = "unknown"
	Date      = "unknown"
	BuiltBy   = "unknown"
)

func main() {
	// Set version in CLI package
	cli.SetVersion(Version)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

func GetVersionInfo() string {
	return fmt.Sprintf("goclaw version %s (commit: %s, built at: %s by: %s)", Version, Commit, Date, BuiltBy)
}
