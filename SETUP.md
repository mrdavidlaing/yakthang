# yakthang Setup

Bootstrap instructions for setting up a new yakthang workspace. Follow these steps mechanically — substitute `<yakthang-path>` with the actual path to your yakthang clone (e.g. `~/repos/yakthang`).

---

## Prerequisites

Check that these tools are installed before continuing:

```bash
git --version
zellij --version
cargo --version
go version
just --version
```

Install any that are missing before proceeding.

---

## Steps

### 1. Clone the yakthang repo

If you don't already have it:

```bash
git clone git@github.com:mrdavidlaing/yakthang.git <yakthang-path>
```

If it's already cloned, note the path — you'll use `<yakthang-path>` throughout.

### 2. Build and install tools

```bash
cd <yakthang-path>
just install
```

This builds and installs `yx` and `yak-box` to `~/.local/bin/`. Verify:

```bash
yx --version
yak-box --help
```

### 3. Create skill and agent directories

In the workspace root where you'll run yakthang (your project directory):

```bash
mkdir -p .claude/skills .claude/agents
```

### 4. Symlink each skill

```bash
ln -s <yakthang-path>/skills/yak-brand .claude/skills/yak-brand
ln -s <yakthang-path>/skills/yak-mapping .claude/skills/yak-mapping
ln -s <yakthang-path>/skills/yak-shaving-handbook .claude/skills/yak-shaving-handbook
ln -s <yakthang-path>/skills/yak-sniff-test .claude/skills/yak-sniff-test
ln -s <yakthang-path>/skills/yak-triage .claude/skills/yak-triage
ln -s <yakthang-path>/skills/yak-wrap .claude/skills/yak-wrap
```

### 5. Symlink the Yakob agent

```bash
ln -s <yakthang-path>/agents/yakob.md .claude/agents/yakob.md
```

### 6. Symlink yakstead files

```bash
ln -s <yakthang-path>/yakstead/orchestrator.kdl orchestrator.kdl
ln -s <yakthang-path>/yakstead/launch.sh launch.sh
```

### 7. Initialize task state

```bash
mkdir -p .yaks
```

### 8. Validate

```bash
yx ls
yak-box --help
cat orchestrator.kdl
```

`yx ls` should print an empty task list (or your existing tasks). `yak-box --help` should print usage. `cat orchestrator.kdl` should show the Zellij layout config.

---

## Notes

- `~/.local/bin/` must be on your `$PATH`. Add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile if needed.
- The yak-map WASM plugin requires a separate install step: `just install-yak-map` prints the path after building.
- Symlinks in `.claude/skills/` and `.claude/agents/` are relative to the workspace root — run the `ln -s` commands from that directory.
