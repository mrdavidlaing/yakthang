// Safe replacement for the atty crate (RUSTSEC-2021-0145).
// Uses std::io::IsTerminal (stable since Rust 1.70) instead of the
// unsafe unaligned-read approach in the original atty 0.2.14.
// On WASM targets, IsTerminal always returns false, which is correct.

pub enum Stream {
    Stdin,
    Stdout,
    Stderr,
}

pub fn is(stream: Stream) -> bool {
    use std::io::IsTerminal;
    match stream {
        Stream::Stdin => std::io::stdin().is_terminal(),
        Stream::Stdout => std::io::stdout().is_terminal(),
        Stream::Stderr => std::io::stderr().is_terminal(),
    }
}

pub fn isnt(stream: Stream) -> bool {
    !is(stream)
}
