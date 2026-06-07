package main

import (
	"context"
	"fmt"
	"os"

	"raku/internal/cliapp"
)

func main() {
	cmd := cliapp.NewCommand(os.Stdout, os.Stderr)
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
