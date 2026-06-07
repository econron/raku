package cliapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"raku/internal/app"
	"raku/internal/config"
	"raku/internal/gh"
	"raku/internal/notify"
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
			configCommand(stdout),
			watchCommand(stdout),
		},
		Writer:    stdout,
		ErrWriter: stderr,
	}
	return cmd
}

func configCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Manage raku config",
		Commands: []*cli.Command{
			{
				Name:  "show",
				Usage: "Show current config as JSON",
				Action: func(context.Context, *cli.Command) error {
					cfg, err := config.LoadOrDefault("")
					if err != nil {
						return err
					}
					encoder := json.NewEncoder(stdout)
					encoder.SetIndent("", "  ")
					return encoder.Encode(cfg)
				},
			},
			{
				Name:  "path",
				Usage: "Show config file path",
				Action: func(context.Context, *cli.Command) error {
					path, err := config.DefaultPath()
					if err != nil {
						return err
					}
					fmt.Fprintln(stdout, path)
					return nil
				},
			},
			{
				Name:  "watch",
				Usage: "Manage watch config",
				Commands: []*cli.Command{
					{
						Name:      "add-repo",
						Usage:     "Add a repository to watch",
						ArgsUsage: "owner/repo",
						Action: func(_ context.Context, cmd *cli.Command) error {
							if cmd.NArg() != 1 {
								return errors.New("usage: raku config watch add-repo owner/repo")
							}
							cfg, err := config.LoadOrDefault("")
							if err != nil {
								return err
							}
							repo := cmd.Args().First()
							if err := cfg.AddWatchRepository(repo); err != nil {
								return err
							}
							if err := config.Save("", cfg); err != nil {
								return err
							}
							fmt.Fprintf(stdout, "Added watch repository: %s\n", repo)
							return nil
						},
					},
					{
						Name:      "remove-repo",
						Usage:     "Remove a repository from watch config",
						ArgsUsage: "owner/repo",
						Action: func(_ context.Context, cmd *cli.Command) error {
							if cmd.NArg() != 1 {
								return errors.New("usage: raku config watch remove-repo owner/repo")
							}
							cfg, err := config.LoadOrDefault("")
							if err != nil {
								return err
							}
							repo := cmd.Args().First()
							if err := cfg.RemoveWatchRepository(repo); err != nil {
								return err
							}
							if err := config.Save("", cfg); err != nil {
								return err
							}
							fmt.Fprintf(stdout, "Removed watch repository: %s\n", repo)
							return nil
						},
					},
					{
						Name:      "set-interval",
						Usage:     "Set watch interval",
						ArgsUsage: "duration",
						Action: func(_ context.Context, cmd *cli.Command) error {
							if cmd.NArg() != 1 {
								return errors.New("usage: raku config watch set-interval 30m")
							}
							cfg, err := config.LoadOrDefault("")
							if err != nil {
								return err
							}
							interval := cmd.Args().First()
							if err := cfg.SetWatchInterval(interval); err != nil {
								return err
							}
							if err := config.Save("", cfg); err != nil {
								return err
							}
							fmt.Fprintf(stdout, "Set watch interval: %s\n", interval)
							return nil
						},
					},
					{
						Name:  "list",
						Usage: "List watch config",
						Action: func(context.Context, *cli.Command) error {
							cfg, err := config.LoadOrDefault("")
							if err != nil {
								return err
							}
							interval, err := cfg.WatchInterval("")
							if err != nil {
								return err
							}
							fmt.Fprintf(stdout, "interval: %s\n", interval)
							if len(cfg.Watch.Repositories) == 0 {
								fmt.Fprintln(stdout, "repositories: (none)")
								return nil
							}
							fmt.Fprintln(stdout, "repositories:")
							for _, repo := range cfg.Watch.Repositories {
								fmt.Fprintf(stdout, "- %s\n", repo)
							}
							return nil
						},
					},
				},
			},
		},
	}
}

func watchCommand(stdout io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "watch",
		Usage: "Watch resources",
		Commands: []*cli.Command{
			{
				Name:  "review-request",
				Usage: "Watch review requests for configured repositories",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "interval", Usage: "override watch interval, e.g. 30m or 1h"},
					&cli.BoolFlag{Name: "once", Usage: "poll once and exit"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runWatchReviewRequest(ctx, stdout, cmd.String("interval"), cmd.Bool("once"))
				},
			},
		},
	}
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

func newWatchService(stdout io.Writer) (*app.WatchService, error) {
	statePath, err := state.DefaultPath()
	if err != nil {
		return nil, err
	}
	return &app.WatchService{
		GitHub:   gh.NewClient(nil),
		State:    state.NewStore(statePath),
		Notifier: notify.ReviewRequestNotifier{Writer: stdout},
	}, nil
}

func runWatchReviewRequest(ctx context.Context, stdout io.Writer, intervalOverride string, once bool) error {
	cfg, err := config.Load("")
	if err != nil {
		return formatError(err)
	}
	repos, err := cfg.WatchRepositories()
	if err != nil {
		return err
	}
	interval, err := cfg.WatchInterval(intervalOverride)
	if err != nil {
		return err
	}

	service, err := newWatchService(stdout)
	if err != nil {
		return err
	}

	watchCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stdout, "Watching review requests for %d repo(s) every %s.\n", len(repos), interval)
	if err := pollReviewRequests(watchCtx, stdout, service, repos); err != nil {
		return formatError(err)
	}
	if once {
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-watchCtx.Done():
			fmt.Fprintln(stdout, "Stopped watching review requests.")
			return nil
		case <-ticker.C:
			if err := pollReviewRequests(watchCtx, stdout, service, repos); err != nil {
				fmt.Fprintf(stdout, "watch error: %v\n", formatError(err))
			}
		}
	}
}

func pollReviewRequests(ctx context.Context, stdout io.Writer, service *app.WatchService, repos []string) error {
	result, err := service.PollReviewRequests(ctx, repos)
	fmt.Fprintf(
		stdout,
		"checked %d repo(s): active=%d notified=%d cleared=%d\n",
		result.CheckedRepos,
		result.Active,
		result.Notified,
		result.Cleared,
	)
	return err
}

func formatError(err error) error {
	var missingConfig *config.MissingError
	if errors.As(err, &missingConfig) {
		return fmt.Errorf(`raku config file was not found.

Create it with:
  raku config watch add-repo owner/repo
  raku config watch set-interval 30m

Config path:
  %s`, missingConfig.Path)
	}

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
