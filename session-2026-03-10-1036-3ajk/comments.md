## Yak Wrap — 2026-03-10 11:48

### Highlights

- yak-map Y key now copies yak IDs to clipboard on macOS — was completely broken due to Linux-only /proc dependency
- Fixed misleading help text in yak-box spawn that referenced a `.tasks` convention that doesn't exist

### Shaved Yaks

**yakthang improvements**
- **yak-map Y key clipboard broken on mac** — Added macOS support to `copy_via_zellij_tty()`: lsof-based PTY detection for OSC 52, with pbcopy as native fallback. Linux /proc path unchanged. Early exit on each path prevents double-copy. Reviewed and passed sniff test. (9c6460d)

**yak-box CLI improvements**
- **misleading yak-path example in spawn help** — Replaced `.tasks` with `.yaks-staging` in the `--yak-path` example so it doesn't suggest a non-existent convention. (815a178)

### New Yaks Added (not shaved)

- **investigate difftastic integration** — research item under future and research
- **investigate ast-grep** — research item under future and research

### Session Stats

- Duration: 10:36 → 11:48 (~1h12m)
- Shavers spawned: 2 (Yakira, Yakoff)
- Yaks shaved: 2/2
- Yaks added: 2 (research, parked)
