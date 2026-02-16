#!/bin/sh
set -eu

REPO="lugassawan/rimba"
BINARY="rimba"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

main() {
    os=$(detect_os)
    arch=$(detect_arch)

    if [ -n "${VERSION:-}" ]; then
        version="$VERSION"
    else
        version=$(get_latest_version)
    fi

    # Strip leading "v" for the archive name
    version_stripped=$(echo "$version" | sed 's/^v//')

    archive="${BINARY}_${version_stripped}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${version}/${archive}"

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    echo "Downloading ${BINARY} ${version} for ${os}/${arch}..."
    curl -sSfL "$url" -o "${tmpdir}/${archive}"

    echo "Extracting..."
    tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"

    echo "Installing to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

    # Auto PATH setup
    ensure_path

    # Migration: detect old binary at /usr/local/bin
    check_old_binary

    echo "Verifying installation..."
    if command -v "$BINARY" >/dev/null 2>&1; then
        "$BINARY" version
        echo "Successfully installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
    else
        echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
        echo "Note: ${INSTALL_DIR} may not be in your PATH"
    fi
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)
            echo "Error: unsupported operating system: $(uname -s)" >&2
            exit 1
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)
            echo "Error: unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac
}

get_latest_version() {
    version=$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        echo "Error: could not determine latest version" >&2
        exit 1
    fi

    echo "$version"
}

ensure_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return ;; # already in PATH
    esac

    shell_name=$(basename "${SHELL:-}")
    case "$shell_name" in
        zsh)  rc_file="$HOME/.zshrc" ;;
        bash) rc_file="$HOME/.bashrc" ;;
        fish)
            fish -c "fish_add_path $INSTALL_DIR" 2>/dev/null || true
            echo "Added ${INSTALL_DIR} to PATH via fish_add_path."
            return
            ;;
        *)
            echo "Note: Add ${INSTALL_DIR} to your PATH manually."
            return
            ;;
    esac

    export_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
    guard="# Added by rimba"

    # Check if already present (idempotent)
    if [ -f "$rc_file" ] && grep -qF "$INSTALL_DIR" "$rc_file" 2>/dev/null; then
        return
    fi

    printf '\n%s\n%s\n' "$guard" "$export_line" >> "$rc_file"
    echo "Added ${INSTALL_DIR} to PATH in ${rc_file}. Restart your shell or run: source ${rc_file}"
}

check_old_binary() {
    old_path="/usr/local/bin/${BINARY}"
    resolved_install="$(cd "$INSTALL_DIR" && pwd)/${BINARY}"

    if [ -f "$old_path" ] && [ "$old_path" != "$resolved_install" ]; then
        echo ""
        echo "Note: an older copy of ${BINARY} exists at ${old_path}"
        echo "To avoid conflicts, remove it:"
        echo "  sudo rm ${old_path}"
    fi
}

main
