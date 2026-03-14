use crate::model::ansi;
use crate::model::{AgentStatusKind, ReviewStatusKind, TaskLine, TaskState};

/// Map review-status field value to display emoji: 🔍 in-progress, ✅ pass, ❌ fail.
/// Uses prefix matching (e.g. "pass: summary", "fail: missing tests") like agent-status.
pub fn review_status_emoji(value: &str) -> Option<&'static str> {
    match ReviewStatusKind::from_status_string(value) {
        ReviewStatusKind::InProgress => Some("🔍"),
        ReviewStatusKind::Pass => Some("✅"),
        ReviewStatusKind::Fail => Some("❌"),
        ReviewStatusKind::Unknown => None,
    }
}

pub fn task_color(task: &TaskLine) -> &'static str {
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

pub fn status_symbol(task: &TaskLine) -> char {
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

pub fn tree_prefix(task: &TaskLine) -> String {
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

pub fn highlight_line(line: &str, padding: &str) -> String {
    let bg = ansi::BG_SELECTED;
    let highlighted = line.replace(ansi::RESET, &format!("{}{bg}", ansi::RESET));
    format!("{bg}{}{}{}", highlighted, padding, ansi::RESET)
}

pub fn render_task(task: &TaskLine) -> String {
    let prefix = tree_prefix(task);
    let status = status_symbol(task);

    let color = task_color(task);

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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::InMemoryTaskSource;

    fn build_tasks_from(source: &dyn crate::TaskSource) -> Vec<TaskLine> {
        crate::tree::build(source)
    }

    #[test]
    fn task_color_red_for_blocked() {
        let task = TaskLine {
            agent_status: Some("blocked: waiting".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(task_color(&task), ansi::RED);
    }

    #[test]
    fn task_color_green_for_done() {
        let task = TaskLine {
            agent_status: Some("done: finished".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(task_color(&task), ansi::GREEN);
    }

    #[test]
    fn task_color_yellow_for_wip() {
        let task = TaskLine {
            agent_status: Some("wip: working".to_string()),
            ..TaskLine::default()
        };
        assert_eq!(task_color(&task), ansi::YELLOW);
    }

    #[test]
    fn task_color_yellow_when_state_is_wip() {
        let task = TaskLine {
            state: TaskState::Wip,
            agent_status: None,
            ..TaskLine::default()
        };
        assert_eq!(task_color(&task), ansi::YELLOW);
    }

    #[test]
    fn task_color_white_for_todo() {
        let task = TaskLine {
            state: TaskState::Todo,
            agent_status: None,
            ..TaskLine::default()
        };
        assert_eq!(task_color(&task), ansi::WHITE);
    }

    #[test]
    fn tree_prefix_depth_2_parent_has_sibling_shows_continuation() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("parent", 0);
        src.add_task("parent/child", 1);
        src.add_task("parent/child/grandchild", 2);
        src.add_task("parent/child2", 1);

        let tasks = build_tasks_from(&src);
        let grandchild = tasks.iter().find(|t| t.name == "grandchild").unwrap();
        let prefix = tree_prefix(grandchild);
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
        let mut src = InMemoryTaskSource::new();
        src.add_task("task-a", 0);
        src.add_task("task-a/child1", 1);
        src.add_task("task-a/child2", 1);

        let tasks = build_tasks_from(&src);
        let child2 = tasks.iter().find(|t| t.name == "child2").unwrap();
        let prefix = tree_prefix(child2);
        assert_eq!(prefix, format!("{}╰─{}", ansi::DIM, ansi::RESET));
    }

    #[test]
    fn tree_prefix_depth_2_no_continuation_when_parent_is_last() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("parent", 0);
        src.add_task("parent/child", 1);
        src.add_task("parent/child/grandchild", 2);

        let tasks = build_tasks_from(&src);
        let grandchild = tasks.iter().find(|t| t.name == "grandchild").unwrap();
        let prefix = tree_prefix(grandchild);
        assert_eq!(prefix, format!("  {}╰─{}", ansi::DIM, ansi::RESET));
    }

    #[test]
    fn tree_prefix_depth_3_shows_two_continuation_columns() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("a", 0);
        src.add_task("a/b", 1);
        src.add_task("a/b/c", 2);
        src.add_task("a/b/c/d", 3);
        src.add_task("a/b2", 1);

        let tasks = build_tasks_from(&src);
        let d = tasks.iter().find(|t| t.name == "d").unwrap();
        let prefix = tree_prefix(d);
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
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", ".state", "wip");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);

        assert!(rendered.contains("●"), "rendered: {:?}", rendered);
    }

    #[test]
    fn render_task_done_shows_strikethrough() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", ".state", "done");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);

        assert!(rendered.contains(ansi::STRIKETHROUGH));
        assert!(rendered.contains("my-task"));
        assert!(rendered.contains(ansi::RESET));
        assert!(rendered.contains("✓"), "rendered: {:?}", rendered);
    }

    #[test]
    fn render_task_todo_shows_white() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);

        assert!(rendered.contains("○"));
        assert!(rendered.contains(ansi::WHITE));
    }

    #[test]
    fn render_task_with_assignment_shows_agent() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", "assigned-to", "bob");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        assert!(
            task.assigned_to.is_some(),
            "assigned_to: {:?}",
            task.assigned_to
        );

        let rendered = render_task(task);
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
        assert_eq!(review_status_emoji("pass: summary"), Some("✅"));
        assert_eq!(review_status_emoji("pass: looks good"), Some("✅"));
        assert_eq!(review_status_emoji("fail: summary"), Some("❌"));
        assert_eq!(review_status_emoji("fail: missing tests"), Some("❌"));
    }

    #[test]
    fn render_task_with_review_status_shows_emoji() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", "review-status", "pass");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);
        assert!(
            rendered.contains("✅"),
            "pass should render ✅: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_fail_shows_cross() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", "review-status", "fail");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);
        assert!(
            rendered.contains("❌"),
            "fail should render ❌: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_in_progress_shows_magnifier() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", "review-status", "in-progress");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);
        assert!(
            rendered.contains("🔍"),
            "in-progress should render 🔍: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_pass_looks_good_shows_check() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", "review-status", "pass: looks good");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);
        assert!(
            rendered.contains("✅"),
            "pass: looks good should render ✅: {:?}",
            rendered
        );
    }

    #[test]
    fn render_task_with_review_status_fail_missing_tests_shows_cross() {
        let mut src = InMemoryTaskSource::new();
        src.add_task("my-task", 0);
        src.set_field("my-task", "review-status", "fail: missing tests");

        let tasks = build_tasks_from(&src);
        let task = tasks.iter().find(|t| t.name == "my-task").unwrap();
        let rendered = render_task(task);
        assert!(
            rendered.contains("❌"),
            "fail: missing tests should render ❌: {:?}",
            rendered
        );
    }

    #[test]
    fn highlight_line_uses_explicit_bg_not_reverse_video() {
        let result = highlight_line("hello", "   ");
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
        let line = &format!("{}foo{}bar", ansi::GREEN, ansi::RESET);
        let result = highlight_line(line, "");
        assert!(
            result.contains(&format!("{}{}", ansi::RESET, ansi::BG_SELECTED)),
            "bg not re-established after reset: {:?}",
            result
        );
    }

    #[test]
    fn highlight_line_padding_uses_same_bg() {
        let result = highlight_line("hi", "     ");
        assert!(result.starts_with(ansi::BG_SELECTED));
        let reset_pos = result.rfind(ansi::RESET).unwrap();
        assert!(
            reset_pos == result.len() - ansi::RESET.len(),
            "final reset should be at end: {:?}",
            result
        );
    }
}
