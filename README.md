# raku

`raku` is a small CLI for managing GitHub PR review work.

It currently has two main jobs:

- Track unread or updated PR comments on the PR for the current branch.
- Watch configured repositories for PR review requests assigned to you.

GitHub authentication and API access are delegated to the GitHub CLI (`gh`).

## Requirements

- Go
- GitHub CLI (`gh`)
- An authenticated `gh` session:

```bash
gh auth login -h github.com
```

`raku get pr-comment` must be run from a GitHub repository checkout. It uses the current branch's PR via `gh pr view`.

## Install / Update

Install from the Homebrew tap:

```bash
brew update
brew install econron/tap/raku
```

Update an existing installation:

```bash
brew update
brew upgrade econron/tap/raku
```

## Build

```bash
go test ./...
go build -o /private/tmp/raku ./cmd/raku
```

Use `/private/tmp/raku` in the examples below, or replace it with your installed `raku` binary.

## PR Comments

Show new or updated comments on the PR for the current branch:

```bash
raku get pr-comment
```

Show all current PR comments, including seen comments:

```bash
raku get pr-comment --all
```

Output JSON:

```bash
raku get pr-comment --json
raku get pr-comment --all --json
```

By default, comments authored by the current `gh` user are excluded. Include your own comments for local testing:

```bash
raku get pr-comment --include-self
raku get pr-comment --all --include-self
```

Mark displayed items as seen. Aliases such as `1` refer only to the latest `raku get pr-comment` output:

```bash
raku seen pr-comment 1
raku seen pr-comment 1 3
raku seen pr-comment 1-3
raku seen pr-comment --all
```

Create a baseline from all current comments:

```bash
raku seen pr-comment --baseline
```

Include your own comments when creating a baseline:

```bash
raku seen pr-comment --baseline --include-self
```

## Config

Show the config path:

```bash
raku config path
```

Show current config as JSON:

```bash
raku config show
```

Manage repositories watched for review requests:

```bash
raku config watch add-repo owner/repo
raku config watch remove-repo owner/repo
raku config watch list
```

Set the watch interval:

```bash
raku config watch set-interval 30m
raku config watch set-interval 1h
```

The config file is stored at:

- `$XDG_CONFIG_HOME/raku/config.json`
- `~/.config/raku/config.json` if `XDG_CONFIG_HOME` is not set

The JSON shape is:

```json
{
  "watch": {
    "interval": "30m",
    "repositories": [
      "owner/repo"
    ]
  }
}
```

## Review Request Watch

Watch configured repositories for PR review requests assigned to you:

```bash
raku watch review-request
```

Override the configured interval:

```bash
raku watch review-request --interval 1h
```

Run one polling pass and exit:

```bash
raku watch review-request --once
```

The watcher uses this GitHub search condition for each configured repository:

```txt
review-requested:@me
```

On macOS, `raku` sends desktop notifications via `osascript`. If desktop notification support is unavailable, it falls back to stdout.

The same active review request is not notified repeatedly. Once a request disappears, it is removed from state, so a future re-request can notify again.

## State

`raku` stores local state outside the repository:

- `$XDG_STATE_HOME/raku/state.json`
- `~/.local/state/raku/state.json` if `XDG_STATE_HOME` is not set

State contains:

- PR comment seen fingerprints
- The latest `current_view` alias mapping
- Review requests already notified by the watcher

## Debug Tips

If your own PR comment does not appear, use:

```bash
raku get pr-comment --all --include-self
```

If `raku watch review-request` reports no active requests, compare with `gh` directly:

```bash
gh pr list \
  --repo owner/repo \
  --state open \
  --search 'review-requested:@me' \
  --limit 100 \
  --json number,title,url,author,updatedAt,isDraft
```

If running inside Codex and you see:

```txt
lookup api.github.com: no such host
```

that usually means the command sandbox cannot reach the network. If `gh auth status` works in your normal terminal, retry the gh-backed command with network approval rather than running `gh auth login` again.

Stop a foreground watcher with `Ctrl-C`. From another terminal:

```bash
pkill -f 'raku watch review-request'
```
