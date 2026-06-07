package cliapp

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"raku/internal/app"
	"raku/internal/gh"
	"raku/internal/output"
	"raku/internal/state"
)

func NewCommand(stdout, stderr io.Writer) *cli.Command {
	cmd := &cli.Command{
		Name:  "raku",
		Usage: "Manage GitHub PR comment read state",
		Commands: []*cli.Command{
			getCommand(stdout),
			seenCommand(stdout),
		},
		Writer:    stdout,
		ErrWriter: stderr,
	}
	return cmd
}

func getCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "get",
		Usage: "Get resources",
		Commands: []*cli.Command{
			{
				Name:  "pr-comment",
				Usage: "Show new or updated comments on the current branch PR",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "all", Usage: "show all current PR comments"},
					&cli.BoolFlag{Name: "json", Usage: "output JSON"},
					&cli.BoolFlag{Name: "include-self", Usage: "include comments authored by the current gh user"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					service, err := newService()
					if err != nil {
						return err
					}
					result, err := service.Get(ctx, app.GetOptions{
						All:         cmd.Bool("all"),
						IncludeSelf: cmd.Bool("include-self"),
					})
					if err != nil {
						return formatError(err)
					}
					if cmd.Bool("json") {
						return output.WriteGetJSON(stdout, result)
					}
					output.WriteGetText(stdout, result)
					return nil
				},
			},
		},
	}
}

func seenCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "seen",
		Usage: "Mark resources as seen",
		Commands: []*cli.Command{
			{
				Name:      "pr-comment",
				Usage:     "Mark PR comment aliases as seen",
				ArgsUsage: "[alias | alias-range ...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "all", Usage: "mark all items from current_view as seen"},
					&cli.BoolFlag{Name: "baseline", Usage: "mark all current PR comments as seen baseline"},
					&cli.BoolFlag{Name: "include-self", Usage: "include comments authored by the current gh user when creating a baseline"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					service, err := newService()
					if err != nil {
						return err
					}
					result, err := service.Seen(ctx, app.SeenOptions{
						All:         cmd.Bool("all"),
						Baseline:    cmd.Bool("baseline"),
						IncludeSelf: cmd.Bool("include-self"),
						Args:        cmd.Args().Slice(),
					})
					if err != nil {
						return formatError(err)
					}
					output.WriteSeenText(stdout, result)
					return nil
				},
			},
		},
	}
}

func newService() (*app.Service, error) {
	statePath, err := state.DefaultPath()
	if err != nil {
		return nil, err
	}
	return &app.Service{
		GitHub: gh.NewClient(nil),
		State:  state.NewStore(statePath),
	}, nil
}

func formatError(err error) error {
	var preflight *gh.PreflightError
	if errors.As(err, &preflight) {
		switch preflight.Kind {
		case gh.PreflightMissingGH:
			return errors.New(`GitHub CLI (gh) was not found in PATH.

Install gh, then retry:
  raku get pr-comment`)
		case gh.PreflightAuth:
			return fmt.Errorf(`GitHub CLI is not authenticated.

Run:
  gh auth login -h github.com

Then retry:
  raku get pr-comment

Detail:
%s`, preflight.Detail)
		case gh.PreflightNetwork:
			return fmt.Errorf(`GitHub API is not reachable from this process.

Detail:
%s

If this is running inside Codex or another sandbox, the local gh login may already be valid.
Retry the gh-backed command with network approval instead of running gh auth login again.`, preflight.Detail)
		}
	}
	return err
}
