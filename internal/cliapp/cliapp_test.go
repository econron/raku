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
	config := cmd.Command("config")
	if config == nil {
		t.Fatal("config command is missing")
	}
	for _, name := range []string{"show", "path", "watch"} {
		if config.Command(name) == nil {
			t.Fatalf("config %s command is missing", name)
		}
	}
	configWatch := config.Command("watch")
	for _, name := range []string{"add-repo", "remove-repo", "set-interval", "list"} {
		if configWatch.Command(name) == nil {
			t.Fatalf("config watch %s command is missing", name)
		}
	}
	watch := cmd.Command("watch")
	if watch == nil {
		t.Fatal("watch command is missing")
	}
	reviewRequest := watch.Command("review-request")
	if reviewRequest == nil {
		t.Fatal("watch review-request command is missing")
	}
	if !hasFlag(reviewRequest, "interval") {
		t.Fatal("watch review-request --interval flag is missing")
	}
	if !hasFlag(reviewRequest, "once") {
		t.Fatal("watch review-request --once flag is missing")
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
