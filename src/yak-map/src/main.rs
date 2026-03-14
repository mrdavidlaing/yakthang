use std::collections::BTreeMap;
use std::path::PathBuf;
use unicode_width::UnicodeWidthStr;
use zellij_tile::prelude::*;

mod ansi {
    pub const RED: &str = "\x1b[31m";
    pub const GREEN: &str = "\x1b[32m";
    pub const YELLOW: &str = "\x1b[33m";
    pub const CYAN: &str = "\x1b[36m";
    pub const WHITE: &str = "\x1b[37m";
    pub const DIM: &str = "\x1b[90m";
    pub const RESET: &str = "\x1b[0m";
    pub const BOLD: &str = "\x1b[1m";
    pub const REVERSE: &str = "\x1b[7m";
    pub const STRIKETHROUGH: &str = "\x1b[9m";
    pub const BG_SELECTED: &str = "\x1b[48;5;237m";
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum TaskState {
    Wip,
    Todo,
    Done,
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum AgentStatusKind {
    Blocked,
    Done,
    Wip,
    Unknown,
}

impl AgentStatusKind {
    fn from_status_string(s: &str) -> Self {
        if s.starts_with("blocked:") {
            AgentStatusKind::Blocked
        } else if s.starts_with("done:") {
            AgentStatusKind::Done
        } else if s.starts_with("wip:") {
            AgentStatusKind::Wip
        } else {
            AgentStatusKind::Unknown
        }
    }
}

pub struct TaskRepository {
    yaks_dir: PathBuf,
}

impl Default for TaskRepository {
    fn default() -> Self {
        Self {
            yaks_dir: PathBuf::new(),
        }
    }
}

impl TaskRepository {
    pub fn new(yaks_dir: PathBuf) -> Self {
        Self { yaks_dir }
    }

    pub fn yaks_dir(&self) -> &PathBuf {
        &self.yaks_dir
    }

    pub fn list_tasks(&self) -> Vec<(String, usize)> {
        let mut tasks = Vec::new();
        if self.yaks_dir.exists() {
            self.walk_dir(&self.yaks_dir, 0, &mut tasks);
        }
        tasks
    }

    fn walk_dir(&self, dir: &std::path::Path, depth: usize, tasks: &mut Vec<(String, usize)>) {
        if let Ok(entries) = std::fs::read_dir(dir) {
            let mut entries: Vec<_> = entries.filter_map(|e| e.ok()).collect();
            entries.sort_by_key(|a| a.file_name());

            for entry in entries {
                let path = entry.path();
                if path.is_dir() {
                    if let Ok(relative) = path.strip_prefix(&self.yaks_dir) {
                        let task_path = relative.to_string_lossy().replace('\\', "/");
                        if !task_path.starts_with('.') {
                            tasks.push((task_path.clone(), depth));
                            self.walk_dir(&path, depth + 1, tasks);
                        }
                    }
                }
            }
        }
    }

    pub fn get_field(&self, task_path: &str, field: &str) -> Option<String> {
        let field_path = self.yaks_dir.join(task_path).join(field);
        std::fs::read_to_string(&field_path)
            .ok()
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
    }

    /// Path to the context.md file for a task (may not exist yet).
    pub fn context_path(&self, task_path: &str) -> PathBuf {
        self.yaks_dir.join(task_path).join("context.md")
    }

    pub fn get_task(&self, path: &str, depth: usize) -> TaskLine {
        let state_str = self.get_field(path, ".state");
        let state = match state_str.as_deref() {
            Some("wip") => TaskState::Wip,
            Some("done") => TaskState::Done,
            _ => TaskState::Todo,
        };

        let name = self
            .get_field(path, ".name")
            .unwrap_or_else(|| path.split('/').next_back().unwrap_or(path).to_string());

        let yak_id = self
            .get_field(path, ".id")
            .unwrap_or_else(|| path.split('/').next_back().unwrap_or(path).to_string());

        TaskLine {
            path: path.to_string(),
            name,
            yak_id,
            depth,
            state,
            assigned_to: self.get_field(path, "assigned-to"),
            agent_status: self.get_field(path, "agent-status"),
            review_status: self.get_field(path, "review-status"),
            has_children: false,
            is_last_sibling: false,
            ancestor_continuations: Vec::new(),
        }
    }
}

#[derive(Default)]
struct State {
    repository: TaskRepository,
    tasks: Vec<TaskLine>,
    selected_index: usize,
    scroll_offset: usize,
    error: Option<String>,
    toast_message: Option<String>,
    toast_ticks_remaining: u8,
    pending_clipboard: Option<String>,
}

#[derive(Debug, Clone)]
pub struct TaskLine {
    path: String,
    name: String,
    yak_id: String,
    depth: usize,
    state: TaskState,
    assigned_to: Option<String>,
    agent_status: Option<String>,
    review_status: Option<String>,
    has_children: bool,
    is_last_sibling: bool,
    ancestor_continuations: Vec<bool>,
}

impl Default for TaskLine {
    fn default() -> Self {
        Self {
            path: String::new(),
            name: String::new(),
            yak_id: String::new(),
            depth: 0,
            state: TaskState::Todo,
            assigned_to: None,
            agent_status: None,
            review_status: None,
            has_children: false,
            is_last_sibling: false,
            ancestor_continuations: Vec::new(),
        }
    }
}

/// Map review-status field value to display emoji: 🔍 in-progress, ✅ pass, ❌ fail.
/// Uses prefix matching (e.g. "pass: summary", "fail: missing tests") like agent-status.
fn review_status_emoji(value: &str) -> Option<&'static str> {
    let v = value.trim().to_lowercase();
    if v.starts_with("in-progress") || v.starts_with("in_progress") {
        Some("🔍")
    } else if v.starts_with("pass") {
        Some("✅")
    } else if v.starts_with("fail") {
        Some("❌")
    } else {
        None
    }
}

/// Escape a string for use inside single-quoted shell literal (replace ' with '\'').
fn escape_single_quoted(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('\'');
    for c in s.chars() {
        if c == '\'' {
            out.push_str("'\\''");
        } else {
            out.push(c);
        }
    }
    out.push('\'');
    out
}

/// Write OSC 52 clipboard sequence to Zellij's outer terminal (the SSH PTY).
/// Finds the Zellij client process (the one with a real PTY, not /dev/null) and
/// writes to its fd/1 — the same TTY Zellij uses for copy-on-select.
/// Falls back to pbcopy on macOS if no PTY is found via /proc or lsof.
fn copy_via_zellij_tty(yx_name: &str) {
    let encoded = base64_encode(yx_name.as_bytes());
    let name_quoted = escape_single_quoted(yx_name);
    // Zellij runs as two processes: a client (with the real TTY) and a server (/dev/null).
    // pgrep finds both; we pick the one whose fd/1 is a character device (the PTY).
    // Linux uses /proc/$pid/fd/1; macOS uses lsof. pbcopy is a macOS-native fallback.
    // base64 output is alphanumeric + +/= — safe to embed in shell without quoting.
    let script = format!(
        r#"for pid in $(pgrep -x zellij 2>/dev/null); do
  tty=$(readlink -f /proc/$pid/fd/1 2>/dev/null)
  if [ -c "$tty" ] && [ "$tty" != /dev/null ]; then
    printf '\033]52;c;{enc}\007' > "$tty"
    exit 0
  fi
done
for pid in $(pgrep -x zellij 2>/dev/null); do
  tty=$(lsof -p "$pid" -a -d 1 -F n 2>/dev/null | grep '^n' | sed 's/^n//' | head -1)
  if [ -c "$tty" ] && [ "$tty" != /dev/null ]; then
    printf '\033]52;c;{enc}\007' > "$tty"
    exit 0
  fi
done
if command -v pbcopy >/dev/null 2>&1; then
  printf '%s' {name} | pbcopy
  exit 0
fi
printf '\033]52;c;{enc}\007' > /dev/tty 2>/dev/null"#,
        enc = encoded,
        name = name_quoted
    );
    run_command(&["sh", "-c", &script], BTreeMap::new());
}

/// Encode bytes as base64 (standard alphabet, with padding).
fn base64_encode(data: &[u8]) -> String {
    const CHARS: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    for chunk in data.chunks(3) {
        let b0 = chunk[0] as usize;
        let b1 = if chunk.len() > 1 {
            chunk[1] as usize
        } else {
            0
        };
        let b2 = if chunk.len() > 2 {
            chunk[2] as usize
        } else {
            0
        };
        out.push(CHARS[b0 >> 2] as char);
        out.push(CHARS[((b0 & 3) << 4) | (b1 >> 4)] as char);
        out.push(if chunk.len() > 1 {
            CHARS[((b1 & 0xf) << 2) | (b2 >> 6)] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            CHARS[b2 & 0x3f] as char
        } else {
            '='
        });
    }
    out
}

/// Compute the display column width of a string that may contain ANSI escapes.
/// Strips ANSI sequences first, then measures unicode display width.
fn line_display_width(s: &str) -> usize {
    strip_ansi(s).width()
}

/// Strip ANSI escape sequences (CSI sequences like \x1b[...m) from a string,
/// returning only the visible characters.
fn strip_ansi(s: &str) -> String {
    let mut result = String::new();
    let mut chars = s.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\x1b' && chars.peek() == Some(&'[') {
            chars.next(); // consume '['
            for inner in chars.by_ref() {
                if inner.is_ascii_alphabetic() {
                    break;
                }
            }
        } else {
            result.push(c);
        }
    }
    result
}

impl State {
    fn refresh_tasks(&mut self) {
        let task_paths = self.repository.list_tasks();
        let mut tasks: Vec<TaskLine> = task_paths
            .into_iter()
            .map(|(path, depth)| self.repository.get_task(&path, depth))
            .collect();

        if tasks.is_empty() {
            self.tasks = tasks;
            self.selected_index = 0;
            return;
        }

        let path_to_index: std::collections::HashMap<String, usize> = tasks
            .iter()
            .enumerate()
            .map(|(i, t)| (t.path.clone(), i))
            .collect();

        for i in 0..tasks.len() {
            let path = &tasks[i].path;
            let prefix = format!("{}/", path);
            tasks[i].has_children = tasks.iter().any(|t| t.path.starts_with(&prefix));
        }

        let mut by_parent: std::collections::BTreeMap<String, Vec<usize>> =
            std::collections::BTreeMap::new();
        for (i, task) in tasks.iter().enumerate() {
            let parent = match task.path.rfind('/') {
                Some(pos) => task.path[..pos].to_string(),
                None => String::new(),
            };
            by_parent.entry(parent).or_default().push(i);
        }

        for indices in by_parent.values() {
            if let Some(&last) = indices.last() {
                tasks[last].is_last_sibling = true;
            }
        }

        let paths: Vec<String> = tasks.iter().map(|t| t.path.clone()).collect();
        for (i, path) in paths.iter().enumerate() {
            let mut continuations = Vec::new();

            // For each ancestor level (parent, grandparent, etc.), check if that ancestor has siblings after it
            let mut current = path.rfind('/').map(|pos| path[..pos].to_string());

            while let Some(ancestor) = current {
                // Get the parent's parent (to find siblings of the ancestor)
                let ancestors_parent = if let Some(pos) = ancestor.rfind('/') {
                    Some(ancestor[..pos].to_string())
                } else {
                    Some(String::new()) // root level
                };

                if let Some(parent_of_ancestor) = ancestors_parent {
                    let ancestors_siblings = by_parent
                        .get(&parent_of_ancestor)
                        .map(|v| v.as_slice())
                        .unwrap_or(&[]);
                    let pos_in_ancestors_siblings = ancestors_siblings.iter().position(|&x| {
                        x == path_to_index.get(&ancestor).copied().unwrap_or(usize::MAX)
                    });

                    if let Some(pos) = pos_in_ancestors_siblings {
                        let has_more_siblings = pos + 1 < ancestors_siblings.len();
                        continuations.push(has_more_siblings);
                    }
                }

                current = ancestor.rfind('/').map(|pos| ancestor[..pos].to_string());
            }
            tasks[i].ancestor_continuations = continuations;
        }

        self.tasks = tasks;

        if self.selected_index >= self.tasks.len() && !self.tasks.is_empty() {
            self.selected_index = self.tasks.len() - 1;
        }
    }

    fn task_color(&self, task: &TaskLine) -> &'static str {
        if let Some(status) = &task.agent_status {
            match AgentStatusKind::from_status_string(status) {
                AgentStatusKind::Blocked => return ansi::RED,
                AgentStatusKind::Done => return ansi::GREEN,
                AgentStatusKind::Wip => return ansi::YELLOW,
                AgentStatusKind::Unknown => {}
            }
        }
        match task.state {
            TaskState::Wip => ansi::YELLOW,
            TaskState::Done => ansi::DIM,
            TaskState::Todo => ansi::WHITE,
        }
    }

    fn status_symbol(&self, task: &TaskLine) -> char {
        if let Some(status) = &task.agent_status {
            match AgentStatusKind::from_status_string(status) {
                AgentStatusKind::Done => return '✓',
                AgentStatusKind::Wip | AgentStatusKind::Blocked => return '●',
                AgentStatusKind::Unknown => {}
            }
        }
        match task.state {
            TaskState::Wip => '●',
            TaskState::Done => '✓',
            TaskState::Todo => '○',
        }
    }

    fn tree_prefix(&self, task: &TaskLine) -> String {
        if task.depth == 0 {
            return String::new();
        }

        let mut prefix = String::new();
        let line_color = ansi::DIM;
        let reset = ansi::RESET;

        // Show continuation columns for each ancestor level (from root-most to parent).
        // ancestor_continuations is ordered [parent, grandparent, ...], so we take
        // the first depth-1 entries (excluding the root-most) and reverse them to
        // render columns from left (root-most) to right (parent-most).
        let col_count = task.depth.saturating_sub(1);
        let cols = &task.ancestor_continuations[..col_count.min(task.ancestor_continuations.len())];
        for &has_continuation in cols.iter().rev() {
            if has_continuation {
                prefix.push_str(&format!("{}│ {}", line_color, reset));
            } else {
                prefix.push_str("  ");
            }
        }

        if task.is_last_sibling {
            prefix.push_str(&format!("{}╰─{}", line_color, reset));
        } else {
            prefix.push_str(&format!("{}├─{}", line_color, reset));
        }

        prefix
    }

    fn highlight_line(&self, line: &str, padding: &str) -> String {
        let bg = ansi::BG_SELECTED;
        let highlighted = line.replace(ansi::RESET, &format!("{}{bg}", ansi::RESET));
        format!("{bg}{}{}{}", highlighted, padding, ansi::RESET)
    }

    fn render_task(&self, task: &TaskLine) -> String {
        let prefix = self.tree_prefix(task);
        let status = self.status_symbol(task);

        let color = self.task_color(task);

        let name = if matches!(task.state, TaskState::Done) {
            format!("{}{}{}", ansi::STRIKETHROUGH, task.name, ansi::RESET)
        } else {
            task.name.clone()
        };

        let review_emoji = task
            .review_status
            .as_deref()
            .and_then(review_status_emoji)
            .unwrap_or("");

        let review_suffix = if review_emoji.is_empty() {
            String::new()
        } else {
            format!(" {}", review_emoji)
        };

        let assignment = if let Some(agent) = &task.assigned_to {
            format!(" [{}{}{}]", ansi::CYAN, agent, ansi::RESET)
        } else {
            String::new()
        };

        let status_color = if matches!(task.state, TaskState::Done) {
            ansi::DIM
        } else {
            color
        };

        format!(
            "{}{}{} {}{}{}{}",
            prefix,
            status_color,
            status,
            name,
            review_suffix,
            assignment,
            ansi::RESET
        )
    }

    /// Open the selected task in a floating pane via `yx show`.
    fn open_selected_task_context(&self) {
        let Some(task) = self.tasks.get(self.selected_index) else {
            return;
        };
        let script = format!(
            "COLUMNS=100 yx show {} | less -R; zellij action close-pane",
            task.yak_id
        );
        let command = CommandToRun {
            path: PathBuf::from("sh"),
            args: vec!["-c".to_string(), script],
            cwd: None,
        };
        let coords = FloatingPaneCoordinates::new(None, None, Some("102".to_string()), None, None);
        open_command_pane_floating(command, coords, BTreeMap::new());
    }
}

impl ZellijPlugin for State {
    fn load(&mut self, _configuration: BTreeMap<String, String>) {
        subscribe(&[EventType::Timer, EventType::Key]);
        set_timeout(2.0);
        request_permission(&[PermissionType::OpenFiles, PermissionType::RunCommands]);

        let yaks_dir = PathBuf::from("/host/.yaks");

        if !yaks_dir.exists() {
            self.error = Some(format!(
                "Yaks directory not found: {}\nRun `yx add <name>` to create a task.",
                yaks_dir.display()
            ));
            return;
        }

        self.repository = TaskRepository::new(yaks_dir);
        self.refresh_tasks();
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::Timer(_) => {
                set_timeout(2.0);
                self.refresh_tasks();
                if self.toast_ticks_remaining > 0 {
                    self.toast_ticks_remaining -= 1;
                    if self.toast_ticks_remaining == 0 {
                        self.toast_message = None;
                    }
                }
                true
            }
            Event::Key(key) => {
                let handled = match key.bare_key {
                    BareKey::Up | BareKey::Char('k') if key.has_no_modifiers() => {
                        if self.selected_index > 0 {
                            self.selected_index -= 1;
                        }
                        true
                    }
                    BareKey::Down | BareKey::Char('j') if key.has_no_modifiers() => {
                        if self.selected_index + 1 < self.tasks.len() {
                            self.selected_index += 1;
                        }
                        true
                    }
                    BareKey::Char('r') if key.has_no_modifiers() => {
                        self.refresh_tasks();
                        true
                    }
                    BareKey::Char('e') if key.has_no_modifiers() => {
                        if let Some(task) = self.tasks.get(self.selected_index) {
                            let context_path = self.repository.context_path(&task.path);
                            if let Some(parent) = context_path.parent() {
                                let _ = std::fs::create_dir_all(parent);
                            }
                            if !context_path.exists() {
                                let _ = std::fs::write(&context_path, "");
                            }
                            let host_path = context_path
                                .strip_prefix("/host")
                                .unwrap_or(&context_path)
                                .to_path_buf();
                            let file_to_open = FileToOpen::new(host_path);
                            open_file_floating(file_to_open, None, BTreeMap::new());
                        }
                        true
                    }
                    BareKey::Char('y') if key.has_no_modifiers() => {
                        if let Some(task) = self.tasks.get(self.selected_index) {
                            // Try both paths: run_command writes directly to Zellij's outer
                            // terminal (SSH PTY) via /proc; pending_clipboard emits OSC 52
                            // in render() as a fallback via the plugin pane pipeline.
                            copy_via_zellij_tty(&task.yak_id);
                            self.pending_clipboard = Some(task.yak_id.clone());
                            self.toast_message = Some(format!("Copied: {}", task.yak_id));
                            self.toast_ticks_remaining = 1;
                        }
                        true
                    }
                    BareKey::Enter if key.has_no_modifiers() => {
                        self.open_selected_task_context();
                        true
                    }
                    _ => false,
                };
                handled
            }
            _ => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        let _ = self.pending_clipboard.take(); // consumed; clipboard written via copy_via_zellij_tty

        if let Some(error) = &self.error {
            println!("{}Error: {}{}", ansi::RED, error, ansi::RESET);
            return;
        }

        if self.tasks.is_empty() {
            println!("No tasks. Run `yx add <name>` to create one.");
            println!("(Refresh interval: 2s)");
            return;
        }

        let toast_rows = if self.toast_message.is_some() { 2 } else { 0 };
        let max_rows = rows.saturating_sub(3 + toast_rows);

        // Keep scroll_offset in sync with selected_index
        if self.selected_index < self.scroll_offset {
            self.scroll_offset = self.selected_index;
        } else if max_rows > 0 && self.selected_index >= self.scroll_offset + max_rows {
            self.scroll_offset = self.selected_index - max_rows + 1;
        }

        for (i, task) in self
            .tasks
            .iter()
            .skip(self.scroll_offset)
            .take(max_rows)
            .enumerate()
        {
            let line = self.render_task(task);

            if self.scroll_offset + i == self.selected_index {
                let visible_len = line_display_width(&line);
                let padding = " ".repeat(cols.saturating_sub(visible_len));
                println!("{}", self.highlight_line(&line, &padding));
            } else {
                println!("{}", line);
            }
        }

        if let Some(msg) = &self.toast_message.clone() {
            println!();
            let toast = format!(" {} ", msg);
            println!("{}{}{}{}", ansi::REVERSE, ansi::BOLD, toast, ansi::RESET);
        }
    }
}

register_plugin!(State);

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::path::Path;
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

    #[test]
    fn get_task_uses_name_file_when_present() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-hyphenated-slug");
        set_field(&yaks, "my-hyphenated-slug", ".name", "my hyphenated slug");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("my-hyphenated-slug", 0);

        assert_eq!(task.name, "my hyphenated slug");
    }

    #[test]
    fn get_task_falls_back_to_slug_when_name_file_absent() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-hyphenated-slug");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("my-hyphenated-slug", 0);

        assert_eq!(task.name, "my-hyphenated-slug");
    }

    #[test]
    fn list_tasks_returns_empty_for_empty_directory() {
        let (_temp, yaks) = mock_yaks();
        let repo = TaskRepository::new(yaks);
        let tasks = repo.list_tasks();
        assert!(tasks.is_empty());
    }

    #[test]
    fn list_tasks_finds_root_level_task() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");

        let repo = TaskRepository::new(yaks);
        let tasks = repo.list_tasks();

        assert_eq!(tasks.len(), 1);
        assert_eq!(tasks[0], ("my-task".to_string(), 0));
    }

    #[test]
    fn list_tasks_finds_nested_task() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "parent/child/grandchild");

        let repo = TaskRepository::new(yaks);
        let tasks = repo.list_tasks();

        // Should find all three levels (parent, child, grandchild)
        assert_eq!(tasks.len(), 3);
        let paths: Vec<_> = tasks.iter().map(|(p, _)| p.as_str()).collect();
        assert!(paths.contains(&"parent"));
        assert!(paths.contains(&"parent/child"));
        assert!(paths.contains(&"parent/child/grandchild"));
    }

    #[test]
    fn list_tasks_finds_multiple_tasks_at_different_depths() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "task-a");
        create_task(&yaks, "parent/task-b");
        create_task(&yaks, "parent/child/task-c");

        let repo = TaskRepository::new(yaks);
        let tasks = repo.list_tasks();

        // Should find all 5 tasks (task-a, parent, parent/task-b, parent/child, parent/child/task-c)
        assert_eq!(tasks.len(), 5);
        let paths: Vec<_> = tasks.iter().map(|(p, _)| p.as_str()).collect();
        assert!(paths.contains(&"task-a"));
        assert!(paths.contains(&"parent"));
        assert!(paths.contains(&"parent/task-b"));
        assert!(paths.contains(&"parent/child"));
        assert!(paths.contains(&"parent/child/task-c"));
    }

    #[test]
    fn get_field_returns_none_for_missing_field() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");

        let repo = TaskRepository::new(yaks);
        assert!(repo.get_field("my-task", "state").is_none());
    }

    #[test]
    fn get_field_returns_value_for_present_field() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "state", "wip");

        let repo = TaskRepository::new(yaks);
        assert_eq!(repo.get_field("my-task", "state"), Some("wip".to_string()));
    }

    #[test]
    fn get_field_trims_whitespace() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "assigned-to", "  alice  \n");

        let repo = TaskRepository::new(yaks);
        assert_eq!(
            repo.get_field("my-task", "assigned-to"),
            Some("alice".to_string())
        );
    }

    #[test]
    fn get_task_assembles_all_fields() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", ".state", "wip");
        set_field(&yaks, "my-task", "assigned-to", "bob");
        set_field(&yaks, "my-task", "agent-status", "wip: implementing");
        set_field(&yaks, "my-task", "review-status", "pass");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("my-task", 0);

        assert_eq!(task.name, "my-task");
        assert_eq!(task.depth, 0);
        assert_eq!(task.state, TaskState::Wip);
        assert_eq!(task.assigned_to, Some("bob".to_string()));
        assert_eq!(task.agent_status, Some("wip: implementing".to_string()));
        assert_eq!(task.review_status, Some("pass".to_string()));
    }

    #[test]
    fn get_task_defaults_to_todo_when_no_state() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("my-task", 0);

        assert_eq!(task.state, TaskState::Todo);
    }

    #[test]
    fn task_color_red_for_blocked() {
        let state = State::default();
        let task = TaskLine {
            agent_status: Some("blocked: waiting".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), ansi::RED);
    }

    #[test]
    fn task_color_green_for_done() {
        let state = State::default();
        let task = TaskLine {
            agent_status: Some("done: finished".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), ansi::GREEN);
    }

    #[test]
    fn task_color_yellow_for_wip() {
        let state = State::default();
        let task = TaskLine {
            agent_status: Some("wip: working".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), ansi::YELLOW);
    }

    #[test]
    fn task_color_yellow_when_state_is_wip() {
        let state = State::default();
        let task = TaskLine {
            state: TaskState::Wip,
            agent_status: None,
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), ansi::YELLOW);
    }

    #[test]
    fn task_color_white_for_todo() {
        let state = State::default();
        let task = TaskLine {
            state: TaskState::Todo,
            agent_status: None,
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), ansi::WHITE);
    }

    #[test]
    fn task_name_extracts_last_path_component() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "parent/child/grandchild");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("parent/child/grandchild", 2);
        assert_eq!(task.name, "grandchild");
    }

    #[test]
    fn handles_special_characters_in_task_name() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "task-with-dashes_and_underscores");
        set_field(&yaks, "task-with-dashes_and_underscores", ".state", "done");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("task-with-dashes_and_underscores", 0);

        assert_eq!(task.name, "task-with-dashes_and_underscores");
        assert_eq!(task.state, TaskState::Done);
    }

    #[test]
    fn handles_empty_field_file() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "assigned-to", "");

        let repo = TaskRepository::new(yaks);
        assert_eq!(repo.get_field("my-task", "assigned-to"), None);
    }

    #[test]
    fn debug_continuation() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "task-a/child1");
        create_task(&yaks, "task-a/child2");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        for task in &state.tasks {
            eprintln!(
                "{}: depth={}, ancestors={:?}, is_last={}",
                task.path, task.depth, task.ancestor_continuations, task.is_last_sibling
            );
        }
    }

    #[test]
    fn tree_prefix_depth_2_parent_has_sibling_shows_continuation() {
        let (_temp, yaks) = mock_yaks();
        // parent "child" has sibling "child2" under "parent"
        create_task(&yaks, "parent/child/grandchild");
        create_task(&yaks, "parent/child2");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let grandchild = state.tasks.iter().find(|t| t.name == "grandchild").unwrap();
        // parent "child" has sibling "child2", so continuation line shows
        let prefix = state.tree_prefix(grandchild);
        assert_eq!(
            prefix,
            format!(
                "{}│ {}{}╰─{}",
                ansi::DIM,
                ansi::RESET,
                ansi::DIM,
                ansi::RESET
            )
        );
    }

    #[test]
    fn tree_prefix_depth_2_last_child_has_continuation() {
        let (_temp, yaks) = mock_yaks();
        // Only task-a, no sibling - children have no continuation needed
        create_task(&yaks, "task-a/child1");
        create_task(&yaks, "task-a/child2");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        // task-a has no sibling at depth 0, so no continuation
        let child2 = state.tasks.iter().find(|t| t.name == "child2").unwrap();
        let prefix = state.tree_prefix(child2);
        assert_eq!(prefix, format!("{}╰─{}", ansi::DIM, ansi::RESET));
    }

    #[test]
    fn tree_prefix_depth_2_no_continuation_when_parent_is_last() {
        let (_temp, yaks) = mock_yaks();
        // "child" is the only child of "parent", so no continuation column
        create_task(&yaks, "parent/child/grandchild");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let grandchild = state.tasks.iter().find(|t| t.name == "grandchild").unwrap();
        // parent "child" has no siblings, so empty continuation column + connector
        let prefix = state.tree_prefix(grandchild);
        assert_eq!(prefix, format!("  {}╰─{}", ansi::DIM, ansi::RESET));
    }

    #[test]
    fn tree_prefix_depth_3_shows_two_continuation_columns() {
        let (_temp, yaks) = mock_yaks();
        // a/b/c/d at depth 3; b has sibling b2, c has no sibling
        create_task(&yaks, "a/b/c/d");
        create_task(&yaks, "a/b2");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let d = state.tasks.iter().find(|t| t.name == "d").unwrap();
        // Columns: [grandparent b has siblings → │ ] [parent c has no siblings → "  "] then ╰─
        let prefix = state.tree_prefix(d);
        assert_eq!(
            prefix,
            format!(
                "{}│ {}  {}╰─{}",
                ansi::DIM,
                ansi::RESET,
                ansi::DIM,
                ansi::RESET
            )
        );
    }

    #[test]
    fn render_task_wip_shows_green_bullet() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", ".state", "wip");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);

        assert!(rendered.contains("●"), "rendered: {:?}", rendered);
    }

    #[test]
    fn render_task_done_shows_strikethrough() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", ".state", "done");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);

        assert!(rendered.contains(ansi::STRIKETHROUGH));
        assert!(rendered.contains("my-task"));
        assert!(rendered.contains(ansi::RESET));
        assert!(rendered.contains("✓"), "rendered: {:?}", rendered);
    }

    #[test]
    fn render_task_todo_shows_white() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);

        assert!(rendered.contains("○"));
        assert!(rendered.contains(ansi::WHITE));
    }

    #[test]
    fn refresh_tasks_sets_is_last_sibling() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "task-a");
        create_task(&yaks, "task-b");
        create_task(&yaks, "task-c");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task_a = state.tasks.iter().find(|t| t.name == "task-a").unwrap();
        let task_b = state.tasks.iter().find(|t| t.name == "task-b").unwrap();
        let task_c = state.tasks.iter().find(|t| t.name == "task-c").unwrap();

        assert!(!task_a.is_last_sibling);
        assert!(!task_b.is_last_sibling);
        assert!(task_c.is_last_sibling);
    }

    #[test]
    fn render_task_with_assignment_shows_agent() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "assigned-to", "bob");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        assert!(
            task.assigned_to.is_some(),
            "assigned_to: {:?}",
            task.assigned_to
        );

        let rendered = state.render_task(task);
        assert!(rendered.contains("bob"), "rendered: {:?}", rendered);
    }

    #[test]
    fn review_status_emoji_maps_correctly() {
        assert_eq!(review_status_emoji("in-progress"), Some("🔍"));
        assert_eq!(review_status_emoji("in_progress"), Some("🔍"));
        assert_eq!(review_status_emoji("pass"), Some("✅"));
        assert_eq!(review_status_emoji("fail"), Some("❌"));
        assert_eq!(review_status_emoji("  PASS  "), Some("✅"));
        assert_eq!(review_status_emoji("unknown"), None);
        // Prefix matching (same pattern as agent-status)
        assert_eq!(review_status_emoji("pass: summary"), Some("✅"));
        assert_eq!(review_status_emoji("pass: looks good"), Some("✅"));
        assert_eq!(review_status_emoji("fail: summary"), Some("❌"));
        assert_eq!(review_status_emoji("fail: missing tests"), Some("❌"));
    }

    #[test]
    fn render_task_with_review_status_shows_emoji() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "review-status", "pass");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);
        assert!(
            rendered.contains("✅"),
            "pass should render ✅: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_fail_shows_cross() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "review-status", "fail");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);
        assert!(
            rendered.contains("❌"),
            "fail should render ❌: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_in_progress_shows_magnifier() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "review-status", "in-progress");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);
        assert!(
            rendered.contains("🔍"),
            "in-progress should render 🔍: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_pass_looks_good_shows_check() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "review-status", "pass: looks good");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);
        assert!(
            rendered.contains("✅"),
            "pass: looks good should render ✅: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_fail_missing_tests_shows_cross() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "review-status", "fail: missing tests");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);
        assert!(
            rendered.contains("❌"),
            "fail: missing tests should render ❌: {:?}",
            rendered
        );
    }

    #[test]
    fn refresh_tasks_handles_empty_directory() {
        let (_temp, yaks) = mock_yaks();

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            selected_index: 5,
            ..Default::default()
        };
        state.refresh_tasks();

        assert!(state.tasks.is_empty());
        assert_eq!(state.selected_index, 0);
    }

    #[test]
    fn escape_single_quoted_empty() {
        assert_eq!(escape_single_quoted(""), "''");
    }

    #[test]
    fn escape_single_quoted_no_special() {
        assert_eq!(escape_single_quoted("foo-bar"), "'foo-bar'");
    }

    #[test]
    fn escape_single_quoted_with_single_quote() {
        assert_eq!(escape_single_quoted("it's"), "'it'\\''s'");
    }

    #[test]
    fn get_task_uses_id_file_when_present() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "parent/my-task");
        set_field(&yaks, "parent/my-task", ".id", "my-task-a1b2");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("parent/my-task", 1);

        assert_eq!(task.yak_id, "my-task-a1b2");
    }

    #[test]
    fn get_task_falls_back_to_leaf_slug_for_id_when_id_file_absent() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "parent/my-task");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("parent/my-task", 1);

        assert_eq!(task.yak_id, "my-task");
    }

    #[test]
    fn refresh_tasks_selected_index_bounded() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "task-a");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            selected_index: 10,
            ..Default::default()
        };
        state.refresh_tasks();

        assert_eq!(state.selected_index, 0);
    }

    #[test]
    fn highlight_line_uses_explicit_bg_not_reverse_video() {
        let state = State::default();
        let result = state.highlight_line("hello", "   ");
        assert!(
            result.starts_with(ansi::BG_SELECTED),
            "should start with explicit bg: {:?}",
            result
        );
        assert!(
            !result.contains(ansi::REVERSE),
            "should not use reverse video: {:?}",
            result
        );
        assert!(
            result.ends_with(ansi::RESET),
            "should end with reset: {:?}",
            result
        );
    }

    #[test]
    fn highlight_line_reestablishes_bg_after_reset() {
        let state = State::default();
        // A line that contains a reset mid-way (e.g. from colored text)
        let line = &format!("{}foo{}bar", ansi::GREEN, ansi::RESET);
        let result = state.highlight_line(line, "");
        // After each RESET the bg color should be re-established
        assert!(
            result.contains(&format!("{}{}", ansi::RESET, ansi::BG_SELECTED)),
            "bg not re-established after reset: {:?}",
            result
        );
    }

    #[test]
    fn line_display_width_counts_emoji_as_two_columns() {
        // 📋 is 2 display columns wide; chars().count() would return 1 for it
        // "📋 foo" = 📋(2) + ' '(1) + 'f'(1) + 'o'(1) + 'o'(1) = 6 display cols
        assert_eq!(line_display_width("📋 foo"), 6);
        // with ANSI: stripped "📋 worklogs" = 📋(2) + ' '(1) + "worklogs"(8) = 11
        assert_eq!(
            line_display_width(&format!("{}📋 worklogs{}", ansi::YELLOW, ansi::RESET)),
            11
        );
        // plain ASCII still works
        assert_eq!(line_display_width("hello"), 5);
    }

    #[test]
    fn highlight_line_padding_uses_same_bg() {
        let state = State::default();
        let result = state.highlight_line("hi", "     ");
        // The bg is set at start before both the text and the padding
        assert!(result.starts_with(ansi::BG_SELECTED));
        // padding is inside the bg scope (before the final reset)
        let reset_pos = result.rfind(ansi::RESET).unwrap();
        assert!(
            reset_pos == result.len() - ansi::RESET.len(),
            "final reset should be at end: {:?}",
            result
        );
    }
}
