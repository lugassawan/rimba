#!/bin/sh
set -eu

REPO="lugassawan/rimba"
BINARY="rimba"
INSTALL_DIR="/usr/local/bin"

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
    if [ -w "$INSTALL_DIR" ]; then
        install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        echo "Elevated permissions required. Running with sudo..."
        sudo install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

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

main
