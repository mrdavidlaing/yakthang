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
