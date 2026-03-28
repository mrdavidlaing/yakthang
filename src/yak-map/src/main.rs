use std::collections::BTreeMap;
use std::path::PathBuf;
use zellij_tile::prelude::*;

pub mod model;
mod render;
pub mod repository;
mod tree;
mod util;

use model::ansi;
use model::TaskLine;
use repository::{InMemoryTaskSource, TaskRepository, TaskSource};

pub(crate) struct State {
    pub(crate) source: Box<dyn TaskSource>,
    pub(crate) repository: TaskRepository,
    pub(crate) tasks: Vec<TaskLine>,
    pub(crate) selected_index: usize,
    scroll_offset: usize,
    error: Option<String>,
    toast_message: Option<String>,
    toast_ticks_remaining: u8,
    pending_clipboard: Option<String>,
    notes_ref_path: PathBuf,
    last_notes_hash: Option<String>,
}

impl Default for State {
    fn default() -> Self {
        let repo = TaskRepository::default();
        Self {
            source: Box::new(InMemoryTaskSource::new()),
            repository: repo,
            tasks: Vec::new(),
            selected_index: 0,
            scroll_offset: 0,
            error: None,
            toast_message: None,
            toast_ticks_remaining: 0,
            pending_clipboard: None,
            notes_ref_path: PathBuf::new(),
            last_notes_hash: None,
        }
    }
}

impl State {
    pub(crate) fn refresh_tasks(&mut self) {
        self.tasks = tree::build(self.source.as_ref());

        if self.tasks.is_empty() {
            self.selected_index = 0;
        } else if self.selected_index >= self.tasks.len() {
            self.selected_index = self.tasks.len() - 1;
        }
    }

    fn handle_show(&self) {
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
        let coords = FloatingPaneCoordinates::new(
            Some("3".to_string()),
            Some("2".to_string()),
            Some("96%".to_string()),
            Some("94%".to_string()),
            None,
            None,
        );
        open_command_pane_floating(command, coords, BTreeMap::new());
    }

    fn read_notes_hash(&self) -> Option<String> {
        std::fs::read_to_string(&self.notes_ref_path)
            .ok()
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
    }

    fn check_and_refresh_if_changed(&mut self) {
        let current_hash = self.read_notes_hash();
        if current_hash != self.last_notes_hash {
            self.last_notes_hash = current_hash;
            self.refresh_tasks();
        }
    }

    fn tick_toast(&mut self) {
        if self.toast_ticks_remaining > 0 {
            self.toast_ticks_remaining -= 1;
            if self.toast_ticks_remaining == 0 {
                self.toast_message = None;
            }
        }
    }

    fn handle_timer(&mut self) {
        set_timeout(2.0);
        self.check_and_refresh_if_changed();
        self.tick_toast();
    }

    fn handle_navigate_up(&mut self) {
        if self.selected_index > 0 {
            self.selected_index -= 1;
        }
    }

    fn handle_navigate_down(&mut self) {
        if self.selected_index + 1 < self.tasks.len() {
            self.selected_index += 1;
        }
    }

    fn handle_edit_context(&self) {
        let Some(task) = self.tasks.get(self.selected_index) else {
            return;
        };
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

    fn handle_yank(&mut self) {
        let Some(task) = self.tasks.get(self.selected_index) else {
            return;
        };
        util::copy_via_zellij_tty(&task.yak_id);
        self.pending_clipboard = Some(task.yak_id.clone());
        self.toast_message = Some(format!("Copied: {}", task.yak_id));
        self.toast_ticks_remaining = 1;
    }

    fn handle_refresh(&mut self) {
        self.refresh_tasks();
        self.last_notes_hash = self.read_notes_hash();
    }

    fn handle_key(&mut self, key: KeyWithModifier) -> bool {
        if !key.has_no_modifiers() {
            return false;
        }
        match key.bare_key {
            BareKey::Up | BareKey::Char('k') => {
                self.handle_navigate_up();
                true
            }
            BareKey::Down | BareKey::Char('j') => {
                self.handle_navigate_down();
                true
            }
            BareKey::Char('r') => {
                self.handle_refresh();
                true
            }
            BareKey::Char('e') => {
                self.handle_edit_context();
                true
            }
            BareKey::Char('y') => {
                self.handle_yank();
                true
            }
            BareKey::Enter => {
                self.handle_show();
                true
            }
            _ => false,
        }
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

        self.notes_ref_path = PathBuf::from("/host/.git/refs/notes/yaks");

        let repo = TaskRepository::new(yaks_dir);
        self.source = Box::new(TaskRepository::new(repo.yaks_dir().clone()));
        self.repository = repo;
        self.refresh_tasks();
        self.last_notes_hash = self.read_notes_hash();
    }

    fn update(&mut self, event: Event) -> bool {
        match event {
            Event::Timer(_) => {
                self.handle_timer();
                true
            }
            Event::Key(key) => self.handle_key(key),
            _ => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        let _ = self.pending_clipboard.take();

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
            let line = render::render_task(task);
            let clipped = util::clip_line(&line, cols);

            if self.scroll_offset + i == self.selected_index {
                let visible_len = util::line_display_width(&clipped);
                let padding = " ".repeat(cols.saturating_sub(visible_len));
                println!("{}", render::highlight_line(&clipped, &padding));
            } else {
                println!("{}", clipped);
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
    use tempfile::NamedTempFile;
    use std::io::Write;

    #[test]
    fn refresh_tasks_handles_empty_source() {
        let mut state = State {
            selected_index: 5,
            ..Default::default()
        };
        state.refresh_tasks();

        assert!(state.tasks.is_empty());
        assert_eq!(state.selected_index, 0);
    }

    #[test]
    fn refresh_tasks_selected_index_bounded() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("task-a", 0);

        let mut state = State {
            source: Box::new(src),
            selected_index: 10,
            ..Default::default()
        };
        state.refresh_tasks();

        assert_eq!(state.selected_index, 0);
    }

    // --- Change detection tests ---

    fn state_with_hash_file(path: PathBuf) -> State {
        let mut src = InMemoryTaskSource::new();
        src.add_task("task-a", 0);
        State {
            source: Box::new(src),
            notes_ref_path: path,
            ..Default::default()
        }
    }

    #[test]
    fn check_and_refresh_skips_rebuild_when_hash_unchanged() {
        let mut tmp = NamedTempFile::new().unwrap();
        write!(tmp, "abc123").unwrap();

        let mut state = state_with_hash_file(tmp.path().to_path_buf());
        state.last_notes_hash = Some("abc123".to_string());

        // Pre-populate tasks so we can detect if refresh_tasks ran
        state.refresh_tasks();
        let tasks_before = state.tasks.len();

        state.check_and_refresh_if_changed();

        // Hash unchanged — tasks untouched, hash stays the same
        assert_eq!(state.tasks.len(), tasks_before);
        assert_eq!(state.last_notes_hash, Some("abc123".to_string()));
    }

    #[test]
    fn check_and_refresh_triggers_rebuild_when_hash_changes() {
        let mut tmp = NamedTempFile::new().unwrap();
        write!(tmp, "def456").unwrap();

        let mut state = state_with_hash_file(tmp.path().to_path_buf());
        state.last_notes_hash = Some("abc123".to_string());

        state.check_and_refresh_if_changed();

        // Hash changed — tasks refreshed and hash updated
        assert_eq!(state.last_notes_hash, Some("def456".to_string()));
        assert_eq!(state.tasks.len(), 1); // source has one task
    }

    #[test]
    fn handle_refresh_bypasses_cache() {
        let mut tmp = NamedTempFile::new().unwrap();
        write!(tmp, "abc123").unwrap();

        let mut state = state_with_hash_file(tmp.path().to_path_buf());
        state.last_notes_hash = Some("abc123".to_string());

        // Clear tasks to detect if refresh_tasks runs
        state.tasks.clear();

        state.handle_refresh();

        // Same hash, but handle_refresh always calls refresh_tasks
        assert_eq!(state.tasks.len(), 1);
        assert_eq!(state.last_notes_hash, Some("abc123".to_string()));
    }

    #[test]
    fn read_notes_hash_returns_none_for_missing_file() {
        let state = State {
            notes_ref_path: PathBuf::from("/nonexistent/path/to/notes"),
            ..Default::default()
        };

        assert_eq!(state.read_notes_hash(), None);
    }

    #[test]
    fn read_notes_hash_returns_none_for_empty_file() {
        let tmp = NamedTempFile::new().unwrap();
        // File exists but is empty

        let state = State {
            notes_ref_path: tmp.path().to_path_buf(),
            ..Default::default()
        };

        assert_eq!(state.read_notes_hash(), None);
    }

    #[test]
    fn first_timer_tick_with_no_prior_hash_always_rebuilds() {
        let mut tmp = NamedTempFile::new().unwrap();
        write!(tmp, "abc123").unwrap();

        let mut state = state_with_hash_file(tmp.path().to_path_buf());
        // last_notes_hash is None by default

        state.check_and_refresh_if_changed();

        // None != Some("abc123") — triggers rebuild
        assert_eq!(state.last_notes_hash, Some("abc123".to_string()));
        assert_eq!(state.tasks.len(), 1);
    }
}
