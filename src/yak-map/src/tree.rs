use std::collections::{BTreeMap, HashMap};

use crate::model::TaskLine;
use crate::repository::{get_task, TaskSource};

/// Build the annotated task tree from a task source.
///
/// Collects tasks, then computes tree-display metadata:
/// `has_children`, `is_last_sibling`, and `ancestor_continuations`.
pub fn build(source: &dyn TaskSource) -> Vec<TaskLine> {
    let task_paths = source.list_tasks();
    let mut tasks: Vec<TaskLine> = task_paths
        .into_iter()
        .map(|(path, depth)| get_task(source, &path, depth))
        .collect();

    if tasks.is_empty() {
        return tasks;
    }

    // Mark nodes that have children (for expand/collapse UI hints).
    for i in 0..tasks.len() {
        let prefix = format!("{}/", tasks[i].path);
        tasks[i].has_children = tasks.iter().any(|t| t.path.starts_with(&prefix));
    }

    // Group task indices by their parent path.
    let path_to_index: HashMap<String, usize> = tasks
        .iter()
        .enumerate()
        .map(|(i, t)| (t.path.clone(), i))
        .collect();

    let mut by_parent: BTreeMap<String, Vec<usize>> = BTreeMap::new();
    for (i, task) in tasks.iter().enumerate() {
        let parent = match task.path.rfind('/') {
            Some(pos) => task.path[..pos].to_string(),
            None => String::new(),
        };
        by_parent.entry(parent).or_default().push(i);
    }

    // Mark the last child in each sibling group.
    for indices in by_parent.values() {
        if let Some(&last) = indices.last() {
            tasks[last].is_last_sibling = true;
        }
    }

    // Compute ancestor continuation flags for tree-line drawing.
    // For each task, walk up to each ancestor and check whether that
    // ancestor has more siblings after it (requiring a vertical continuation line).
    let paths: Vec<String> = tasks.iter().map(|t| t.path.clone()).collect();
    for (i, path) in paths.iter().enumerate() {
        let mut continuations = Vec::new();
        let mut current = path.rfind('/').map(|pos| path[..pos].to_string());

        while let Some(ancestor) = current {
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

    tasks
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::repository::InMemoryTaskSource;

    #[test]
    fn build_returns_empty_for_empty_source() {
        let src = InMemoryTaskSource::new();
        let tasks = build(&src);
        assert!(tasks.is_empty());
    }

    #[test]
    fn build_sets_is_last_sibling() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("task-a", 0);
        src.add_task("task-b", 0);
        src.add_task("task-c", 0);

        let tasks = build(&src);

        let task_a = tasks.iter().find(|t| t.name == "task-a").unwrap();
        let task_b = tasks.iter().find(|t| t.name == "task-b").unwrap();
        let task_c = tasks.iter().find(|t| t.name == "task-c").unwrap();

        assert!(!task_a.is_last_sibling);
        assert!(!task_b.is_last_sibling);
        assert!(task_c.is_last_sibling);
    }

    #[test]
    fn build_sets_has_children() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("leaf", 0);
        src.add_task("parent", 0);
        src.add_task("parent/child", 1);

        let tasks = build(&src);

        let parent = tasks.iter().find(|t| t.name == "parent").unwrap();
        let child = tasks.iter().find(|t| t.name == "child").unwrap();
        let leaf = tasks.iter().find(|t| t.name == "leaf").unwrap();

        assert!(parent.has_children);
        assert!(!child.has_children);
        assert!(!leaf.has_children);
    }

    #[test]
    fn build_computes_continuations() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("task-a", 0);
        src.add_task("task-a/child1", 1);
        src.add_task("task-a/child2", 1);

        let tasks = build(&src);

        for task in &tasks {
            eprintln!(
                "{}: depth={}, ancestors={:?}, is_last={}",
                task.path, task.depth, task.ancestor_continuations, task.is_last_sibling
            );
        }
    }

    #[test]
    fn build_continuation_with_parent_sibling() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("parent", 0);
        src.add_task("parent/child", 1);
        src.add_task("parent/child/grandchild", 2);
        src.add_task("parent/child2", 1);

        let tasks = build(&src);

        let grandchild = tasks.iter().find(|t| t.name == "grandchild").unwrap();
        // grandchild's parent (child) has a sibling (child2), so continuation should be true
        assert!(!grandchild.ancestor_continuations.is_empty());
        assert!(grandchild.ancestor_continuations.contains(&true));
    }
}
