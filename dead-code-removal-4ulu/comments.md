Removed all 4 dead code items:

1. **resolveAnthropicKeySuffix** (helpers.go) — deleted. No callers anywhere in codebase.
2. **messageCmd** (cmd/message.go + cmd/message_test.go) — deleted both files. Defined and tested but never registered with rootCmd. Git log shows it was created during a consolidation; likely an incomplete feature. No registration = unreachable.
3. **generateDevContainerID dead branch** (pkg/devcontainer/variables.go) — SHA-256 (32 bytes) → base32 always produces 56 chars, so `len(encoded) > 52` is always true. Removed the dead `return encoded` else branch.
4. **generateNativeWrapperScript paneName** (internal/runtime/native.go) — simplified from `(content, paneName string)` to `string`. The only production caller discarded paneName. Updated 8 test call sites to match new single-return signature, removing paneName assertions.

Verification: `go build ./...` and `go test ./...` both pass. `shellspec` failures are pre-existing (read-only filesystem in sandbox prevents Go build cache writes — not related to these changes).
