# yak-map Plugin Design

## Overview

yak-map is a Zellij plugin that reads the `.yaks/` directory directly to display task hierarchy with rich visualization, keyboard navigation, and interactive features.

## Goals

1. **Replace the watch loop** with a native Zellij plugin
2. **Show task hierarchy** with proper tree visualization
3. **Display metadata annotations** (assigned-to, agent-status)
4. **Enable keyboard navigation** (select tasks, scroll)
5. **Provide color-coded status** (wip, done, blocked)

## Architecture

### Key Decision: Direct Filesystem Access

The plugin reads `.yaks/` directly rather than calling `yx` as a subprocess.

**Why:**
- WASM plugins have limited subprocess support
- No runtime dependency on yx
- Simpler and faster

**Risk mitigation:** An anti-corruption layer abstracts the storage format.

### Data Flow

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│   .yaks/    │────▶│  TaskRepository  │────▶│   State     │
│  (filesystem)     │ (anti-corruption) │     │  (Vec<Task>)│
└─────────────┘     └──────────────────┘     └─────────────┘
         │                                       │
         │  TaskRepository provides:             ▼
         │  - list_tasks() → Vec<TaskPath> ┌─────────────┐
         │  - get_field(path, field)       │   Render    │
         │  - Handles format changes       │ (ANSI text) │
         ▼                                  └─────────────┘
    .yaks/
    ├── task-name/
    │   ├── state          # "wip" | "todo" | "done"
    │   ├── assigned-to    # worker name
    │   └── agent-status   # "wip: doing X"
    └── parent/
        └── child/
            └── state
```

### Components

#### 1. Task Data Model

```rust
struct TaskLine {
    path: String,              // Full path: "yurt-poc/worker-assignment"
    name: String,              // Just the name: "worker-assignment"
    depth: usize,              // 0 = root, 1 = child, etc.
    state: TaskState,          // Wip, Todo, Done
    status: char,              // '●' (active), '○' (todo)
    assigned_to: Option<String>,
    agent_status: Option<String>,  // Not used for colors, stored for reference
    is_last_sibling: bool,
    ancestor_continuations: Vec<bool>,  // For tree continuation lines
}
```

#### 2. TaskRepository (Anti-Corruption Layer)

- Read `.yaks/` directory structure directly
- Provides `list_tasks()`, `get_field()`, `get_task()` methods
- If yx changes storage format, update TaskRepository only — rest of plugin stays unchanged

```rust
struct TaskRepository {
    yaks_dir: PathBuf,
}

impl TaskRepository {
    fn list_tasks(&self) -> Vec<(String, usize)>;
    fn get_field(&self, path: &str, field: &str) -> Option<String>;
    fn get_task(&self, path: &str, depth: usize) -> Task;
}
```

#### 3. Renderer Module

- Convert task list to ANSI-rendered text
- Apply colors based on task state (from `state` file):
  - `wip` → Green
  - `done` → Gray (with strikethrough)
  - `todo` → White
- Assigned agent shown in cyan: `[agent-name]`
- Lines wrap (no truncation)
- Selected line highlighted (reverse video)

### Event Handling

The plugin subscribes to:

| Event | Action |
|-------|--------|
| `Timer` (2s interval) | Refresh task list |
| `Key::Up` / `Key::Down` | Navigate tasks |
| `Key::Enter` | Open `context.md` in floating editor pane |
| `Key::r` | Manual refresh |
| `Resize` | Recalculate truncation |

### Build Requirements

```bash
rustup target add wasm32-wasip1
cargo build --release --target wasm32-wasip1
```

Output: `target/wasm32-wasip1/release/yak-map.wasm`

## Integration

### Option A: Direct Plugin Load

In `orchestrator.kdl`:

```
pane size="33%" name="yak-map" {
    plugin "file:target/wasm32-wasi/debug/yak-map.wasm"
}
```

### Option B: Via Plugin Manager

If the plugin is installed to `~/.local/share/zellij/plugins/`:

```
pane size="33%" name="yak-map" {
    plugin "yak-map"
}
```

## Comparison: Subprocess vs Direct Filesystem

| Feature | yx subprocess | Direct filesystem |
|---------|---------------|-------------------|
| Tree visualization | Via yx command | Direct directory read |
| Assignment display | Via format flag | Read from files |
| Color-coded status | Via ANSI in output | Rich state colors |
| Keyboard navigation | None | Up/Down/Enter |
| Error handling | Subprocess failures | Graceful degradation |
| Resize handling | Terminal default | Smart truncation |
| Dependencies | Requires yx binary | No external deps |

## Implementation Phases

### Phase 1: Core Display (MVP)

- [x] Read `.yaks/` directory via TaskRepository
- [x] Parse task structure (state, assigned-to, agent-status)
- [x] Render task list with colors
- [x] Timer-based refresh (2s)
- [x] Integrate into orchestrator.kdl
- [x] Tree rendering with continuation lines
- [x] Lines wrap (no truncation)

### Phase 2: Interactive Features

- [x] Keyboard navigation (up/down)
- [x] Selected task highlighting
- [ ] Manual refresh key (r)
- [ ] Scroll beyond visible area

### Phase 3: Enhancements

- [ ] Floating editor pane (on Enter)
  - Calls `open_file_floating(".yaks/{task-path}/context.md")`
  - Uses user's `$EDITOR` environment variable
  - Tree remains visible underneath
  - Requires `OpenFiles` permission
- [ ] Quick-jump to task by name
- [ ] Filter by status/assignment
- [ ] Persist last selection

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| yx storage format changes | TaskRepository anti-corruption layer |
| `$EDITOR` not set | Fallback to `cat` in command pane |
| WASM binary size | Optimize with LTO, single function |
| Plugin loading failures | Fallback to script if plugin fails |

## Testing Strategy

### Unit Tests (`cargo test`)

All unit tests use temp directories — no WASM runtime needed.

#### Test Module Structure

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    fn mock_yaks() -> (TempDir, PathBuf) {
        let temp = TempDir::new().unwrap();
        let yaks = temp.path().join(".yaks");
        fs::create_dir_all(&yaks).unwrap();
        (temp, yaks)
    }

    fn create_task(yaks: &Path, path: &str) {
        fs::create_dir_all(yaks.join(path)).unwrap();
    }

    fn set_field(yaks: &Path, task_path: &str, field: &str, value: &str) {
        fs::write(yaks.join(task_path).join(field), value).unwrap();
    }
}
```

#### Test Cases: TaskRepository

- `list_tasks_returns_empty_for_empty_directory`
- `list_tasks_finds_root_level_task`
- `list_tasks_finds_nested_task`
- `list_tasks_finds_multiple_tasks_at_different_depths`
- `get_field_returns_none_for_missing_field`
- `get_field_returns_value_for_present_field`
- `get_field_trims_whitespace`
- `get_task_assembles_all_fields`
- `get_task_defaults_to_todo_when_no_state`

#### Test Cases: State / Rendering

- `task_color_green_for_wip` / `task_color_green_for_done`
- `task_color_gray_for_done`
- `task_color_white_for_todo`
- `task_name_extracts_last_path_component`
- `tree_prefix_*` - Tree rendering with continuations
- `render_task_*` - Full line rendering with assignments

#### Test Cases: Edge Cases

- `handles_special_characters_in_task_name`
- `handles_empty_field_file`

### Manual Testing

Build and load in Zellij:

```bash
cd src/yak-map
cargo build --release --target wasm32-wasip1
cp target/wasm32-wasip1/release/yak-map.wasm ../../bin/yak-map.wasm
```

#### Manual Test Checklist

| Test | Steps | Expected |
|------|-------|----------|
| Plugin loads | Load plugin | Task tree appears |
| Refresh works | Modify `.yaks/`, wait 2s | Tree updates |
| Navigation | Press `↑` / `↓` | Selection moves |
| Details (Phase 2) | Select task, press `Enter` | Floating editor opens with `context.md` |
| Colors | Create tasks with different states | Correct colors per state |

## References

- Zellij Plugin API: https://zellij.dev/documentation/plugins.html
- zellij-tile crate: https://docs.rs/zellij-tile/latest/zellij_tile/
- Example plugin: https://github.com/zellij-org/rust-plugin-example
- Current PoC: `docs/research/FINDINGS-yak-map-wrapper.md`

## Open Questions

1. How to detect `.yaks/` changes? (Timer polling vs FileSystem events)
2. Should TaskRepository cache reads?
3. How to handle very large task trees (100+ tasks)?
4. Should we persist selection across refreshes?
5. Integration with Zellij's built-in search?