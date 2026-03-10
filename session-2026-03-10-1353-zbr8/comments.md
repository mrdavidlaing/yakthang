## Yak Wrap — 2026-03-10 14:03

### Highlights

- Sandboxed workers can now use yx natively — the Dockerfile pulls the upstream statically-linked release binary instead of copying a locally-built dynamic one.

### Shaved Yaks

#### worker runtime fixes
- **static link yx** — Discovered the upstream release binary is already statically linked. Replaced the COPY-based install in the Dockerfile with the upstream `install.sh` script. Removed yx binary staging from `setup-vm.sh`. Added `unzip` to apt deps. Verified with `ldd` in Ubuntu 24.04 container.

### Interesting Findings

- The upstream yx binary was statically linked all along — the problem was our build/install path, not the binary itself. No Cargo.toml changes needed.

### Loose Ends

- None identified this session.
