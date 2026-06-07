package cliapp

import (
	"bytes"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestNewCommandDefinesMVPCommands(t *testing.T) {
	t.Parallel()

	cmd := NewCommand(&bytes.Buffer{}, &bytes.Buffer{})
	get := cmd.Command("get")
	if get == nil {
		t.Fatal("get command is missing")
	}
	if get.Command("pr-comment") == nil {
		t.Fatal("get pr-comment command is missing")
	}
	if !hasFlag(get.Command("pr-comment"), "include-self") {
		t.Fatal("get pr-comment --include-self flag is missing")
	}
	seen := cmd.Command("seen")
	if seen == nil {
		t.Fatal("seen command is missing")
	}
	if seen.Command("pr-comment") == nil {
		t.Fatal("seen pr-comment command is missing")
	}
	if !hasFlag(seen.Command("pr-comment"), "include-self") {
		t.Fatal("seen pr-comment --include-self flag is missing")
	}
}

func hasFlag(cmd *cli.Command, name string) bool {
	for _, flag := range cmd.VisibleFlags() {
		for _, flagName := range flag.Names() {
			if flagName == name {
				return true
			}
		}
	}
	return false
}
