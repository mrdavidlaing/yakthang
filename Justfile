# yakthang Justfile
# Build and install yakthang tools to ~/.local/bin/

default: build

# Build all tools
build: build-yx build-yak-box build-yak-map

# Build yx (Rust)
build-yx:
    cd src/yaks && cargo build --release

# Build yak-box (Go)
build-yak-box:
    cd src/yak-box && go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o yak-box .

# Build yak-map WASM plugin (Rust/WASM)
build-yak-map:
    cd src/yak-map && cargo build --target wasm32-wasip1 --release

# Build and install all tools
install: install-yx install-yak-box install-yak-map

# Build and install yx
install-yx: build-yx
    cp src/yaks/target/release/yx ~/.local/bin/yx

# Build and install yak-box
install-yak-box: build-yak-box
    cp src/yak-box/yak-box ~/.local/bin/yak-box

# Build and install yak-map WASM plugin to shared Zellij plugin dir
install-yak-map: build-yak-map
    mkdir -p ~/.local/share/zellij/plugins
    cp src/yak-map/target/wasm32-wasip1/release/yak-map.wasm ~/.local/share/zellij/plugins/yak-map.wasm

# Launch yakstead Zellij session
launch: install
    yakstead/launch.sh

# Clean all build artifacts
clean:
    cd src/yaks && cargo clean
    cd src/yak-map && cargo clean
    rm -f src/yak-box/yak-box
