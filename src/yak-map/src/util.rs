use std::collections::BTreeMap;
use unicode_width::UnicodeWidthStr;
use zellij_tile::prelude::*;

/// Escape a string for use inside single-quoted shell literal (replace ' with '\'').
pub fn escape_single_quoted(s: &str) -> String {
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
pub fn copy_via_zellij_tty(yx_name: &str) {
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
pub fn base64_encode(data: &[u8]) -> String {
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
pub fn line_display_width(s: &str) -> usize {
    strip_ansi(s).width()
}

/// Strip ANSI escape sequences (CSI sequences like \x1b[...m) from a string,
/// returning only the visible characters.
pub fn strip_ansi(s: &str) -> String {
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

#[cfg(test)]
mod tests {
    use super::*;

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
    fn line_display_width_counts_emoji_as_two_columns() {
        assert_eq!(line_display_width("\u{1f4cb} foo"), 6);
        assert_eq!(
            line_display_width(&format!("\x1b[33m\u{1f4cb} worklogs\x1b[0m")),
            11
        );
        assert_eq!(line_display_width("hello"), 5);
    }
}
