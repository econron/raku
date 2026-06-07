# raku debug guide

This guide shows how to test `raku` locally against a real GitHub PR.

The examples use:

```bash
/Users/okuyamaaron/Desktop/raku
/Users/okuyamaaron/Desktop/ink
```

`raku` reads the current working directory as the target GitHub repository, so build the binary from this repo, then run it from the PR repository.

## 1. Build raku

```bash
cd /Users/okuyamaaron/Desktop/raku
go test ./...
go build -o /private/tmp/raku ./cmd/raku
/private/tmp/raku --help
```

## 2. Check the target repository

Run these from the repository that has the PR you want to inspect.

```bash
cd /Users/okuyamaaron/Desktop/ink
gh auth status
gh pr view --json number,title,url,author,headRefName,baseRefName
```

If the current branch has no PR, list your open PRs and check out one.

```bash
gh pr list --author @me --state open
gh pr checkout <pr-number>
```

## 3. Use isolated debug state

Use a temporary state directory so local testing does not touch your normal raku state at `~/.local/state/raku/state.json`.

```bash
export XDG_STATE_HOME=/private/tmp/raku-debug-state
```

To reset the debug state:

```bash
rm -rf /private/tmp/raku-debug-state
export XDG_STATE_HOME=/private/tmp/raku-debug-state
```

## 4. Show PR comments

By default, raku excludes comments authored by the current `gh` user.

```bash
/private/tmp/raku get pr-comment --all
```

For local testing, include your own comments.

```bash
/private/tmp/raku get pr-comment --all --include-self
/private/tmp/raku get pr-comment --all --include-self --json
```

## 5. Baseline current comments

Mark all current comments as seen. Include `--include-self` if your test comments were authored by you.

```bash
/private/tmp/raku seen pr-comment --baseline --include-self
```

After baselining, this should show no new or updated comments until a comment is added or edited.

```bash
/private/tmp/raku get pr-comment --include-self
```

## 6. Test new or updated comments

Add or edit a PR comment on GitHub, then run:

```bash
/private/tmp/raku get pr-comment --include-self
```

Mark one displayed item as seen:

```bash
/private/tmp/raku seen pr-comment 1
```

Confirm it no longer appears:

```bash
/private/tmp/raku get pr-comment --include-self
```

If multiple items are displayed:

```bash
/private/tmp/raku seen pr-comment 1 3
/private/tmp/raku seen pr-comment 1-3
/private/tmp/raku seen pr-comment --all
```

## 7. Inspect state

```bash
cat /private/tmp/raku-debug-state/raku/state.json
```

`current_view` stores the latest displayed aliases. Aliases such as `[1]` are temporary and only refer to the latest `raku get pr-comment` output.

## Common failures

### `not a git repository`

Run `/private/tmp/raku` from the target PR repository, not from this repository unless this repository itself has the PR.

```bash
cd /Users/okuyamaaron/Desktop/ink
/private/tmp/raku get pr-comment --all --include-self
```

### `gh pr view` fails

The current branch probably has no PR.

```bash
gh pr list --author @me --state open
gh pr checkout <pr-number>
```

### Your own comment does not appear

Use `--include-self`.

```bash
/private/tmp/raku get pr-comment --all --include-self
```

### First run does not show comments

If the PR has no raku state yet, normal unread mode shows setup guidance. Use one of these:

```bash
/private/tmp/raku get pr-comment --all --include-self
/private/tmp/raku seen pr-comment --baseline --include-self
```

### Codex shows `lookup api.github.com: no such host`

This usually means the Codex command sandbox cannot reach the network. If `gh auth status` works in your normal terminal, do not re-run `gh auth login`; retry the gh-backed command with network approval in Codex.

### Alias does not match the comment you expected

Aliases are only valid for the latest `raku get pr-comment` output. Run `get` again before `seen` if in doubt.

```bash
/private/tmp/raku get pr-comment --include-self
/private/tmp/raku seen pr-comment 1
```
