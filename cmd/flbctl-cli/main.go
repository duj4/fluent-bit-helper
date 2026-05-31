package main

import (
	"fmt"
	"os"

	"flbctl/cli"
)

// main is the application entrypoint.

// The main package intentionally stays extremely small.
// All CLI logic lives inside the cli package.

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
