# Yak Wrap — 2026-03-08

## Highlights

- Patched a Dependabot vulnerability in yak-map without waiting for upstream zellij-tile
- Got Claude Code working inside sandboxed Docker workers — collapsed two Dockerfiles into one
- Stood up a new yakthang workspace for wellmaintained-packages, fully symlinked to the main repo
- Evaluated desloppify for code quality — worth adopting for Go, skip for Rust

## Shaved Yaks

### Standalone
- **fix atty dep** — Dependabot alert for `atty` (RUSTSEC-2021-0145). Transitive dep via zellij-tile → clap 3. Created `atty-stub/` with safe reimplementation using `std::io::IsTerminal`. All 46 tests pass.
- **setup wellmaintained-packages yakthang** — New workspace at `~/Work/wellmaintained-packages-yakthang`. Skills, agents, and yakstead all symlinked back to main yakthang repo. Lightweight — just project-specific CLAUDE.md, Justfile, and .claude/settings.json.

### yakthang improvements > future and research
- **sandbox demo3** — Smoke test for sandboxed Claude OAuth. Revealed: Claude Code missing from worker image, `--network=none` blocking OAuth token exchange, and yx dynamically linked against libs not in Ubuntu 24.04. Fixed the first two; yx in sandbox deferred to static-link-yx yak.

### yakthang improvements > yak-box CLI improvements
- **investigate desloppify for yakthang** — Ran desloppify on both yak-map (Rust) and yak-box (Go). Rust: skip (clippy integration broken, codebase too small). Go: worth adopting (96.3/100 objective score, good structural debt tracking). Created follow-up yak to configure it.

## Untracked Fixes (committed directly by Yakob)

- **yak-box symlink skill copy** — `filepath.Walk` uses `Lstat`, so symlinked skill dirs failed with "is a directory". Added `filepath.EvalSymlinks` resolution. Tests added.
- **Justfile** — Fixed yak-map.wasm filename (underscores → hyphens), added `just launch` recipe.
- **Collapsed nested devcontainer** — Deleted `src/yak-box/.devcontainer/Dockerfile`, moved Claude Code install + config pre-seed into root `.devcontainer/Dockerfile`.
- **OAuth network fix** — Changed `--network=none` to `--network=bridge` in auth login container.

## Interesting Findings

- The Claude Code native installer puts the binary in `~/.local/bin/claude` — fine for root, invisible to non-root container users. Had to `cp` to `/usr/local/bin/`.
- `yx` is dynamically linked against 6 shared libs including libgit2 1.9. Ubuntu 24.04 only ships 1.7. Static linking is the proper fix — tracked in static-link-yx yak.
- Yakira independently created two new yaks during her setup work: "automate workspace setup for new yakthang projects" and "fix tilde paths in yak-box prompt.txt". Industrious shaver.

## Loose Ends

- yx doesn't work in sandboxed containers (needs static linking)
- desloppify's `.desloppify/` dir was created in `src/yak-map/` by Yakira — may want to gitignore or clean up
- Dependabot alert may persist even with the patch (GitHub may not recognize `[patch.crates-io]` overrides in Cargo.lock)
