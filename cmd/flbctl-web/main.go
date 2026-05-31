package main

import (
	"fmt"
	"os"

	"flbctl/web"
)

func main() {
	if err := web.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
