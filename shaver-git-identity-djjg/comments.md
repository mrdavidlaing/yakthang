Investigation found both runtimes already pass through host git identity correctly via GIT_CONFIG_GLOBAL. Native runtime: sets GIT_CONFIG_GLOBAL to host ~/.gitconfig in run.sh wrapper (native.go:230). Sandboxed runtime: mounts host .gitconfig as read-only volume and sets GIT_CONFIG_GLOBAL env var (helpers.go:134-155).

Chose Option 3 (hybrid): keep host identity as git author, add shaver name to Co-Authored-By trailer for per-shaver traceability. Updated skills/yak-brand/SKILL.md to use $YAK_SHAVER_NAME env var (already available in both runtimes). Commits from shavers will now show e.g. "Co-Authored-By: Yakoff (Claude) <noreply@anthropic.com>" instead of generic "Co-Authored-By: Claude <noreply@anthropic.com>".

Existing deployed shaver skill copies (in .yak-boxes/@home/*) are not updated — they will pick up the change when next spawned from the source skill.
