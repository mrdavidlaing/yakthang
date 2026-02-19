# Quality Audit: yak-box

**Date:** 2026-02-18  
**Codebase Size:** ~3,347 lines across 21 Go source files + 8 test files  
**Scope:** Comprehensive review across UX, testing, simplicity, Go idioms, and security

---

## Executive Summary

yak-box is a well-structured Docker-based worker orchestration CLI with solid foundations. The codebase demonstrates strong Go idioms (consistent error wrapping with `%w`, proper package organization) and reasonable CLI UX patterns using Cobra. However, several areas present opportunities for improvement:

**Strengths:** Consistent error handling, clear package boundaries, comprehensive devcontainer support, good command documentation with examples.

**Key Concerns:** Minimal test coverage (8 test files covering only ~38% of packages), no input validation for path traversal risks, inconsistent container name prefixes across codebase ("yak-shaver-" vs "yak-worker-"), duplicate git repository root detection logic, and missing godoc comments on exported functions.

The audit identified 23 discrete improvement tasks ranging from quick wins (S effort) to more substantial refactoring (L effort). Priority should be given to security hardening (path validation) and test coverage expansion.

---

## 1. UX Consistency

### Findings

**CLI Flag Naming:** ✅ Consistent kebab-case convention across all commands (`--cwd`, `--name`, `--runtime`, `--yaks`, `--task`). The `--task` alias for `--yaks` improves discoverability.

**Help Text Quality:** ✅ Strong. All commands include Short/Short descriptions, comprehensive Long text with multi-paragraph explanations, and concrete examples. Example blocks follow consistent formatting with backticks.

**Error Messages:** ⚠️ Mixed quality.
- **Good:** Error wrapping with `%w` preserves context
- **Good:** Helpful suggestions in spawn.go: "To try native mode instead, run: yak-box spawn --runtime=native [same options]"
- **Poor:** Generic messages like "failed to resolve working directory" without actionable guidance
- **Poor:** No validation feedback before execution (e.g., warning if CWD doesn't exist until spawn fails)

**Output Formatting:** ⚠️ Inconsistent table alignment and section headers.
- check.go uses `%-20s %-15s %-10s` for tables
- Mix of "===" headers vs plain text
- No color coding (all plain text) - harder to scan visually
- "Warning:" prefix is consistent across commands ✅

**Exit Codes:** ⚠️ Uniform `os.Exit(1)` for all errors - no distinction between user error vs system error vs validation failure. This makes scripting and automation harder.

### Issues
1. No color support for improved readability (errors, warnings, success)
2. Exit codes don't distinguish error types (user error = 1, system error = 2, validation = 3)
3. Error messages lack actionable guidance in some cases
4. No progress indicators for long operations (Docker builds, container startup)
5. Table formatting inconsistencies in check.go output sections

---

## 2. Test Coverage

### Findings

**Coverage Map:**
- ✅ **cmd/**: spawn_test.go, stop_test.go, check_test.go (flag validation only)
- ✅ **internal/metadata/**: metadata_test.go (basic happy path)
- ✅ **internal/persona/**: persona_test.go (personality loading)
- ✅ **pkg/devcontainer/**: config_test.go (JSON parsing)
- ✅ **pkg/types/**: types_test.go (basic struct)
- ✅ **pkg/worktree/**: manager_test.go (worktree operations)
- ❌ **internal/config/**: No tests
- ❌ **internal/prompt/**: No tests
- ❌ **internal/runtime/**: No tests (sandboxed, native, devcontainer)
- ❌ **internal/sessions/**: No tests
- ❌ **internal/zellij/**: No tests
- ❌ **pkg/devcontainer/**: build.go, lifecycle.go, variables.go untested

**Test Quality:** ⚠️ Existing tests are minimal
- cmd/*_test.go only verify flag registration, not command logic
- metadata_test.go only tests happy path (no error cases)
- No integration tests for end-to-end workflows
- No table-driven tests for edge cases

### Issues
6. ~62% of packages have zero test coverage
7. Existing tests only cover happy paths, no error scenarios
8. No tests for critical runtime logic (Docker operations, Zellij integration)
9. No integration tests for spawn → check → stop workflow
10. Missing edge case coverage (invalid paths, missing Docker, permission errors)

---

## 3. Simplicity

### Findings

**Package Structure:** ✅ Logical separation of concerns. `internal/` for implementation details, `pkg/` for potentially reusable components.

**Code Duplication:** ⚠️ Several functions repeated across packages:
- `findWorkspaceRoot()` appears in 4+ packages (persona, metadata, sessions, config, runtime) with identical implementation
- Container name prefix inconsistency: "yak-shaver-" in sandboxed.go vs "yak-worker-" in spawn.go/stop.go
- Error handling boilerplate repeated (could use helper functions)

**Over-Engineering:** ✅ Mostly appropriate. Devcontainer package is comprehensive but necessarily so for spec compliance.

**Dead Code:** ⚠️ Minor issues:
- metadata.go package appears unused (spawn.go uses sessions.go instead for tracking)
- Some exported types in pkg/devcontainer (LockedFeature, HostRequirements) parsed but never used

**Function Complexity:** ⚠️ Several long functions:
- `SpawnSandboxedWorker()` (230 lines) - does script generation, Docker config, Zellij layout in one function
- `runSpawn()` (150+ lines) - handles validation, worktree creation, persona selection, spawning in sequence

### Issues
11. Duplicate `findWorkspaceRoot()` function across 5+ packages
12. Container name prefix mismatch: "yak-shaver-" vs "yak-worker-"
13. `SpawnSandboxedWorker()` does too much (230 lines) - should be broken into helpers
14. metadata.go package seems unused - sessions.go provides similar functionality
15. No shared error helper functions despite repeated patterns

---

## 4. Go Idioms

### Findings

**Error Handling:** ✅ Strong adherence to Go 1.13+ error wrapping
- 56 instances of `fmt.Errorf(..., %w, err)` for proper error chains
- Consistent function signatures returning `error` as last parameter
- One sentinel error in sessions.go: `var ErrSessionNotFound = fmt.Errorf("session not found")`
- No unnecessary panic usage

**Interface Usage:** ✅ Minimal and appropriate. No over-abstraction with interfaces.

**Context Usage:** ❌ No context.Context usage anywhere
- Long-running operations (Docker build, container spawn) don't accept context
- No cancellation support for operations

**Naming Conventions:** ✅ Mostly idiomatic
- Exported functions are clear (LoadConfig, GetRandomPersona)
- Package names are concise (runtime, sessions, persona)
- Minor: "yak-box" hyphenated vs "yakbox" in import paths is inconsistent

**Documentation:** ❌ Poor godoc coverage
- Most exported functions lack godoc comments
- Package-level documentation missing for internal packages
- Only devcontainer package has comprehensive comments

**Struct Field Tags:** ✅ JSON tags properly used for marshaling

### Issues
16. No context.Context support for cancellable operations (Docker builds, spawn)
17. Missing godoc comments on most exported functions and types
18. Package-level documentation missing for all internal packages
19. No use of functional options pattern for complex constructors (e.g., SpawnSandboxedWorker)
20. Import path "github.com/yakthang/yakbox" uses different casing than binary name "yak-box"

---

## 5. Security

### Findings

**Docker Socket Access:** ✅ Uses Docker CLI commands via `exec.Command()`, not direct socket access. Lower risk of Docker daemon compromise.

**File Path Handling:** ⚠️ **CRITICAL ISSUE**
- `worktree/manager.go`: `sanitizeTaskPath()` only replaces `/`, `:`, and space - doesn't prevent `..` traversal
- `spawn.go`: Accepts user-provided `--cwd` and `--yak-path` without validation
- No checks for path traversal (e.g., `--yak-path ../../../../etc/passwd`)
- `filepath.Abs()` resolves symlinks but doesn't validate against escaping project root

**Environment Variable Handling:** ⚠️ Moderate risk
- Reads host environment variables (HOME, XDG_DATA_HOME, YAK_PATH) and passes to containers
- devcontainer.json can specify arbitrary environment variables passed to containers
- No sanitization or allowlist for environment variable propagation

**Container Configuration:** ⚠️ Configurable privileges
- devcontainer.json supports `privileged: true` (unrestricted host access)
- Supports custom `capAdd` (Linux capabilities) without validation
- Supports custom `securityOpt` without validation
- Default sandboxed.go is secure: `--cap-drop ALL`, `--security-opt no-new-privileges`

**Input Validation:** ❌ Minimal validation
- CLI flags marked required, but no semantic validation (path existence, format)
- devcontainer.json parsed with `json.Unmarshal` but no schema validation
- Container names sanitized in spawn.go but not in stop.go (inconsistent)
- No validation that CWD is within workspace root

**Command Injection Risk:** ✅ Low. Uses `exec.Command()` with separate arguments, not shell execution.

### Issues
21. **CRITICAL:** No path traversal validation in worktree creation or task paths
22. **HIGH:** devcontainer.json can specify privileged mode without warning/confirmation
23. **MEDIUM:** No validation that --cwd is within workspace boundaries
24. **MEDIUM:** Environment variables passed to containers without sanitization
25. **LOW:** Container name sanitization inconsistent between spawn and stop

---

## Recommended Yaks

Below are 23 discrete improvement tasks, each independently actionable.

### Priority 1: Security (Address First)

1. **Add path traversal validation** [Security] — **S**  
   Create `pkg/pathutil/validate.go` with `ValidatePath(path, root string) error` that checks for `..`, symlink escapes, and ensures path is within root. Use in spawn.go, worktree/manager.go, and check.go.

2. **Validate devcontainer.json privileges** [Security] — **S**  
   In `pkg/devcontainer/config.go`, add `ValidateSecurityConfig()` that warns or prompts when privileged mode, dangerous capabilities, or insecure securityOpt are detected.

3. **Implement environment variable allowlist** [Security] — **M**  
   Create allowlist of safe environment variables (PATH, HOME, TERM, LANG) and filter/warn when passing arbitrary env vars from devcontainer.json to containers.

### Priority 2: Test Coverage

4. **Add runtime package tests** [Test Coverage] — **L**  
   Create `internal/runtime/sandboxed_test.go`, `native_test.go`, `devcontainer_test.go` with tests for Docker command construction, script generation, error handling, and Zellij layout generation.

5. **Add sessions package tests** [Test Coverage] — **M**  
   Create `internal/sessions/sessions_test.go` covering Load/Save, Register/Unregister, GetByContainer, and error cases (missing file, corrupt JSON).

6. **Add integration tests** [Test Coverage] — **L**  
   Create `integration_test.go` with table-driven tests for full spawn → check → stop workflows using Docker test containers or mocks.

7. **Expand cmd tests beyond flag validation** [Test Coverage] — **M**  
   Update `cmd/*_test.go` to test command logic with mocked dependencies, covering error paths and edge cases.

### Priority 3: Code Quality

8. **Extract shared workspace root function** [Simplicity, Go Idioms] — **S**  
   Create `pkg/gitutil/root.go` with `FindWorkspaceRoot() (string, error)` and replace 5+ duplicate implementations.

9. **Fix container name prefix inconsistency** [UX Consistency, Simplicity] — **S**  
   Standardize on "yak-worker-" prefix. Update `sandboxed.go` constant from "yak-shaver-" to "yak-worker-" to match spawn.go/stop.go/check.go.

10. **Refactor SpawnSandboxedWorker into helpers** [Simplicity] — **M**  
    Break 230-line function into: `generateWorkerScripts()`, `buildDockerRunCommand()`, `createZellijLayout()`, `spawnZellijTab()`. Improves testability and readability.

11. **Remove or document unused metadata package** [Simplicity] — **S**  
    Either remove `internal/metadata/` (sessions.go replaces it) or document why both exist and their distinct purposes.

12. **Add godoc comments to exported functions** [Go Idioms] — **M**  
    Add package-level and function-level godoc comments to all packages, focusing on pkg/ packages first (public API surface).

13. **Add context.Context support for cancellation** [Go Idioms] — **L**  
    Update spawn functions to accept `context.Context`, enable cancellation of Docker builds and container spawns. Propagate context through runtime package.

### Priority 4: UX Improvements

14. **Add color support for CLI output** [UX Consistency] — **S**  
    Use `github.com/fatih/color` or similar to colorize errors (red), warnings (yellow), success (green), headers (bold). Add `--no-color` flag.

15. **Implement semantic exit codes** [UX Consistency] — **S**  
    Define exit code constants: 1=user error, 2=system error, 3=validation error. Update cmd/ to use appropriate codes.

16. **Add progress indicators for long operations** [UX Consistency] — **M**  
    Show spinner or progress bar during Docker build, container startup, worktree creation using `github.com/schollz/progressbar` or similar.

17. **Improve error message actionability** [UX Consistency] — **S**  
    Add "Did you mean?" suggestions, "Run this to fix:" commands, or "Check this:" diagnostics to error messages. Use consistent format.

18. **Validate inputs before execution** [UX Consistency] — **S**  
    In runSpawn(), check if CWD exists, Docker is available, yak-path is accessible BEFORE attempting spawn. Report all validation errors together.

### Priority 5: Polish

19. **Standardize table formatting in check.go** [UX Consistency] — **S**  
    Use consistent alignment width across all tables, standardize section headers to "=== Section ===" format, ensure columns line up.

20. **Add functional options pattern for spawn** [Go Idioms] — **M**  
    Replace long parameter lists with `type SpawnOption func(*spawnConfig)` pattern for cleaner API and better defaults.

21. **Create shared error helper package** [Simplicity] — **S**  
    Create `internal/errors/` with helpers like `WrapWithSuggestion(err, msg, suggestion string) error` to reduce boilerplate.

22. **Add --version flag** [UX Consistency] — **S**  
    Add version flag to root command, embed version via build-time ldflags, display version and build info.

23. **Audit and test edge cases** [Test Coverage] — **M**  
    Create edge case test suite: missing Docker, invalid JSON in devcontainer.json, corrupted sessions.json, permission errors, disk full scenarios.

---

## Implementation Priority Matrix

| Priority | Tasks | Estimated Total Effort |
|----------|-------|------------------------|
| P1: Security | 1, 2, 3 | 2-3 days |
| P2: Testing | 4, 5, 6, 7 | 5-7 days |
| P3: Quality | 8, 9, 10, 11, 12, 13 | 4-6 days |
| P4: UX | 14, 15, 16, 17, 18 | 3-4 days |
| P5: Polish | 19, 20, 21, 22, 23 | 3-4 days |

**Recommended First Sprint:** Tasks 1, 2, 8, 9, 14, 15, 22 (Quick wins + critical security)

---

## Conclusion

yak-box demonstrates solid Go engineering practices with room for focused improvements. The most critical gaps are security validation (path traversal) and test coverage. Addressing the Priority 1 and Priority 2 tasks would significantly strengthen the codebase's robustness and maintainability.

The modular package structure makes these improvements independently implementable - each yak can be shaved in isolation without blocking others.
