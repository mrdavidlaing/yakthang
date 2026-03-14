# yak-map Architecture Skill

**Trigger:** Use when working on yak-map code — adding features, fixing bugs, refactoring, or reviewing changes in `src/yak-map/`.

This skill codifies the architectural conventions of the yak-map Zellij plugin. Every convention is grounded in the actual codebase and includes rationale so you don't accidentally undo deliberate design decisions.

---

## Module Structure

### Convention

Six files, each with a clear responsibility:

| File | Responsibility |
|------|---------------|
| `main.rs` | Plugin entry point, Zellij lifecycle (`load`, `update`, `render`), `State` struct, key dispatch |
| `model.rs` | Domain types: `TaskLine`, `TaskState`, `AgentStatusKind`, `ReviewStatusKind`, `ansi` constants |
| `repository.rs` | Data access: `TaskSource` trait, `TaskRepository` (filesystem), `InMemoryTaskSource` (tests), `get_task()` |
| `tree.rs` | Pure tree builder: `build()` takes a `&dyn TaskSource`, returns `Vec<TaskLine>` with tree metadata |
| `render.rs` | Pure rendering: `render_task()`, `tree_prefix()`, `task_color()`, `status_symbol()`, `highlight_line()` |
| `util.rs` | Standalone utilities: ANSI stripping, base64 encoding, shell escaping, clipboard via OSC 52 |

### Dependency DAG

```
main.rs
  -> model.rs
  -> repository.rs -> model.rs
  -> tree.rs       -> model.rs, repository.rs
  -> render.rs     -> model.rs
  -> util.rs       (standalone)
```

No circular dependencies. `model.rs` is the root — everything depends on it, it depends on nothing internal.

### Rationale

Clear responsibility boundaries keep each file small enough to reason about independently. The DAG means you can change `render.rs` without touching `repository.rs`, and vice versa.

### When to deviate

If a new feature genuinely doesn't fit any existing module (e.g., a network client), create a new file. But exhaust existing modules first — most features land in one of these six.

---

## Parse Once, Carry in Types

### Convention

Three enums handle status parsing at the boundary:

- `TaskState` (`Wip`, `Todo`, `Done`) — implements `FromStr` for `"wip".parse()` (see `model.rs:22-36`)
- `AgentStatusKind` (`Blocked`, `Done`, `Wip`, `Unknown`) — uses `from_status_string()` with prefix matching (see `model.rs:46-58`)
- `ReviewStatusKind` (`Pass`, `Fail`, `InProgress`, `Unknown`) — uses `from_status_string()` with prefix matching (see `model.rs:68-81`)

String parsing happens once in `repository.rs:get_task()` and the enum constructors. After that, all logic uses exhaustive `match` on the enum variants.

### Rationale

The compiler catches missing cases. No scattered `starts_with("blocked:")` calls spread across render, color, and symbol functions. When a new status variant is added, every `match` that needs updating will fail to compile.

### When to deviate

If a status type is only used in one place and would never need exhaustive matching, a simple string comparison is fine. But if two or more functions branch on it, make an enum.

---

## Named Constants Over Magic Values

### Convention

All ANSI escape codes live in `model::ansi` (`model.rs:1-13`):

```rust
pub mod ansi {
    pub const RED: &str = "\x1b[31m";
    pub const GREEN: &str = "\x1b[32m";
    // ...
}
```

Usage throughout the codebase is `ansi::RED`, `ansi::RESET`, etc. — never raw `\x1b[` sequences.

### Rationale

Readable at the call site, consistent across all render paths, and one place to change if the escape format ever needs updating.

### When to deviate

Never for ANSI codes. For other magic values (e.g., timer intervals), inline constants are fine if used exactly once.

---

## Ports and Adapters (TaskSource trait)

### Convention

The `TaskSource` trait (`repository.rs:5-8`) defines the port:

```rust
pub trait TaskSource {
    fn list_tasks(&self) -> Vec<(String, usize)>;
    fn get_field(&self, task_path: &str, field: &str) -> Option<String>;
}
```

Two implementations:
- `TaskRepository` — reads from the `.yaks/` filesystem directory (`repository.rs:40-102`)
- `InMemoryTaskSource` — in-memory HashMap for tests (`repository.rs:104-146`)

**Parameter conventions:** Take `&str` for inputs (borrowed, cheap), return `String` for outputs (owned, caller keeps it).

### Rationale

Tests run without touching the filesystem. `tree::build()` and all rendering code work identically against either implementation, so tests are fast and deterministic.

### When to deviate

Filesystem-specific methods go on `TaskRepository` directly, not the trait. Example: `context_path()` (`repository.rs:61-63`) returns a `PathBuf` for a specific task's context file — this is a filesystem concern that `InMemoryTaskSource` doesn't need.

---

## Contract Test Macro

### Convention

The `task_source_tests!` macro (`repository.rs:153-179`) defines behavioral tests that both implementations must pass:

```rust
macro_rules! task_source_tests {
    ($create_source:expr) => {
        #[test]
        fn list_tasks_returns_empty() { ... }
        #[test]
        fn get_field_returns_none_for_missing_field() { ... }
        #[test]
        fn get_field_returns_value_for_present_field() { ... }
        #[test]
        fn get_field_returns_none_for_empty_value() { ... }
    };
}
```

Each impl module invokes the macro with its own factory function. The `in_memory_contract` and `filesystem_contract` modules (`repository.rs:181-243`) instantiate the same tests against different backends.

### Rationale

Guarantees behavioral parity between the real and test implementations. If a new trait method is added, the contract tests enforce that both impls handle edge cases identically.

### When to deviate

If you add a filesystem-only method (not on the trait), test it only in the filesystem module.

---

## Pure Functions Over Methods

### Convention

Core logic functions take data in, return data out, and don't mutate external state:

- `tree::build(&dyn TaskSource) -> Vec<TaskLine>` — builds the annotated task tree (`tree.rs:10-86`)
- `render::render_task(&TaskLine) -> String` — renders a single task line (`render.rs:84-130`)
- `render::task_color(&TaskLine) -> &'static str` — determines color (`render.rs:15-29`)
- `render::status_symbol(&TaskLine) -> char` — determines status character (`render.rs:31-44`)
- `render::tree_prefix(&TaskLine) -> String` — draws tree connectors (`render.rs:46-76`)

### Rationale

Pure functions are independently testable — feed in a `TaskLine`, assert on the output. No mocks, no setup, no hidden dependencies. Composable: `render_task` calls `tree_prefix`, `status_symbol`, and `task_color` without any shared mutable state.

### When to deviate

Side-effecting operations (clipboard copy via `util::copy_via_zellij_tty`, file I/O in `handle_edit_context`) necessarily mutate external state. Keep these in `main.rs` handler methods or `util.rs`, and keep them thin — delegate to pure functions for any logic.

---

## Dispatcher Pattern

### Convention

The `update()` method (`main.rs:184-193`) is a thin dispatcher:

```rust
fn update(&mut self, event: Event) -> bool {
    match event {
        Event::Timer(_) => { self.handle_timer(); true }
        Event::Key(key) => self.handle_key(key),
        _ => false,
    }
}
```

`handle_key()` (`main.rs:128-159`) is similarly a match that routes to `handle_navigate_up`, `handle_show`, `handle_yank`, etc. No inline logic in match arms.

### Rationale

Keeps complexity scores low — each handler is independently readable and testable. The dispatcher reads like a table of contents for the plugin's behavior.

### When to deviate

If a handler is truly a single expression (e.g., `self.selected_index += 1`), it can stay inline. But if it grows past 3-4 lines, extract it.

---

## Quality Gate

### Convention

Run `bin/dev check` before marking work done. It runs in order:

1. **WASM build** — `cargo build --target wasm32-wasip1 --release` (catches WASM-incompatible dependencies)
2. **Lint** — `cargo clippy -- -D warnings` + `cargo fmt --check`
3. **Tests** — `cargo test` (native target)
4. **Complexity** — cognitive <= 29, cyclomatic <= 26 (per function, via `rust-code-analysis-cli`)

Subcommands: `bin/dev lint`, `bin/dev test`, `bin/dev cx`, `bin/dev check` (all).

### Rationale

The WASM build catches issues that `cargo test` (native) won't — some crates compile on the host but fail under `wasm32-wasip1`. Complexity thresholds prevent any single function from growing unwieldy.

### When to deviate

If `rust-code-analysis-cli` is not installed, the complexity check is skipped with a warning. Install via `cargo install rust-code-analysis-cli` or use the nix shell.

---

## Testing Patterns

### Convention

- Use `InMemoryTaskSource` for unit tests — fast, no filesystem, deterministic
- Keep at least one filesystem smoke test per module that touches data (see `filesystem_contract` in `repository.rs:201-266`)
- Contract test macro for `TaskSource` implementations
- Tests live in the module they test: `#[cfg(test)] mod tests { ... }` at the bottom of each file
- Build test fixtures with the builder pattern: `InMemoryTaskSource::new()` then `.add_task()` / `.set_field()`

### Rationale

In-memory tests run in milliseconds. Filesystem tests use `tempfile::TempDir` for isolation. Co-located tests stay in sync with the code they test.

### When to deviate

Integration tests that span multiple modules can go in a `tests/` directory, but prefer composing existing pure functions in unit tests first.

---

## WASM Build Constraints

### Convention

The plugin compiles to `wasm32-wasip1`. Not all crates work in WASM — check compatibility before adding dependencies. `tempfile` and other test-only crates go in `[dev-dependencies]` (not included in the WASM build).

### Rationale

The Zellij plugin runtime is WASI-based. A dependency that uses threads, networking, or other unsupported syscalls will fail at build time or runtime.

### When to deviate

If a crate is needed at runtime and doesn't compile to WASM, look for a WASM-compatible alternative or implement the functionality inline (as done with `base64_encode` in `util.rs` instead of pulling in a base64 crate).

---

## Anti-Patterns

| Don't | Do instead |
|-------|-----------|
| `starts_with("blocked:")` scattered in logic | Use `AgentStatusKind::from_status_string()` and match the enum |
| Raw `\x1b[31m` in render code | Use `ansi::RED` from `model.rs` |
| Inline logic in `update()` match arms | Extract to `handle_*` method |
| `TaskRepository` in unit tests | Use `InMemoryTaskSource` — reserve filesystem tests for the contract suite |
| New functionality dumped in `main.rs` | Find the right module: types in `model.rs`, data access in `repository.rs`, tree logic in `tree.rs`, display in `render.rs`, utilities in `util.rs` |
| Adding a runtime crate without WASM check | Run `bin/dev check` — the WASM build step will catch incompatible deps |

---

## Where to Put New Code

Decision tree for new functionality:

1. **New domain type or enum?** -> `model.rs`
2. **Reading data from `.yaks/`?** -> `repository.rs` (add to `TaskSource` trait if both impls need it)
3. **Building or transforming the task tree?** -> `tree.rs`
4. **Formatting output for display?** -> `render.rs`
5. **Standalone utility (no domain knowledge)?** -> `util.rs`
6. **Zellij lifecycle, event handling, state management?** -> `main.rs`
7. **None of the above?** -> New module, but justify it first
