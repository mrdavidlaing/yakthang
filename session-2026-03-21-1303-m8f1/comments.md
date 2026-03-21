## Yak Wrap — 2026-03-21 18:30 (supervised by @mrdavidlaing)

### Highlights

- Completed the entire **yak-map quality journey** — all children now done
- Fixed a rendering bug where done/todo yaks showed stale wip-state emoji
- Improved `yx show` so .md fields always render as long-form sections
- Made comments.md mandatory in the yak-shaving handbook (no more "skip it")

### Shaved Yaks

#### yak-map quality journey
- **make comments.md mandatory** — Updated yak-shaving-handbook SKILL.md to remove the "skip it" escape hatch. Comments.md is now always expected. (Shaved in a previous session, confirmed done today.)
- **only show wip-state emoji when state is wip** — Fixed model.rs and render.rs so wip-state emoji only renders when task state is wip. Done/todo yaks now show clean ✓/○. Added 2 tests, all 65 pass. Shaver: Yakriel (Sonnet). Commit: 5a9c976.

#### yx feature requests
- **render comments.md as long field** — Added `name.ends_with(".md")` check in show_yak.rs field classification. Single-line .md fields now render as ruled sections, not header entries. Test added, all 565 pass. Shaver: Yakueline (Sonnet). Commit: c08f1eb (submodule), 791dad9 (parent).

### Interesting Findings

- Opus spent 20 minutes "Pontificating" on the wip-state emoji task without making a single tool call. Sonnet did it in 2 minutes. For well-scoped mechanical tasks, Sonnet is dramatically more efficient.
- The ast-grep research was started but the agent call blocked Yakob for ~3.5 hours, causing us to blow past the 17:00 hard stop. Long-running foreground agents are a session risk — should use background agents or time-box research.
- Yakueline ran `yx reset` inside the yx submodule, which switched the yak map to the yx project's own task state. Shavers working in submodules need to be careful about yx context.

### Loose Ends

- **fix yak-box tab names and assigned-to values** — Parked (sleeping). Has no context. Needs operator input to define what's actually broken.
- **ast-grep investigation** — Research started but incomplete. Got sandbox architecture mapping done but web research on ast-grep itself was interrupted.
- **sync-push-error-handling** — Context references `let _ = remote.push(...)` but current code already checks push status with bail!. May be stale/already fixed.
- **different-color-wip** — Brief is vague. Current code already differentiates wip (green+bold) from done (grey+strikethrough). Needs operator to clarify what's insufficient.

### Session Notes

- Session ran 13:03–18:30 (planned hard stop: 17:00). Overran due to blocked research agent.
- WIP limit of 1 was respected throughout. Serial shaving worked well for a hands-off session.
- 3 yaks completed. Bob (Opus) was fired after 20min of inaction; Yakriel and Yakueline (both Sonnet) were fast and clean.
