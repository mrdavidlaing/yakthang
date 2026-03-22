mod task;

pub use task::render_task;

use crate::model::ansi;

pub fn highlight_line(line: &str, padding: &str) -> String {
    let bg = ansi::BG_SELECTED;
    let highlighted = line.replace(ansi::RESET, &format!("{}{bg}", ansi::RESET));
    format!("{bg}{}{}{}", highlighted, padding, ansi::RESET)
}

#[cfg(test)]
mod tests {
    use super::*;

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
