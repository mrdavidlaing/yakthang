Changes made:
1. Cargo.toml: zellij-tile 0.43 → 0.44, updated atty patch comment
2. main.rs:70-77: Added 6th arg (None) to FloatingPaneCoordinates::new() — new pinned parameter in 0.44 API
3. main.rs:25,41,81-89: Added last_yaks_mtime field for mtime-based change detection (linter addition — avoids re-parsing tasks when .yaks dir unchanged)
4. util.rs:126: Replaced unwrap() with if-let for safe ANSI bracket consumption

Build: cargo build --target wasm32-wasip1 --release — clean, no warnings
Tests: 70/70 pass
