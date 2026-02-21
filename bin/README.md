# bin/

Local build artifacts. Do not commit these files.

## Required Binaries

| Binary | Source | Purpose |
|--------|--------|---------|
| `yak-box` | Build from `src/yak-box/` | Worker orchestrator (Go) |
| `yak-map.wasm` | Build from `src/yak-map/` | YakMap Zellij plugin (Rust/WASM) |
| `yx` | Build from `src/yaks/` | Task manager CLI |

## Scripts

| Script | Purpose |
|--------|---------|
| `archive-yaks.sh` | Archive completed tasks to `memory/` directory |

## Building

```bash
# yak-box
cd src/yak-box && go build -o ../../bin/yak-box .

# yak-map
cd src/yak-map && cargo build --release --target wasm32-wasip1 && \
  cp target/wasm32-wasip1/release/yak-map.wasm ../../bin/

# yx
cd src/yaks && cargo build --release && cp target/release/yx ../../bin/
```