## Yak Wrap — 2026-03-10 14:03

### Highlights

- Sandboxed workers can now use yx natively — the Dockerfile pulls the upstream statically-linked release binary instead of copying a locally-built dynamic one.

### Shaved Yaks

#### worker runtime fixes
- **static link yx** — Discovered the upstream release binary is already statically linked. Replaced the COPY-based install in the Dockerfile with the upstream `install.sh` script. Removed yx binary staging from `setup-vm.sh`. Added `unzip` to apt deps. Verified with `ldd` in Ubuntu 24.04 container.

### Interesting Findings

- The upstream yx binary was statically linked all along — the problem was our build/install path, not the binary itself. No Cargo.toml changes needed.

### Loose Ends

- None identified this session.

---

## Yak Wrap — 2026-03-10 21:03

### Highlights

- Sandboxed workers can now use yx natively via upstream statically-linked binary (from earlier spawn)
- Shavers now get per-shaver attribution in Co-Authored-By trailers, with host git identity passed through correctly in both runtimes

### Shaved Yaks

#### worker runtime fixes
- **static link yx** — Upstream release binary was already statically linked. Replaced COPY-based install in Dockerfile with upstream install.sh. Removed yx staging from setup-vm.sh.
- **shaver git identity** — Both runtimes already pass through host git identity via GIT_CONFIG_GLOBAL. Added explicit env var pinning (GIT_AUTHOR_NAME/EMAIL, GIT_COMMITTER_NAME/EMAIL). Updated yak-brand skill to include shaver name in Co-Authored-By trailer via $YAK_SHAVER_NAME.

### Interesting Findings

- The upstream yx binary was statically linked all along — the problem was our install path, not the binary itself
- Host .gitconfig uses include.path=~/.gitconfig-mrdavidlaing with tilde, which breaks when HOME is overridden — the new explicit env var pinning solves this

### Loose Ends

- shaver git identity changes are uncommitted (sitting in working tree) — Yakoff needs to commit or Yakob needs to handle
- No new unit tests for resolveGitIdentityExports()
- Skill duplication between .claude/skills/yak-brand/SKILL.md and skills/yak-brand/SKILL.md could drift

### Remaining Yaks

See post-prune map below.
