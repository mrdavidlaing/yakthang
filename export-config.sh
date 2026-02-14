#!/usr/bin/env bash
#
# export-config.sh — Export all config and state from this Ubuntu VM
# for migration to a new machine.
#
# Captures: ~/.openclaw/, .yaks, workspace files, systemd services,
#           cron jobs, Docker images. Outputs a timestamped tarball
#           with manifest.
#
# Idempotent: safe to run multiple times. Uses a fresh staging dir
# each run and cleans up on exit.
#
# Usage:
#   ./export-config.sh                  # default output: ~/yakthang-export-<timestamp>.tar.gz
#   ./export-config.sh /path/to/out.tar.gz  # custom output path
#
set -euo pipefail

# ── Constants ─────────────────────────────────────────────────────────
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
HOSTNAME=$(hostname)
DEFAULT_OUTPUT="$HOME/yakthang-export-${TIMESTAMP}.tar.gz"
OUTPUT="${1:-$DEFAULT_OUTPUT}"
STAGING=$(mktemp -d "/tmp/yakthang-export-XXXXXX")
WORKSPACE="$HOME/yakthang"
MANIFEST="${STAGING}/MANIFEST.txt"
ERRORS=()

# ── Helpers ───────────────────────────────────────────────────────────
log()  { printf '\033[1;34m[export]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2; }
fail() { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; ERRORS+=("$*"); }

cleanup() {
    if [[ -d "$STAGING" ]]; then
        rm -rf "$STAGING"
        log "Cleaned up staging dir"
    fi
}
trap cleanup EXIT

manifest_add() {
    local section="$1" path="$2" note="${3:-}"
    local size
    if [[ -e "${STAGING}/${path}" ]]; then
        size=$(du -sh "${STAGING}/${path}" 2>/dev/null | cut -f1)
        printf '%-24s %-50s %s  %s\n' "[$section]" "$path" "$size" "$note" >> "$MANIFEST"
    fi
}

copy_if_exists() {
    local src="$1" dest="$2" section="$3" note="${4:-}"
    if [[ -e "$src" ]]; then
        mkdir -p "$(dirname "${STAGING}/${dest}")"
        cp -a "$src" "${STAGING}/${dest}"
        manifest_add "$section" "$dest" "$note"
        return 0
    else
        warn "Skipped (not found): $src"
        return 1
    fi
}

copy_dir_if_exists() {
    local src="$1" dest="$2" section="$3" note="${4:-}"
    if [[ -d "$src" ]]; then
        mkdir -p "${STAGING}/${dest}"
        cp -a "$src/." "${STAGING}/${dest}/"
        manifest_add "$section" "$dest" "$note"
        return 0
    else
        warn "Skipped dir (not found): $src"
        return 1
    fi
}

# ── Header ────────────────────────────────────────────────────────────
log "Starting export at ${TIMESTAMP}"
log "Staging to: ${STAGING}"
log "Output: ${OUTPUT}"

cat > "$MANIFEST" <<EOF
# Yakthang VM Export Manifest
# Generated: ${TIMESTAMP}
# Hostname:  ${HOSTNAME}
# User:      $(whoami)
# ──────────────────────────────────────────────────────────────────────
# Section                  Path                                       Size  Note
EOF

# ── 1. ~/.openclaw/ (home config) ────────────────────────────────────
log "1/6  ~/.openclaw/ config"
OPENCLAW_HOME="$HOME/.openclaw"
if [[ -d "$OPENCLAW_HOME" ]]; then
    mkdir -p "${STAGING}/openclaw-home"

    # Core config
    copy_if_exists "$OPENCLAW_HOME/openclaw.json"      "openclaw-home/openclaw.json"      "openclaw" "main config"

    # Identity & device pairing (critical for migration)
    copy_dir_if_exists "$OPENCLAW_HOME/identity"       "openclaw-home/identity"            "openclaw" "device identity"
    copy_dir_if_exists "$OPENCLAW_HOME/devices"        "openclaw-home/devices"             "openclaw" "paired devices"
    copy_dir_if_exists "$OPENCLAW_HOME/credentials"    "openclaw-home/credentials"         "openclaw" "credentials"

    # Agent sessions & config
    copy_dir_if_exists "$OPENCLAW_HOME/agents"         "openclaw-home/agents"              "openclaw" "agent config + sessions"

    # Cron jobs (openclaw-managed)
    copy_dir_if_exists "$OPENCLAW_HOME/cron"           "openclaw-home/cron"                "openclaw" "scheduled jobs"

    # Shell completions
    copy_dir_if_exists "$OPENCLAW_HOME/completions"    "openclaw-home/completions"         "openclaw" "shell completions"

    # Canvas / UI state
    copy_dir_if_exists "$OPENCLAW_HOME/canvas"         "openclaw-home/canvas"              "openclaw" "canvas state"

    # Update check
    copy_if_exists "$OPENCLAW_HOME/update-check.json"  "openclaw-home/update-check.json"   "openclaw" "update check"
else
    fail "~/.openclaw/ not found"
fi

# ── 2. .yaks (task tracker state) ────────────────────────────────────
log "2/6  .yaks task tracker"
if [[ -d "$WORKSPACE/.yaks" ]]; then
    # Copy full yaks tree (task definitions, state, context)
    copy_dir_if_exists "$WORKSPACE/.yaks" "yaks" "yaks" "task tracker state"
else
    fail ".yaks directory not found in workspace"
fi

# ── 3. Workspace files ───────────────────────────────────────────────
log "3/6  Workspace files"
mkdir -p "${STAGING}/workspace"

# Scripts
for f in "$WORKSPACE"/*.sh; do
    [[ -f "$f" ]] || continue
    basename_f=$(basename "$f")
    [[ "$basename_f" == "export-config.sh" ]] && continue  # don't export ourselves mid-write
    copy_if_exists "$f" "workspace/${basename_f}" "workspace" "script"
done

# Config files
copy_if_exists "$WORKSPACE/orchestrator.kdl"     "workspace/orchestrator.kdl"     "workspace" "zellij layout"
copy_if_exists "$WORKSPACE/worker.Dockerfile"    "workspace/worker.Dockerfile"    "workspace" "worker image def"
copy_if_exists "$WORKSPACE/opencode.json"        "workspace/opencode.json"        "workspace" "opencode config"
copy_if_exists "$WORKSPACE/oh-my-opencode.json"  "workspace/oh-my-opencode.json"  "workspace" "oh-my-opencode config"
copy_if_exists "$WORKSPACE/launch.sh"            "workspace/launch.sh"            "workspace" "launch script"
copy_if_exists "$WORKSPACE/.gitignore"           "workspace/.gitignore"           "workspace" "gitignore"

# Docs directory
copy_dir_if_exists "$WORKSPACE/docs"             "workspace/docs"                 "workspace" "documentation"

# Themes
copy_dir_if_exists "$WORKSPACE/themes"           "workspace/themes"               "workspace" "UI themes"

# .claude config (workspace-level)
copy_dir_if_exists "$WORKSPACE/.claude"          "workspace/.claude"              "workspace" "claude config"

# .opencode agents/personalities (skip node_modules, plans, lock files)
if [[ -d "$WORKSPACE/.opencode" ]]; then
    mkdir -p "${STAGING}/workspace/.opencode"
    copy_dir_if_exists "$WORKSPACE/.opencode/agents"        "workspace/.opencode/agents"        "workspace" "opencode agents"
    copy_dir_if_exists "$WORKSPACE/.opencode/personalities"  "workspace/.opencode/personalities"  "workspace" "opencode personalities"
    copy_if_exists     "$WORKSPACE/.opencode/package.json"   "workspace/.opencode/package.json"   "workspace" "opencode package.json"
fi

# .openclaw workspace (inside repo)
copy_dir_if_exists "$WORKSPACE/.openclaw"        "workspace/.openclaw"            "workspace" "openclaw workspace"

# Worker cost tracking
copy_dir_if_exists "$WORKSPACE/.worker-costs"    "workspace/.worker-costs"        "workspace" "cost tracking data"

# Sisyphus state
copy_dir_if_exists "$WORKSPACE/.sisyphus"        "workspace/.sisyphus"            "workspace" "sisyphus agent state"

# ── 4. Systemd services ──────────────────────────────────────────────
log "4/6  Systemd services"
mkdir -p "${STAGING}/systemd"

for svc in openclaw-gateway yak-orchestrator; do
    svc_file="/etc/systemd/system/${svc}.service"
    if [[ -f "$svc_file" ]]; then
        cp -a "$svc_file" "${STAGING}/systemd/"
        manifest_add "systemd" "systemd/${svc}.service" "unit file"
    fi

    # Override directories
    override_dir="/etc/systemd/system/${svc}.service.d"
    if [[ -d "$override_dir" ]]; then
        mkdir -p "${STAGING}/systemd/${svc}.service.d"
        cp -a "$override_dir/." "${STAGING}/systemd/${svc}.service.d/"
        manifest_add "systemd" "systemd/${svc}.service.d/" "overrides (contains secrets!)"
    fi
done

# Capture enabled/active state
{
    echo "# Systemd service state at export time"
    echo "# Generated: ${TIMESTAMP}"
    for svc in openclaw-gateway yak-orchestrator; do
        printf '\n## %s\n' "$svc"
        systemctl is-enabled "${svc}.service" 2>/dev/null || echo "(not found)"
        systemctl is-active "${svc}.service" 2>/dev/null || true
    done
} > "${STAGING}/systemd/service-state.txt"
manifest_add "systemd" "systemd/service-state.txt" "enabled/active state"

# ── 5. Cron jobs ──────────────────────────────────────────────────────
log "5/6  Cron jobs"
mkdir -p "${STAGING}/cron"

# User crontab
if crontab -l &>/dev/null; then
    crontab -l > "${STAGING}/cron/user-crontab.txt" 2>/dev/null
    manifest_add "cron" "cron/user-crontab.txt" "user crontab"
else
    echo "# No user crontab configured" > "${STAGING}/cron/user-crontab.txt"
    manifest_add "cron" "cron/user-crontab.txt" "empty (no user crontab)"
fi

# System crontab (best-effort, may need sudo)
if sudo -n crontab -l &>/dev/null 2>&1; then
    sudo -n crontab -l > "${STAGING}/cron/root-crontab.txt" 2>/dev/null
    manifest_add "cron" "cron/root-crontab.txt" "root crontab"
fi

# /etc/cron.d entries (non-default)
if [[ -d /etc/cron.d ]]; then
    for f in /etc/cron.d/*; do
        [[ -f "$f" ]] || continue
        basename_f=$(basename "$f")
        # Skip default system cron entries
        case "$basename_f" in
            e2scrub_all|popularity-contest|.placeholder) continue ;;
        esac
        cp "$f" "${STAGING}/cron/cron.d-${basename_f}"
        manifest_add "cron" "cron/cron.d-${basename_f}" "/etc/cron.d entry"
    done
fi

# OpenClaw managed cron (already captured in section 1, note it here)
echo "# OpenClaw cron jobs are in openclaw-home/cron/jobs.json" >> "${STAGING}/cron/README.txt"
manifest_add "cron" "cron/README.txt" "cron notes"

# ── 6. Docker images ─────────────────────────────────────────────────
log "6/6  Docker images"
mkdir -p "${STAGING}/docker"

# Save image list
docker images --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}' \
    > "${STAGING}/docker/image-list.txt" 2>/dev/null || true
manifest_add "docker" "docker/image-list.txt" "image inventory"

# Save the yak-worker image (the custom one we built)
if docker image inspect yak-worker:latest &>/dev/null; then
    log "  Saving yak-worker:latest (this may take a moment)..."
    docker save yak-worker:latest | gzip > "${STAGING}/docker/yak-worker-latest.tar.gz"
    manifest_add "docker" "docker/yak-worker-latest.tar.gz" "worker image"
else
    warn "yak-worker:latest image not found, skipping"
fi

# Save container list (for reference, not the containers themselves)
docker ps -a --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' \
    > "${STAGING}/docker/container-list.txt" 2>/dev/null || true
manifest_add "docker" "docker/container-list.txt" "running containers (reference)"

# ── 7. Extra: user-level config ──────────────────────────────────────
log "  Bonus: user-level configs"
mkdir -p "${STAGING}/user-config"

# Zellij config
copy_dir_if_exists "$HOME/.config/zellij" "user-config/zellij" "user-config" "zellij config"

# gh CLI config
copy_dir_if_exists "$HOME/.config/gh" "user-config/gh" "user-config" "GitHub CLI config"

# opencode config (under .config)
copy_dir_if_exists "$HOME/.config/opencode" "user-config/opencode" "user-config" "opencode config"

# .claude transcripts
copy_dir_if_exists "$HOME/.claude" "user-config/.claude" "user-config" "claude transcripts"

# ── Build tarball ─────────────────────────────────────────────────────
log "Building tarball..."

# Finalize manifest
{
    echo ""
    echo "# ──────────────────────────────────────────────────────────────────────"
    echo "# Errors/Warnings:"
    if [[ ${#ERRORS[@]} -gt 0 ]]; then
        for e in "${ERRORS[@]}"; do
            echo "#   ERROR: $e"
        done
    else
        echo "#   None"
    fi
    echo ""
    echo "# Total staged size:"
    du -sh "$STAGING" | cut -f1
} >> "$MANIFEST"

# Create tarball with restrictive permissions (contains secrets)
tar czf "$OUTPUT" -C "$STAGING" .
chmod 600 "$OUTPUT"

# ── Summary ───────────────────────────────────────────────────────────
SIZE=$(du -sh "$OUTPUT" | cut -f1)
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log "Export complete: $OUTPUT ($SIZE)"
log "Manifest is embedded in the tarball as MANIFEST.txt"
log ""
log "⚠️  This tarball contains secrets (API keys, tokens)."
log "   Transfer securely (scp, not public URLs)."
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    warn "${#ERRORS[@]} error(s) during export — check manifest"
    exit 1
fi
