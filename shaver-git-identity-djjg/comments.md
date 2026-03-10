Root cause: host .gitconfig uses include.path=~/.gitconfig-mrdavidlaing with tilde. Git resolves ~ via HOME, but shavers have HOME overridden to their yak-box dir, so the include silently fails and user.name/user.email are never loaded. Yakthang worked by accident because it has the identity set in its local .git/config.

Fix: resolveGitIdentityExports() in helpers.go shells out to git config --global user.name/email at spawn time (while HOME is still correct) and returns export lines for GIT_AUTHOR_NAME, GIT_AUTHOR_EMAIL, GIT_COMMITTER_NAME, GIT_COMMITTER_EMAIL. These env vars take precedence over config files and do not depend on ~ resolution.

Applied to both runtimes:
- Native (native.go): gitIdentityLines embedded in wrapper script
- Sandboxed (helpers.go generateRunScript): -e flags added to docker run

Also updated yak-brand skill (skills/yak-brand/SKILL.md) to include shaver name in Co-Authored-By trailer when $YAK_SHAVER_NAME is set.
