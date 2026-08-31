package main

import (
	"os"

	"aegion-dynamic/apismith/cli"
)

// console is an alias for `apismith ui`. With no arguments it starts the
// explorer; any other arguments are forwarded to the apismith CLI.
func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "ui")
	}
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
