pub mod ansi {
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

impl std::str::FromStr for TaskState {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "wip" => Ok(TaskState::Wip),
            "done" => Ok(TaskState::Done),
            "todo" => Ok(TaskState::Todo),
            _ => Err(format!(
                "Invalid task state '{}'. Valid states are: todo, wip, done",
                s
            )),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum AgentStatusKind {
    Blocked,
    Done,
    Wip,
    Unknown,
}

impl AgentStatusKind {
    pub fn from_status_string(s: &str) -> Self {
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

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum ReviewStatusKind {
    Pass,
    Fail,
    InProgress,
    Unknown,
}

impl ReviewStatusKind {
    pub fn from_status_string(s: &str) -> Self {
        let v = s.trim().to_lowercase();
        if v.starts_with("in-progress") || v.starts_with("in_progress") {
            ReviewStatusKind::InProgress
        } else if v.starts_with("pass") {
            ReviewStatusKind::Pass
        } else if v.starts_with("fail") {
            ReviewStatusKind::Fail
        } else {
            ReviewStatusKind::Unknown
        }
    }
}

/// Yakob-owned wip sub-state. When a task is in `wip`, this field tracks
/// where it sits in the shaving workflow. Yakob drives all transitions;
/// shavers communicate intent via `shaver-message`.
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum WipState {
    /// 🪒 Shaver is actively working
    Shaving,
    /// 🚫 Shaver reported stuck, Yakob intervening
    Blocked,
    /// 💤 Parked intentionally
    Sleeping,
    /// 👀🙏 Shaver thinks it's done, awaiting sniff test
    ReadyForSniffTest,
    /// 👀 Sniff-test reviewer agent is running
    UnderReview,
    /// 👀❌ Review found gaps, needs rework
    FailedSniffTest,
    /// 👀🧑 Passed sniff test, human should verify
    ReadyForHuman,
}

impl WipState {
    pub fn from_field(s: &str) -> Option<Self> {
        match s.trim() {
            "shaving" => Some(WipState::Shaving),
            "blocked" => Some(WipState::Blocked),
            "sleeping" => Some(WipState::Sleeping),
            "ready-for-sniff-test" => Some(WipState::ReadyForSniffTest),
            "under-review" => Some(WipState::UnderReview),
            "failed-sniff-test" => Some(WipState::FailedSniffTest),
            "ready-for-human" => Some(WipState::ReadyForHuman),
            _ => None,
        }
    }

    pub fn emoji(self) -> &'static str {
        match self {
            WipState::Shaving => "🪒",
            WipState::Blocked => "🚫",
            WipState::Sleeping => "💤",
            WipState::ReadyForSniffTest => "👀🙏",
            WipState::UnderReview => "👀",
            WipState::FailedSniffTest => "👀❌",
            WipState::ReadyForHuman => "👀🧑",
        }
    }
}

/// The resolved visual state for a task, determined by priority:
/// 1. wip-state (Yakob-owned) takes precedence for wip tasks
/// 2. Legacy agent-status (if present) provides backward compatibility
/// 3. TaskState (.state) is the base fallback
///
/// Key distinction: `AgentDone` (agent reported done, shown green) vs
/// `TaskDone` (yx state is done but no agent confirmation, shown dim).
#[derive(Debug, Clone, Copy, PartialEq)]
pub enum ResolvedVisualState {
    /// wip-state: blocked OR agent-status: "blocked:..." → red ●
    Blocked,
    /// agent-status: "done:..." → green ✓
    AgentDone,
    /// wip-state: shaving/under-review/etc OR agent-status: "wip:..." → yellow ●
    Wip,
    /// TaskState::Done (no overrides) → dim ✓
    TaskDone,
    /// TaskState::Todo → white ○
    Todo,
    /// wip-state: sleeping → dim ●
    Sleeping,
}

impl ResolvedVisualState {
    /// Resolve the visual state from wip-state, legacy agent-status, and task state.
    /// Priority: wip-state > agent-status > .state
    pub fn resolve(
        wip_state: Option<WipState>,
        agent_status: Option<&str>,
        state: TaskState,
    ) -> Self {
        // Layer 1: wip-state (Yakob-owned) takes precedence — only when task is wip
        if state == TaskState::Wip {
            if let Some(ws) = wip_state {
                return match ws {
                    WipState::Blocked => ResolvedVisualState::Blocked,
                    WipState::Sleeping => ResolvedVisualState::Sleeping,
                    _ => ResolvedVisualState::Wip,
                };
            }
        }
        // Layer 2: legacy agent-status for backward compatibility
        if let Some(status) = agent_status {
            match AgentStatusKind::from_status_string(status) {
                AgentStatusKind::Blocked => return ResolvedVisualState::Blocked,
                AgentStatusKind::Done => return ResolvedVisualState::AgentDone,
                AgentStatusKind::Wip => return ResolvedVisualState::Wip,
                AgentStatusKind::Unknown => {}
            }
        }
        // Layer 3: base task state
        match state {
            TaskState::Wip => ResolvedVisualState::Wip,
            TaskState::Done => ResolvedVisualState::TaskDone,
            TaskState::Todo => ResolvedVisualState::Todo,
        }
    }
}

#[derive(Debug, Clone)]
pub struct TaskLine {
    pub path: String,
    pub name: String,
    pub yak_id: String,
    pub depth: usize,
    pub state: TaskState,
    pub assigned_to: Option<String>,
    pub wip_state: Option<WipState>,
    pub agent_status: Option<String>,
    pub review_status: Option<String>,
    pub has_children: bool,
    pub is_last_sibling: bool,
    pub ancestor_continuations: Vec<bool>,
}

impl TaskLine {
    pub fn resolved_visual_state(&self) -> ResolvedVisualState {
        ResolvedVisualState::resolve(self.wip_state, self.agent_status.as_deref(), self.state)
    }
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
            wip_state: None,
            agent_status: None,
            review_status: None,
            has_children: false,
            is_last_sibling: false,
            ancestor_continuations: Vec::new(),
        }
    }
}
