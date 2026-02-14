#!/usr/bin/env bash
#
# import-config.sh — Import config and state from an export tarball
# onto a new machine (designed for baljeet migration).
#
# Counterpart to export-config.sh. Restores:
#   1. ~/.openclaw/       (gateway config, identity, credentials)
#   2. .yaks              (task tracker state)
#   3. workspace files    (scripts, configs, .opencode, .openclaw)
#   4. systemd services   (unit files + overrides)
#   5. cron jobs          (user crontab + /etc/cron.d entries)
#   6. Docker images      (yak-worker:latest)
#   7. user-level config  (zellij, gh, opencode, .claude)
#
# Safety:
#   - Backs up existing files before overwriting (*.bak-import-<timestamp>)
#   - --dry-run mode shows what would happen without touching anything
#   - Idempotent: safe to run multiple times
#   - Rewrites hardcoded paths from source machine to target
#
# Prerequisites:
#   - setup-vm.sh (or install-on-arch) already run on target
#   - Running as the target user (yakob), with sudo available for systemd/cron
#
# Usage:
#   ./import-config.sh /path/to/yakthang-export-*.tar.gz
#   ./import-config.sh --dry-run /path/to/yakthang-export-*.tar.gz
#
set -euo pipefail

# ── Constants ─────────────────────────────────────────────────────────
TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP_SUFFIX="bak-import-${TIMESTAMP}"
WORKSPACE="$HOME/yakthang"
OPENCLAW_HOME="$HOME/.openclaw"
DRY_RUN=false
ERRORS=()
WARNINGS=()
RESTORED=0
SKIPPED=0
BACKED_UP=0

# ── Argument parsing ─────────────────────────────────────────────────
usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS] <export-tarball>

Import config and state from a yakthang export tarball.

Options:
  --dry-run    Show what would be done without making changes
  --help       Show this help message

Arguments:
  <export-tarball>  Path to yakthang-export-*.tar.gz from export-config.sh

Examples:
  $(basename "$0") ~/yakthang-export-20250214T120000Z.tar.gz
  $(basename "$0") --dry-run ~/yakthang-export-20250214T120000Z.tar.gz
EOF
    exit "${1:-0}"
}

TARBALL=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run) DRY_RUN=true; shift ;;
        --help|-h) usage 0 ;;
        -*)        echo "Unknown option: $1" >&2; usage 1 ;;
        *)
            if [[ -n "$TARBALL" ]]; then
                echo "Error: multiple tarballs specified" >&2; usage 1
            fi
            TARBALL="$1"; shift
            ;;
    esac
done

if [[ -z "$TARBALL" ]]; then
    echo "Error: no tarball specified" >&2
    usage 1
fi

if [[ ! -f "$TARBALL" ]]; then
    echo "Error: tarball not found: $TARBALL" >&2
    exit 1
fi

# ── Helpers ───────────────────────────────────────────────────────────
log()  { printf '\033[1;34m[import]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2; WARNINGS+=("$*"); }
fail() { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; ERRORS+=("$*"); }
dry()  { printf '\033[1;35m[dry-run]\033[0m %s\n' "$*"; }

# Backup a file/dir before overwriting. Returns 0 if backup was made.
backup_existing() {
    local path="$1"
    if [[ -e "$path" ]]; then
        local backup="${path}.${BACKUP_SUFFIX}"
        if $DRY_RUN; then
            dry "Would backup: $path -> $backup"
        else
            cp -a "$path" "$backup"
            ((BACKED_UP++))
        fi
        return 0
    fi
    return 1
}

# Restore a file from staging to destination.
# Skips if source and dest are byte-identical.
restore_file() {
    local src="$1" dest="$2" desc="${3:-file}"
    if [[ ! -e "$src" ]]; then
        return 1
    fi

    # Skip if identical
    if [[ -f "$dest" ]] && cmp -s "$src" "$dest"; then
        log "  Skip (identical): $desc"
        ((SKIPPED++))
        return 0
    fi

    if $DRY_RUN; then
        if [[ -e "$dest" ]]; then
            dry "Would overwrite: $dest ($desc)"
        else
            dry "Would create: $dest ($desc)"
        fi
    else
        backup_existing "$dest" || true
        mkdir -p "$(dirname "$dest")"
        cp -a "$src" "$dest"
        ((RESTORED++))
        log "  Restored: $desc"
    fi
    return 0
}

# Restore a directory tree from staging to destination.
# Merges into existing directory, backing up changed files.
restore_dir() {
    local src="$1" dest="$2" desc="${3:-directory}"
    if [[ ! -d "$src" ]]; then
        return 1
    fi

    if $DRY_RUN; then
        local count
        count=$(find "$src" -type f | wc -l)
        dry "Would restore dir: $dest ($count files, $desc)"
        return 0
    fi

    # If dest exists and is a directory, merge; otherwise backup and replace
    if [[ -d "$dest" ]]; then
        # Merge: walk source files, backup+overwrite changed ones
        while IFS= read -r -d '' src_file; do
            local rel="${src_file#"$src"/}"
            local dest_file="$dest/$rel"
            restore_file "$src_file" "$dest_file" "$desc/$rel"
        done < <(find "$src" -type f -print0)
    else
        backup_existing "$dest" || true
        mkdir -p "$(dirname "$dest")"
        cp -a "$src" "$dest"
        ((RESTORED++))
        log "  Restored dir: $desc"
    fi
    return 0
}

# Rewrite paths in a file. Replaces old home/user with current.
# Only touches the file if the old path actually appears in it.
rewrite_paths() {
    local file="$1" old_home="$2" new_home="$3"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    if [[ "$old_home" == "$new_home" ]]; then
        return 0  # nothing to rewrite
    fi
    if grep -q "$old_home" "$file" 2>/dev/null; then
        if $DRY_RUN; then
            dry "Would rewrite paths in: $file ($old_home -> $new_home)"
        else
            sed -i "s|${old_home}|${new_home}|g" "$file"
            log "  Rewrote paths: $file"
        fi
    fi
}

# ── Extract tarball ──────────────────────────────────────────────────
STAGING=$(mktemp -d "/tmp/yakthang-import-XXXXXX")
cleanup() {
    if [[ -d "$STAGING" ]]; then
        rm -rf "$STAGING"
        log "Cleaned up staging dir"
    fi
}
trap cleanup EXIT

log "Extracting tarball: $TARBALL"
tar xzf "$TARBALL" -C "$STAGING"

# ── Detect source machine info from manifest ─────────────────────────
SOURCE_USER=""
SOURCE_HOME=""
if [[ -f "$STAGING/MANIFEST.txt" ]]; then
    SOURCE_USER=$(grep '^# User:' "$STAGING/MANIFEST.txt" | awk '{print $NF}' || true)
    SOURCE_HOST=$(grep '^# Hostname:' "$STAGING/MANIFEST.txt" | awk '{print $NF}' || true)
    log "Export from: ${SOURCE_USER:-unknown}@${SOURCE_HOST:-unknown}"
fi
SOURCE_USER="${SOURCE_USER:-yakob}"
SOURCE_HOME="/home/${SOURCE_USER}"
TARGET_HOME="$HOME"

log "Path mapping: $SOURCE_HOME -> $TARGET_HOME"
if [[ "$SOURCE_HOME" == "$TARGET_HOME" ]]; then
    log "  (same paths — no rewriting needed)"
fi
if $DRY_RUN; then
    dry "=== DRY RUN MODE — no changes will be made ==="
fi

# ── 1. Restore ~/.openclaw/ ──────────────────────────────────────────
log "1/7  ~/.openclaw/ config"
if [[ -d "$STAGING/openclaw-home" ]]; then
    mkdir -p "$OPENCLAW_HOME"

    # Critical: identity and credentials first
    for subdir in identity devices credentials agents cron completions canvas; do
        if [[ -d "$STAGING/openclaw-home/$subdir" ]]; then
            restore_dir "$STAGING/openclaw-home/$subdir" "$OPENCLAW_HOME/$subdir" "openclaw/$subdir"
        fi
    done

    # Config files
    for f in openclaw.json update-check.json; do
        if [[ -f "$STAGING/openclaw-home/$f" ]]; then
            restore_file "$STAGING/openclaw-home/$f" "$OPENCLAW_HOME/$f" "openclaw/$f"
        fi
    done

    # Rewrite paths in config
    if [[ -f "$OPENCLAW_HOME/openclaw.json" ]]; then
        rewrite_paths "$OPENCLAW_HOME/openclaw.json" "$SOURCE_HOME" "$TARGET_HOME"
    fi

    # Ensure credentials dir has restrictive permissions
    if [[ -d "$OPENCLAW_HOME/credentials" ]] && ! $DRY_RUN; then
        chmod 700 "$OPENCLAW_HOME/credentials"
    fi
else
    warn "No openclaw-home/ in tarball — skipping ~/.openclaw/ restore"
fi

# ── 2. Restore .yaks task tracker ────────────────────────────────────
log "2/7  .yaks task tracker"
if [[ -d "$STAGING/yaks" ]]; then
    restore_dir "$STAGING/yaks" "$WORKSPACE/.yaks" ".yaks task state"
else
    warn "No yaks/ in tarball — skipping .yaks restore"
fi

# ── 3. Restore workspace files ───────────────────────────────────────
log "3/7  Workspace files"
if [[ -d "$STAGING/workspace" ]]; then
    mkdir -p "$WORKSPACE"

    # Scripts (*.sh)
    for f in "$STAGING/workspace/"*.sh; do
        [[ -f "$f" ]] || continue
        local_name=$(basename "$f")
        restore_file "$f" "$WORKSPACE/$local_name" "workspace/$local_name"
        # Ensure scripts are executable
        if ! $DRY_RUN && [[ -f "$WORKSPACE/$local_name" ]]; then
            chmod +x "$WORKSPACE/$local_name"
        fi
    done

    # Config files
    for f in orchestrator.kdl worker.Dockerfile opencode.json oh-my-opencode.json launch.sh .gitignore; do
        if [[ -e "$STAGING/workspace/$f" ]]; then
            restore_file "$STAGING/workspace/$f" "$WORKSPACE/$f" "workspace/$f"
        fi
    done
    # Ensure launch.sh is executable
    if ! $DRY_RUN && [[ -f "$WORKSPACE/launch.sh" ]]; then
        chmod +x "$WORKSPACE/launch.sh"
    fi

    # Directories
    for d in docs themes .claude .openclaw .worker-costs .sisyphus; do
        if [[ -d "$STAGING/workspace/$d" ]]; then
            restore_dir "$STAGING/workspace/$d" "$WORKSPACE/$d" "workspace/$d"
        fi
    done

    # .opencode (selective: agents, personalities, package.json)
    if [[ -d "$STAGING/workspace/.opencode" ]]; then
        for d in agents personalities; do
            if [[ -d "$STAGING/workspace/.opencode/$d" ]]; then
                restore_dir "$STAGING/workspace/.opencode/$d" "$WORKSPACE/.opencode/$d" "workspace/.opencode/$d"
            fi
        done
        if [[ -f "$STAGING/workspace/.opencode/package.json" ]]; then
            restore_file "$STAGING/workspace/.opencode/package.json" \
                "$WORKSPACE/.opencode/package.json" "workspace/.opencode/package.json"
        fi
    fi

    # Rewrite paths in workspace configs that may reference the old home
    for f in "$WORKSPACE/orchestrator.kdl" "$WORKSPACE/opencode.json" "$WORKSPACE/oh-my-opencode.json"; do
        if [[ -f "$f" ]]; then
            rewrite_paths "$f" "$SOURCE_HOME" "$TARGET_HOME"
        fi
    done

    # Rewrite paths in shell scripts
    for f in "$WORKSPACE/"*.sh; do
        [[ -f "$f" ]] || continue
        rewrite_paths "$f" "$SOURCE_HOME" "$TARGET_HOME"
    done
else
    warn "No workspace/ in tarball — skipping workspace restore"
fi

# Ensure .openclaw/workspace/.yaks symlink exists
OPENCLAW_WS="$WORKSPACE/.openclaw/workspace"
YAKS_LINK="$OPENCLAW_WS/.yaks"
if [[ -d "$OPENCLAW_WS" ]] && [[ -d "$WORKSPACE/.yaks" ]]; then
    if [[ -L "$YAKS_LINK" ]]; then
        log "  .yaks symlink already exists"
    elif [[ ! -e "$YAKS_LINK" ]]; then
        if $DRY_RUN; then
            dry "Would create symlink: $YAKS_LINK -> $WORKSPACE/.yaks"
        else
            ln -s "$WORKSPACE/.yaks" "$YAKS_LINK"
            log "  Created .yaks symlink in openclaw workspace"
        fi
    else
        warn "$YAKS_LINK exists but is not a symlink — not touching it"
    fi
fi

# ── 4. Restore systemd services ──────────────────────────────────────
log "4/7  Systemd services"
if [[ -d "$STAGING/systemd" ]]; then
    NEEDS_DAEMON_RELOAD=false

    for svc_file in "$STAGING/systemd/"*.service; do
        [[ -f "$svc_file" ]] || continue
        svc_name=$(basename "$svc_file")
        dest="/etc/systemd/system/$svc_name"

        if $DRY_RUN; then
            dry "Would install: $dest (requires sudo)"
            if [[ "$SOURCE_HOME" != "$TARGET_HOME" ]]; then
                dry "  Would rewrite paths: $SOURCE_HOME -> $TARGET_HOME"
            fi
        else
            # Copy to a temp file, rewrite paths, then install
            local_tmp=$(mktemp)
            cp "$svc_file" "$local_tmp"
            if [[ "$SOURCE_HOME" != "$TARGET_HOME" ]]; then
                sed -i "s|${SOURCE_HOME}|${TARGET_HOME}|g" "$local_tmp"
            fi
            # Also rewrite User= if source user differs
            if [[ "$SOURCE_USER" != "$(whoami)" ]]; then
                sed -i "s|User=${SOURCE_USER}|User=$(whoami)|g" "$local_tmp"
                sed -i "s|Group=${SOURCE_USER}|Group=$(whoami)|g" "$local_tmp"
            fi

            if [[ -f "$dest" ]] && cmp -s "$local_tmp" "$dest"; then
                log "  Skip (identical): $svc_name"
                ((SKIPPED++))
            else
                sudo cp "$local_tmp" "$dest"
                sudo chmod 644 "$dest"
                NEEDS_DAEMON_RELOAD=true
                ((RESTORED++))
                log "  Installed: $svc_name"
            fi
            rm -f "$local_tmp"
        fi
    done

    # Override directories (contain secrets like API keys)
    for override_dir in "$STAGING/systemd/"*.service.d; do
        [[ -d "$override_dir" ]] || continue
        dir_name=$(basename "$override_dir")
        dest="/etc/systemd/system/$dir_name"

        if $DRY_RUN; then
            dry "Would install override dir: $dest (contains secrets, requires sudo)"
        else
            sudo mkdir -p "$dest"
            for conf_file in "$override_dir/"*.conf; do
                [[ -f "$conf_file" ]] || continue
                conf_name=$(basename "$conf_file")
                local_tmp=$(mktemp)
                cp "$conf_file" "$local_tmp"
                if [[ "$SOURCE_HOME" != "$TARGET_HOME" ]]; then
                    sed -i "s|${SOURCE_HOME}|${TARGET_HOME}|g" "$local_tmp"
                fi
                if [[ "$SOURCE_USER" != "$(whoami)" ]]; then
                    sed -i "s|User=${SOURCE_USER}|User=$(whoami)|g" "$local_tmp"
                fi
                sudo cp "$local_tmp" "$dest/$conf_name"
                sudo chmod 600 "$dest/$conf_name"
                rm -f "$local_tmp"
                NEEDS_DAEMON_RELOAD=true
                ((RESTORED++))
                log "  Installed override: $dir_name/$conf_name"
            done
        fi
    done

    if $NEEDS_DAEMON_RELOAD && ! $DRY_RUN; then
        sudo systemctl daemon-reload
        log "  Ran systemctl daemon-reload"
    fi

    # Show original service state for reference
    if [[ -f "$STAGING/systemd/service-state.txt" ]]; then
        log "  Original service state (from source machine):"
        while IFS= read -r line; do
            [[ "$line" =~ ^#  ]] && continue
            [[ -z "$line" ]] && continue
            log "    $line"
        done < "$STAGING/systemd/service-state.txt"
    fi

    log "  NOTE: Services not auto-started. Review and enable manually:"
    log "    sudo systemctl enable --now openclaw-gateway"
else
    warn "No systemd/ in tarball — skipping systemd restore"
fi

# ── 5. Restore cron jobs ─────────────────────────────────────────────
log "5/7  Cron jobs"
if [[ -d "$STAGING/cron" ]]; then
    # User crontab
    if [[ -f "$STAGING/cron/user-crontab.txt" ]]; then
        # Check if it's just the "no crontab" placeholder
        if grep -q "^# No user crontab configured" "$STAGING/cron/user-crontab.txt"; then
            log "  Source had no user crontab — skipping"
        else
            if $DRY_RUN; then
                dry "Would import user crontab ($(wc -l < "$STAGING/cron/user-crontab.txt") lines)"
                if [[ "$SOURCE_HOME" != "$TARGET_HOME" ]]; then
                    dry "  Would rewrite paths: $SOURCE_HOME -> $TARGET_HOME"
                fi
            else
                # Rewrite paths in crontab
                local_tmp=$(mktemp)
                cp "$STAGING/cron/user-crontab.txt" "$local_tmp"
                if [[ "$SOURCE_HOME" != "$TARGET_HOME" ]]; then
                    sed -i "s|${SOURCE_HOME}|${TARGET_HOME}|g" "$local_tmp"
                fi
                if [[ "$SOURCE_USER" != "$(whoami)" ]]; then
                    sed -i "s|/home/${SOURCE_USER}|${TARGET_HOME}|g" "$local_tmp"
                fi

                # Backup existing crontab
                existing_crontab=$(mktemp)
                crontab -l > "$existing_crontab" 2>/dev/null || true
                if [[ -s "$existing_crontab" ]]; then
                    cp "$existing_crontab" "$HOME/crontab.${BACKUP_SUFFIX}"
                    ((BACKED_UP++))
                    log "  Backed up existing crontab"
                fi
                rm -f "$existing_crontab"

                crontab "$local_tmp"
                rm -f "$local_tmp"
                ((RESTORED++))
                log "  Imported user crontab"
            fi
        fi
    fi

    # /etc/cron.d entries
    for f in "$STAGING/cron/"cron.d-*; do
        [[ -f "$f" ]] || continue
        # Strip the "cron.d-" prefix to get original filename
        orig_name="${f##*cron.d-}"
        dest="/etc/cron.d/$orig_name"

        if $DRY_RUN; then
            dry "Would install: $dest (requires sudo)"
        else
            local_tmp=$(mktemp)
            cp "$f" "$local_tmp"
            if [[ "$SOURCE_HOME" != "$TARGET_HOME" ]]; then
                sed -i "s|${SOURCE_HOME}|${TARGET_HOME}|g" "$local_tmp"
            fi
            if [[ "$SOURCE_USER" != "$(whoami)" ]]; then
                sed -i "s|${SOURCE_USER}|$(whoami)|g" "$local_tmp"
            fi
            sudo cp "$local_tmp" "$dest"
            sudo chmod 644 "$dest"
            rm -f "$local_tmp"
            ((RESTORED++))
            log "  Installed /etc/cron.d/$orig_name"
        fi
    done
else
    warn "No cron/ in tarball — skipping cron restore"
fi

# ── 6. Load Docker images ────────────────────────────────────────────
log "6/7  Docker images"
if [[ -d "$STAGING/docker" ]]; then
    if [[ -f "$STAGING/docker/yak-worker-latest.tar.gz" ]]; then
        # Check if image already exists
        if docker image inspect yak-worker:latest &>/dev/null; then
            log "  yak-worker:latest already exists — skipping load"
            log "  (to replace, remove first: docker rmi yak-worker:latest)"
            ((SKIPPED++))
        else
            if $DRY_RUN; then
                local_size=$(du -sh "$STAGING/docker/yak-worker-latest.tar.gz" | cut -f1)
                dry "Would load yak-worker:latest ($local_size)"
            else
                log "  Loading yak-worker:latest (this may take a moment)..."
                gunzip -c "$STAGING/docker/yak-worker-latest.tar.gz" | docker load
                ((RESTORED++))
                log "  Loaded yak-worker:latest"
            fi
        fi
    else
        warn "No yak-worker image in tarball"
    fi

    # Show image inventory for reference
    if [[ -f "$STAGING/docker/image-list.txt" ]]; then
        log "  Source machine had these images:"
        while IFS= read -r line; do
            log "    $line"
        done < "$STAGING/docker/image-list.txt"
    fi
else
    log "  No docker/ in tarball — skipping Docker restore"
fi

# ── 7. Restore user-level config ─────────────────────────────────────
log "7/7  User-level config"
if [[ -d "$STAGING/user-config" ]]; then
    # Zellij config
    if [[ -d "$STAGING/user-config/zellij" ]]; then
        mkdir -p "$HOME/.config"
        restore_dir "$STAGING/user-config/zellij" "$HOME/.config/zellij" "zellij config"
    fi

    # GitHub CLI config
    if [[ -d "$STAGING/user-config/gh" ]]; then
        mkdir -p "$HOME/.config"
        restore_dir "$STAGING/user-config/gh" "$HOME/.config/gh" "gh CLI config"
        # Fix permissions on gh config (contains tokens)
        if ! $DRY_RUN && [[ -d "$HOME/.config/gh" ]]; then
            chmod 700 "$HOME/.config/gh"
            find "$HOME/.config/gh" -type f -exec chmod 600 {} \;
        fi
    fi

    # OpenCode config (under .config)
    if [[ -d "$STAGING/user-config/opencode" ]]; then
        mkdir -p "$HOME/.config"
        restore_dir "$STAGING/user-config/opencode" "$HOME/.config/opencode" "opencode config"
    fi

    # .claude transcripts
    if [[ -d "$STAGING/user-config/.claude" ]]; then
        restore_dir "$STAGING/user-config/.claude" "$HOME/.claude" "claude transcripts"
    fi

    # Rewrite paths in user configs
    for f in "$HOME/.config/gh/hosts.yml" "$HOME/.config/opencode/config.json"; do
        if [[ -f "$f" ]]; then
            rewrite_paths "$f" "$SOURCE_HOME" "$TARGET_HOME"
        fi
    done
else
    log "  No user-config/ in tarball — skipping user config restore"
fi

# ── Summary ───────────────────────────────────────────────────────────
log ""
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if $DRY_RUN; then
    log "DRY RUN COMPLETE — no changes were made"
else
    log "Import complete!"
    log "  Restored:  $RESTORED items"
    log "  Skipped:   $SKIPPED items (already identical)"
    log "  Backed up: $BACKED_UP items (*.${BACKUP_SUFFIX})"
fi
log ""

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
    log "Warnings (${#WARNINGS[@]}):"
    for w in "${WARNINGS[@]}"; do
        log "  - $w"
    done
    log ""
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    fail "Errors (${#ERRORS[@]}):"
    for e in "${ERRORS[@]}"; do
        fail "  - $e"
    done
    log ""
fi

log "Next steps:"
log "  1. Review systemd overrides (contain API keys):"
log "     sudo cat /etc/systemd/system/openclaw-gateway.service.d/override.conf"
log "  2. Enable and start services:"
log "     sudo systemctl enable --now openclaw-gateway"
log "  3. Start Zellij session:"
log "     cd $WORKSPACE && ./launch.sh"
log "  4. Verify cron jobs:"
log "     crontab -l"
log "     openclaw cron list"
log "  5. Run validation:"
log "     openclaw doctor"
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    exit 1
fi
