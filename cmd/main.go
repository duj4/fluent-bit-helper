package main

import (
	"fmt"
	"os"

	"fluent-bit-helper/web"
)

func main() {
	if err := web.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
