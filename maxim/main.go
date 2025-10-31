package main

import (
	"fmt"
	"os"

	"github.com/ASHUTOSH-SWAIN-GIT/maxim/cmd"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Add version flag if requested
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("maxim version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}
	
	cmd.Execute()
}
