## Yak Wrap — 2026-03-10

## Highlights

- Fixed misleading `--yak-path` example in `yak-box spawn --help`; now shows a realistic absolute path (`/home/<username>/yakthang/.yaks`) instead of the non-existent `.tasks` convention.

## Shaved Yaks

### yak-box CLI improvements
- **misleading yak-path example in spawn help** — Changed `.tasks` → `.yaks-staging` (initial fix), then updated to `/home/<username>/yakthang/.yaks` per David's preference. Two commits: 815a178, f14e502. Source: `src/yak-box/cmd/spawn.go:72`.

## Loose Ends

- The note in the agent-status references commit 815a178 (first fix); final state is f14e502.

## Remaining Yaks

See map below — no structural changes made this session.
