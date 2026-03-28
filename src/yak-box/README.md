# yak-box

Docker-based worker orchestration CLI for managing devcontainer and native workers.

## Build

```bash
go build -o yak-box .
```

## What it does

yak-box spawns, manages, and stops containerized worker environments. It provides commands for:

- **spawn** - Start a new worker (devcontainer via Docker or native)
- **stop** - Stop a running worker
- **check** - Verify environment and prerequisites
- **message** - Send messages to workers

## Worktrees Field Convention

Yaks can declare extra repositories that should be attached to a worker by
setting a `worktrees` field on the yak.

Example:

```bash
yx field sc-12345 worktrees "repos/releng/release,repos/releng/monix"
```

Convention:

- Field name is `worktrees`
- Value is plain text in `.yaks/<yak-path>/worktrees`
- Value format is comma-separated repo paths with no spaces
- Paths are relative to the workspace root (where `.yaks/` lives)
- Each listed path must point to an existing git repository

Branch naming:

- The top-level yak name is used as the branch name for all created worktrees
- For yak `sc-12345`, all listed repos are worktree-checked out on branch `sc-12345`

This convention lets one yak coordinate the same branch name across multiple
repositories.

## Worktree Cleanup

`yak-box stop` removes the worker home directory, which deletes the checked-out
worktree directories. Source repositories can still retain stale worktree
references until they are pruned.

For each source repository listed in the yak `worktrees` field, run:

```bash
git -C repos/releng/release worktree prune
git -C repos/releng/monix worktree prune
```

This keeps `git worktree list` clean and prevents stale entries from
accumulating over time.
