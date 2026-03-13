## Yak Wrap — 2026-03-13 17:35

**Highlights**
- yak-map now has a bin/dev quality gate — lint, test, complexity analysis, and WASM build all run through a single entry point. This is the foundation for measurable code improvements.

**Shaved Yaks**

*yak-map quality journey*
- **create yak-map bin dev** — Built src/yak-map/bin/dev with 4 subcommands (check/lint/test/cx). Fixed 8 pre-existing clippy warnings to make the gate pass on current code. Complexity thresholds set at baseline: cognitive<=29, cyclomatic<=26. Sniff test passed.

**Interesting Findings**
- rust-code-analysis-cli not available in native runtime env — dev check gracefully skips complexity with a warning. Use nix-shell -p rust-code-analysis for full analysis.
- The update() function is the complexity hotspot (cognitive=28, cyclomatic=25) — natural first target for structural refactors.

**Loose Ends**
- Complexity analysis only runs inside nix-shell — could add rust-code-analysis-cli to the flake devShell

---

## Yak Wrap — 2026-03-13 (evening session)

**Highlights**
- Mapped a 10-yak quality journey for yak-map, structured so each yak teaches a specific yx process pattern — learning through doing
- First two quick wins shipped: compiler warnings enabled, ANSI codes extracted into named constants
- Installed rust-code-analysis-cli via nix profile, making dev cx work without nix-shell
- Updated src/yaks submodule to latest upstream (16 commits)

**Shaved Yaks**

*yak-map quality journey > quick wins*
- **remove allow unused** — Removed #![allow(unused)] from main.rs. Only 1 warning was hiding (unused configuration param). Small yak, but the pattern lands: let the compiler talk.
- **extract color constants** — Extracted 11 ANSI escape codes into an ansi module (RED, GREEN, YELLOW, etc.). Replaced all raw \x1b codes in implementation and tests. Only strip_ansi parser retains raw codes (legitimately parsing them).

*Planning*
- **yak-map quality journey** — Mapped 10 yaks in 4 phases: bin/dev gate -> quick wins (4) -> structural refactors (4) -> capstone (module split + architecture skill). Each yak tagged with the yx pattern it exercises.
- **rust code quality inspiration from yaks** — Yakira's research yak completed. Produced prioritized list of 12 improvements with concrete yx references.

*Housekeeping*
- **yx upstream done checkmark** — PR merged, submodule bumped, feature confirmed working.
- **src/yaks submodule bump** — 16 new commits including prune --exclude-tag, onboarding gitignore prompt, Claude Code plugin scaffold.

**Interesting Findings**
- nix profile install nixpkgs#rust-code-analysis works despite nix search timing out — the package exists, the search just couldn't walk the full package set
- The two quick wins together only changed 82 insertions / 44 deletions — incremental quality improvement without disruption
- BOLD and REVERSE constants were initially flagged as unused (from Yakira's work), then became used when Yakoff added the ansi module — a nice example of the two yaks composing

**Loose Ends**
- rust-code-analysis-cli installed to nix profile but not in flake.nix devShell — other machines won't have it automatically
