use std::path::PathBuf;

use crate::model::{TaskLine, TaskState, WipState};

pub trait TaskSource {
    fn list_tasks(&self) -> Vec<(String, usize)>;
    fn get_field(&self, task_path: &str, field: &str) -> Option<String>;
}

pub fn get_task(source: &dyn TaskSource, path: &str, depth: usize) -> TaskLine {
    let state_str = source.get_field(path, ".state");
    let state = state_str
        .as_deref()
        .and_then(|s| s.parse().ok())
        .unwrap_or(TaskState::Todo);

    let name = source
        .get_field(path, ".name")
        .unwrap_or_else(|| path.split('/').next_back().unwrap_or(path).to_string());

    let yak_id = source
        .get_field(path, ".id")
        .unwrap_or_else(|| path.split('/').next_back().unwrap_or(path).to_string());

    TaskLine {
        path: path.to_string(),
        name,
        yak_id,
        depth,
        state,
        assigned_to: source.get_field(path, "assigned-to"),
        wip_state: source
            .get_field(path, "wip-state")
            .as_deref()
            .and_then(WipState::from_field),
        agent_status: source.get_field(path, "agent-status"),
        review_status: source.get_field(path, "review-status"),
        has_children: false,
        is_last_sibling: false,
        ancestor_continuations: Vec::new(),
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

    pub fn context_path(&self, task_path: &str) -> PathBuf {
        self.yaks_dir.join(task_path).join("context.md")
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
}

impl TaskSource for TaskRepository {
    fn list_tasks(&self) -> Vec<(String, usize)> {
        let mut tasks = Vec::new();
        if self.yaks_dir.exists() {
            self.walk_dir(&self.yaks_dir, 0, &mut tasks);
        }
        tasks
    }

    fn get_field(&self, task_path: &str, field: &str) -> Option<String> {
        let field_path = self.yaks_dir.join(task_path).join(field);
        std::fs::read_to_string(&field_path)
            .ok()
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
    }
}

pub struct InMemoryTaskSource {
    tasks: Vec<(String, usize)>,
    fields: std::collections::HashMap<(String, String), String>,
}

impl Default for InMemoryTaskSource {
    fn default() -> Self {
        Self::new()
    }
}

impl InMemoryTaskSource {
    pub fn new() -> Self {
        Self {
            tasks: Vec::new(),
            fields: std::collections::HashMap::new(),
        }
    }

    pub fn add_task(&mut self, path: &str, depth: usize) {
        self.tasks.push((path.to_string(), depth));
    }

    pub fn set_field(&mut self, task_path: &str, field: &str, value: &str) {
        self.fields.insert(
            (task_path.to_string(), field.to_string()),
            value.to_string(),
        );
    }
}

impl TaskSource for InMemoryTaskSource {
    fn list_tasks(&self) -> Vec<(String, usize)> {
        self.tasks.clone()
    }

    fn get_field(&self, task_path: &str, field: &str) -> Option<String> {
        self.fields
            .get(&(task_path.to_string(), field.to_string()))
            .cloned()
            .filter(|s| !s.is_empty())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::TaskState;

    macro_rules! task_source_tests {
        ($create_source:expr) => {
            #[test]
            fn list_tasks_returns_empty() {
                let src = $create_source(&[], &[]);
                assert!(src.list_tasks().is_empty());
            }

            #[test]
            fn get_field_returns_none_for_missing_field() {
                let src = $create_source(&[("my-task", 0)], &[]);
                assert!(src.get_field("my-task", "state").is_none());
            }

            #[test]
            fn get_field_returns_value_for_present_field() {
                let src = $create_source(&[("my-task", 0)], &[("my-task", "state", "wip")]);
                assert_eq!(src.get_field("my-task", "state"), Some("wip".to_string()));
            }

            #[test]
            fn get_field_returns_none_for_empty_value() {
                let src = $create_source(&[("my-task", 0)], &[("my-task", "assigned-to", "")]);
                assert_eq!(src.get_field("my-task", "assigned-to"), None);
            }
        };
    }

    mod in_memory_contract {
        use super::*;

        fn create_source(
            tasks: &[(&str, usize)],
            fields: &[(&str, &str, &str)],
        ) -> InMemoryTaskSource {
            let mut src = InMemoryTaskSource::new();
            for &(path, depth) in tasks {
                src.add_task(path, depth);
            }
            for &(task, field, value) in fields {
                src.set_field(task, field, value);
            }
            src
        }

        task_source_tests!(create_source);
    }

    mod filesystem_contract {
        use super::*;
        use std::fs;
        use std::path::Path;
        use tempfile::TempDir;

        struct FsSource {
            _temp: TempDir,
            repo: TaskRepository,
        }

        impl std::ops::Deref for FsSource {
            type Target = TaskRepository;
            fn deref(&self) -> &Self::Target {
                &self.repo
            }
        }

        fn create_fs_task(yaks: &Path, path: &str) {
            fs::create_dir_all(yaks.join(path)).unwrap();
        }

        fn set_fs_field(yaks: &Path, task_path: &str, field: &str, value: &str) {
            fs::write(yaks.join(task_path).join(field), value).unwrap();
        }

        fn create_source(tasks: &[(&str, usize)], fields: &[(&str, &str, &str)]) -> FsSource {
            let temp = TempDir::new().unwrap();
            let yaks = temp.path().join(".yaks");
            fs::create_dir_all(&yaks).unwrap();
            for &(path, _depth) in tasks {
                create_fs_task(&yaks, path);
            }
            for &(task, field, value) in fields {
                set_fs_field(&yaks, task, field, value);
            }
            FsSource {
                _temp: temp,
                repo: TaskRepository::new(yaks),
            }
        }

        task_source_tests!(create_source);

        #[test]
        fn list_tasks_finds_nested_tasks_on_filesystem() {
            let src = create_source(&[("parent/child/grandchild", 2)], &[]);
            let tasks = src.repo.list_tasks();
            assert_eq!(tasks.len(), 3);
            let paths: Vec<_> = tasks.iter().map(|(p, _)| p.as_str()).collect();
            assert!(paths.contains(&"parent"));
            assert!(paths.contains(&"parent/child"));
            assert!(paths.contains(&"parent/child/grandchild"));
        }

        #[test]
        fn get_field_trims_whitespace_on_filesystem() {
            let src = create_source(
                &[("my-task", 0)],
                &[("my-task", "assigned-to", "  alice  \n")],
            );
            assert_eq!(
                src.repo.get_field("my-task", "assigned-to"),
                Some("alice".to_string())
            );
        }
    }

    #[test]
    fn get_task_uses_name_field_when_present() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-hyphenated-slug", 0);
        src.set_field("my-hyphenated-slug", ".name", "my hyphenated slug");

        let task = get_task(&src, "my-hyphenated-slug", 0);
        assert_eq!(task.name, "my hyphenated slug");
    }

    #[test]
    fn get_task_falls_back_to_slug_when_name_absent() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-hyphenated-slug", 0);

        let task = get_task(&src, "my-hyphenated-slug", 0);
        assert_eq!(task.name, "my-hyphenated-slug");
    }

    #[test]
    fn get_task_assembles_all_fields() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", ".state", "wip");
        src.set_field("my-task", "assigned-to", "bob");
        src.set_field("my-task", "agent-status", "wip: implementing");
        src.set_field("my-task", "review-status", "pass");

        let task = get_task(&src, "my-task", 0);

        assert_eq!(task.name, "my-task");
        assert_eq!(task.depth, 0);
        assert_eq!(task.state, TaskState::Wip);
        assert_eq!(task.assigned_to, Some("bob".to_string()));
        assert_eq!(task.agent_status, Some("wip: implementing".to_string()));
        assert_eq!(task.review_status, Some("pass".to_string()));
    }

    #[test]
    fn get_task_defaults_to_todo_when_no_state() {
        let src = InMemoryTaskSource::new();
        let task = get_task(&src, "my-task", 0);
        assert_eq!(task.state, TaskState::Todo);
    }

    #[test]
    fn get_task_extracts_last_path_component_for_name() {
        let src = InMemoryTaskSource::new();
        let task = get_task(&src, "parent/child/grandchild", 2);
        assert_eq!(task.name, "grandchild");
    }

    #[test]
    fn get_task_handles_special_characters() {
        let mut src = InMemoryTaskSource::new();
        src.set_field("task-with-dashes_and_underscores", ".state", "done");

        let task = get_task(&src, "task-with-dashes_and_underscores", 0);
        assert_eq!(task.name, "task-with-dashes_and_underscores");
        assert_eq!(task.state, TaskState::Done);
    }

    #[test]
    fn get_task_uses_id_field_when_present() {
        let mut src = InMemoryTaskSource::new();
        src.set_field("parent/my-task", ".id", "my-task-a1b2");

        let task = get_task(&src, "parent/my-task", 1);
        assert_eq!(task.yak_id, "my-task-a1b2");
    }

    #[test]
    fn get_task_falls_back_to_leaf_slug_for_id() {
        let src = InMemoryTaskSource::new();
        let task = get_task(&src, "parent/my-task", 1);
        assert_eq!(task.yak_id, "my-task");
    }
}
