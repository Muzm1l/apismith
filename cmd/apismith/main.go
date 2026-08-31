package main

import (
	"os"

	"aegion-dynamic/apismith/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
