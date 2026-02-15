# bin/

Local build artifacts. Do not commit these files.

## Required Binaries

| Binary | Source | Purpose |
|--------|--------|---------|
| `yak-box` | Build from `src/yak-box/` | Worker orchestrator |
| `yx` | Build from `src/yaks/` | Task manager CLI |

## Building

```bash
# yak-box
cd src/yak-box && cargo build --release && cp target/release/yak-box ../../bin/

# yx
cd src/yaks && cargo build --release && cp target/release/yx ../../bin/
```