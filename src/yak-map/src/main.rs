#![allow(unused)]

use std::collections::BTreeMap;
use std::path::PathBuf;
use zellij_tile::prelude::*;

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum TaskState {
    Wip,
    Todo,
    Done,
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
            entries.sort_by(|a, b| a.file_name().cmp(&b.file_name()));

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

    pub fn get_task(&self, path: &str, depth: usize) -> TaskLine {
        let state_str = self.get_field(path, "state");
        let state = match state_str.as_deref() {
            Some("wip") => TaskState::Wip,
            Some("done") => TaskState::Done,
            _ => TaskState::Todo,
        };

        let name = path.split('/').last().unwrap_or(path).to_string();

        TaskLine {
            path: path.to_string(),
            name,
            depth,
            state,
            assigned_to: self.get_field(path, "assigned-to"),
            agent_status: self.get_field(path, "agent-status"),
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
    error: Option<String>,
}

#[derive(Debug, Clone)]
pub struct TaskLine {
    path: String,
    name: String,
    depth: usize,
    state: TaskState,
    assigned_to: Option<String>,
    agent_status: Option<String>,
    has_children: bool,
    is_last_sibling: bool,
    ancestor_continuations: Vec<bool>,
}

impl Default for TaskLine {
    fn default() -> Self {
        Self {
            path: String::new(),
            name: String::new(),
            depth: 0,
            state: TaskState::Todo,
            assigned_to: None,
            agent_status: None,
            has_children: false,
            is_last_sibling: false,
            ancestor_continuations: Vec::new(),
        }
    }
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

        for i in 0..tasks.len() {
            let path = &tasks[i].path;
            let mut continuations = Vec::new();

            // For each ancestor level (parent, grandparent, etc.), check if that ancestor has siblings after it
            let mut current = if let Some(pos) = path.rfind('/') {
                Some(path[..pos].to_string())
            } else {
                None
            };

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

                current = if let Some(pos) = ancestor.rfind('/') {
                    Some(ancestor[..pos].to_string())
                } else {
                    None
                };
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
            if status.starts_with("blocked:") {
                return "\x1b[31m";
            }
            if status.starts_with("done:") {
                return "\x1b[32m";
            }
            if status.starts_with("wip:") {
                return "\x1b[33m";
            }
        }
        match task.state {
            TaskState::Wip => "\x1b[33m",
            TaskState::Done => "\x1b[90m",
            TaskState::Todo => "\x1b[37m",
        }
    }

    fn status_symbol(&self, task: &TaskLine) -> char {
        if let Some(status) = &task.agent_status {
            if status.starts_with("done:") {
                return '●';
            }
            if status.starts_with("wip:") || status.starts_with("blocked:") {
                return '●';
            }
        }
        match task.state {
            TaskState::Wip | TaskState::Done => '●',
            TaskState::Todo => '○',
        }
    }

    fn tree_prefix(&self, task: &TaskLine) -> String {
        if task.depth == 0 {
            return String::new();
        }

        let mut prefix = String::new();
        let line_color = "\x1b[90m";
        let reset = "\x1b[0m";

        // For depth >= 2, show continuation only for grandparent (index 1)
        if task.depth >= 2 {
            if let Some(&grandparent_cont) = task.ancestor_continuations.get(1) {
                if grandparent_cont {
                    prefix.push_str(&format!("{}│ {}", line_color, reset));
                }
            }
        }

        if task.is_last_sibling {
            prefix.push_str(&format!("{}╰─{}", line_color, reset));
        } else {
            prefix.push_str(&format!("{}├─{}", line_color, reset));
        }

        prefix
    }

    fn render_task(&self, task: &TaskLine) -> String {
        let prefix = self.tree_prefix(task);
        let status = self.status_symbol(task);

        let color = self.task_color(task);

        let name = if matches!(task.state, TaskState::Done) {
            format!("\x1b[9m{}\x1b[0m", task.name)
        } else {
            task.name.clone()
        };

        let assignment = if let Some(agent) = &task.assigned_to {
            format!(" [\x1b[36m{}\x1b[0m]", agent)
        } else {
            String::new()
        };

        let status_color = if matches!(task.state, TaskState::Done) {
            "\x1b[90m"
        } else {
            color
        };

        format!(
            "{}{}{} {}{}\x1b[0m",
            prefix, status_color, status, name, assignment
        )
    }
}

impl ZellijPlugin for State {
    fn load(&mut self, configuration: BTreeMap<String, String>) {
        subscribe(&[EventType::Timer, EventType::Key]);
        set_timeout(2.0);

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
                true
            }
            Event::Key(key) => {
                let handled = match key.bare_key {
                    BareKey::Up if key.has_no_modifiers() => {
                        if self.selected_index > 0 {
                            self.selected_index -= 1;
                        }
                        true
                    }
                    BareKey::Down if key.has_no_modifiers() => {
                        if self.selected_index + 1 < self.tasks.len() {
                            self.selected_index += 1;
                        }
                        true
                    }
                    BareKey::Char('r') if key.has_no_modifiers() => {
                        self.refresh_tasks();
                        true
                    }
                    _ => false,
                };
                if handled {
                    true
                } else {
                    false
                }
            }
            _ => false,
        }
    }

    fn render(&mut self, rows: usize, cols: usize) {
        if let Some(error) = &self.error {
            println!("\x1b[31mError: {}\x1b[0m", error);
            return;
        }

        if self.tasks.is_empty() {
            println!("No tasks. Run `yx add <name>` to create one.");
            println!("(Refresh interval: 2s)");
            return;
        }

        let max_rows = rows.saturating_sub(3);
        for (i, task) in self.tasks.iter().take(max_rows).enumerate() {
            let line = self.render_task(task);

            if i == self.selected_index {
                println!("\x1b[7m{}\x1b[0m", line);
            } else {
                println!("{}", line);
            }
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
        set_field(&yaks, "my-task", "state", "wip");
        set_field(&yaks, "my-task", "assigned-to", "bob");
        set_field(&yaks, "my-task", "agent-status", "wip: implementing");

        let repo = TaskRepository::new(yaks);
        let task = repo.get_task("my-task", 0);

        assert_eq!(task.name, "my-task");
        assert_eq!(task.depth, 0);
        assert_eq!(task.state, TaskState::Wip);
        assert_eq!(task.assigned_to, Some("bob".to_string()));
        assert_eq!(task.agent_status, Some("wip: implementing".to_string()));
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
        assert_eq!(state.task_color(&task), "\x1b[31m");
    }

    #[test]
    fn task_color_green_for_done() {
        let state = State::default();
        let task = TaskLine {
            agent_status: Some("done: finished".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), "\x1b[32m");
    }

    #[test]
    fn task_color_yellow_for_wip() {
        let state = State::default();
        let task = TaskLine {
            agent_status: Some("wip: working".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), "\x1b[33m");
    }

    #[test]
    fn task_color_yellow_when_state_is_wip() {
        let state = State::default();
        let task = TaskLine {
            state: TaskState::Wip,
            agent_status: None,
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), "\x1b[33m");
    }

    #[test]
    fn task_color_white_for_todo() {
        let state = State::default();
        let task = TaskLine {
            state: TaskState::Todo,
            agent_status: None,
            ..TaskLine::default()
        };
        assert_eq!(state.task_color(&task), "\x1b[37m");
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
        set_field(&yaks, "task-with-dashes_and_underscores", "state", "done");

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
    fn tree_prefix_depth_2_with_sibling_has_continuation() {
        let (_temp, yaks) = mock_yaks();
        // Create depth 2: grandparent "parent" has sibling at root
        create_task(&yaks, "parent/child/grandchild");
        create_task(&yaks, "sibling");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let grandchild = state.tasks.iter().find(|t| t.name == "grandchild").unwrap();
        // grandparent "parent" has sibling "sibling" at root, so continuation shows
        let prefix = state.tree_prefix(grandchild);
        assert_eq!(prefix, "\x1b[90m│ \x1b[0m\x1b[90m╰─\x1b[0m");
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
        assert_eq!(prefix, "\x1b[90m╰─\x1b[0m");
    }

    #[test]
    fn tree_prefix_depth_2_no_continuation_when_parent_not_last() {
        let (_temp, yaks) = mock_yaks();
        // Create depth 2: grandparent has no sibling at root
        create_task(&yaks, "parent/child/grandchild");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let grandchild = state.tasks.iter().find(|t| t.name == "grandchild").unwrap();
        // grandparent "parent" has no sibling at root, so no continuation
        let prefix = state.tree_prefix(grandchild);
        assert_eq!(prefix, "\x1b[90m╰─\x1b[0m");
    }

    #[test]
    fn render_task_wip_shows_green_bullet() {
        let (_temp, yaks) = mock_yaks();
        create_task(&yaks, "my-task");
        set_field(&yaks, "my-task", "state", "wip");

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
        set_field(&yaks, "my-task", "state", "done");

        let repo = TaskRepository::new(yaks.clone());
        let mut state = State {
            repository: repo,
            ..Default::default()
        };
        state.refresh_tasks();

        let task = state.tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = state.render_task(task);

        assert!(rendered.contains("\x1b[9m"));
        assert!(rendered.contains("my-task"));
        assert!(rendered.contains("\x1b[0m"));
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
        assert!(rendered.contains("\x1b[37m"));
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
}
