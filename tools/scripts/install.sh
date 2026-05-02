#!/usr/bin/env bash

set -euo pipefail

REPO="yuluo-yx/typo"
BINARY_NAME="typo"
VERSION_SELECTOR=""
BUILD_FROM_SOURCE=0
TMP_DIR=""

usage() {
  cat <<'EOF'
Install typo on macOS or Linux.

Usage:
  install.sh
  install.sh -s 0.2.0
  install.sh -b

Options:
  -s VERSION   Install a Release version (semver, e.g. 0.2.0); `latest` means the newest Release
  -b           Build from the `main` branch source (requires `go`)
  -h           Show help

Environment:
  TYPO_INSTALL_DIR   Override install directory
EOF
}

while getopts ":s:bh" opt; do
  case "$opt" in
    s)
      VERSION_SELECTOR="$OPTARG"
      ;;
    b)
      BUILD_FROM_SOURCE=1
      ;;
    h)
      usage
      exit 0
      ;;
    :)
      echo "Option -$OPTARG requires an argument." >&2
      usage >&2
      exit 1
      ;;
    \?)
      echo "Unknown option: -$OPTARG" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$BUILD_FROM_SOURCE" -eq 1 && -n "$VERSION_SELECTOR" ]]; then
  echo "Options -b and -s cannot be used together." >&2
  usage >&2
  exit 1
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

normalize_arch() {
  local os_name="$1"

  case "$(uname -m)" in
    arm64|aarch64)
      echo "${os_name}-arm64"
      ;;
    x86_64|amd64)
      echo "${os_name}-amd64"
      ;;
    *)
      echo "Unsupported architecture for ${os_name}: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

detect_shell() {
  local shell_name
  shell_name="$(basename "${SHELL:-}")"

  case "$shell_name" in
    zsh)  echo "zsh" ;;
    bash) echo "bash" ;;
    fish) echo "fish" ;;
    *)    echo "" ;;
  esac
}

normalize_tag() {
  local input="$1"

  if [[ -z "$input" || "$input" == "latest" ]]; then
    echo ""
    return
  fi

  if [[ "$input" == v* ]]; then
    echo "$input"
    return
  fi

  echo "v$input"
}

resolve_release_tag() {
  local tag="$1"

  if [[ -n "$tag" ]]; then
    echo "$tag"
    return
  fi

  resolve_latest_release_tag
}

resolve_install_dir() {
  local os_name="$1"

  if [[ -n "${TYPO_INSTALL_DIR:-}" ]]; then
    echo "$TYPO_INSTALL_DIR"
    return
  fi

  local candidate_dirs=()
  case "$os_name" in
    darwin)
      candidate_dirs=(/usr/local/bin /opt/homebrew/bin)
      ;;
    linux)
      candidate_dirs=(/usr/local/bin)
      ;;
    *)
      candidate_dirs=()
      ;;
  esac

  for dir in "${candidate_dirs[@]}"; do
    if [[ -d "$dir" && -w "$dir" ]]; then
      echo "$dir"
      return
    fi
  done

  echo "$HOME/.local/bin"
}

download_release_binary() {
  local platform="$1"
  local tag="$2"
  local output="$3"
  local url=""

  url="https://github.com/${REPO}/releases/download/${tag}/${BINARY_NAME}-${platform}"

  echo "Downloading ${BINARY_NAME} ${tag} from ${url}"
  curl -fsSL "$url" -o "$output"
}

verify_release_checksum() {
  local platform="$1"
  local tag="$2"
  local binary_path="$3"
  local asset="${BINARY_NAME}-${platform}"
  local checksums_path="${TMP_DIR}/checksums.txt"
  local checksums_url="https://github.com/${REPO}/releases/download/${tag}/checksums.txt"

  echo "Downloading checksums.txt from ${checksums_url}"
  local http_code curl_status
  curl_status=0
  http_code="$(curl -fsSL -w "%{http_code}" "$checksums_url" -o "$checksums_path")" || curl_status=$?
  if [[ "$curl_status" -ne 0 ]]; then
    if [[ "$http_code" == "404" ]]; then
      echo "Warning: checksums.txt is not available for ${tag}. Checksum verification will be skipped." >&2
      return
    fi
    echo "Unable to download checksums.txt for ${tag}." >&2
    echo "Refusing to install an unverified binary because the checksum manifest download failed." >&2
    exit 1
  fi

  local checksum_line
  checksum_line="$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+\\*?${asset}$" "$checksums_path" || true)"
  if [[ -z "$checksum_line" ]]; then
    echo "Unable to find checksum entry for ${asset}" >&2
    exit 1
  fi

  local expected_hash actual_hash
  expected_hash="$(printf '%s\n' "$checksum_line" | awk '{print tolower($1)}')"
  if command -v sha256sum >/dev/null 2>&1; then
    actual_hash="$(sha256sum "$binary_path" | awk '{print tolower($1)}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual_hash="$(shasum -a 256 "$binary_path" | awk '{print tolower($1)}')"
  else
    echo "Missing required command: sha256sum or shasum" >&2
    exit 1
  fi

  if [[ "$actual_hash" != "$expected_hash" ]]; then
    echo "Checksum verification failed for ${asset}" >&2
    echo "Expected: ${expected_hash}" >&2
    echo "Actual:   ${actual_hash}" >&2
    exit 1
  fi

  echo "Checksum verified for ${asset}"
}

resolve_latest_release_tag() {
  local api_url="https://api.github.com/repos/${REPO}/releases?per_page=1"
  local tag=""

  tag="$(curl -fsSL -H 'Accept: application/vnd.github+json' "$api_url" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')"

  if [[ -z "$tag" ]]; then
    echo "Unable to resolve the latest release tag from ${api_url}" >&2
    exit 1
  fi

  echo "$tag"
}

build_main_binary() {
  local tmp_dir="$1"
  local output="$2"
  local source_archive="${tmp_dir}/main.tar.gz"
  local source_dir="${tmp_dir}/${BINARY_NAME}-main"

  require_cmd tar
  require_cmd go

  echo "Building ${BINARY_NAME} from the main branch"
  curl -fsSL "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" -o "$source_archive"
  tar -xzf "$source_archive" -C "$tmp_dir"
  (cd "$source_dir" && go build -o "$output" ./cmd/typo)
}

install_binary() {
  local source_file="$1"
  local install_dir="$2"
  local target_file="${install_dir}/${BINARY_NAME}"

  mkdir -p "$install_dir"

  if [[ ! -w "$install_dir" ]]; then
    echo "Install directory is not writable: $install_dir" >&2
    echo "Set TYPO_INSTALL_DIR to a writable path, for example:" >&2
    echo "  TYPO_INSTALL_DIR=\$HOME/.local/bin bash install.sh" >&2
    exit 1
  fi

  install -m 755 "$source_file" "$target_file"
  echo "Installed ${BINARY_NAME} to ${target_file}"

  case ":$PATH:" in
    *":${install_dir}:"*)
      ;;
    *)
      echo "Add ${install_dir} to your PATH if it is not already available."
      ;;
  esac
}

main() {
  local kernel_name
  kernel_name="$(uname -s)"

  local os_name=""
  case "$kernel_name" in
    Darwin)
      os_name="darwin"
      ;;
    Linux)
      os_name="linux"
      ;;
    *)
      echo "This installer currently supports macOS and Linux only." >&2
      exit 1
      ;;
  esac

  require_cmd curl
  require_cmd install

  local platform
  platform="$(normalize_arch "$os_name")"

  local install_dir
  install_dir="$(resolve_install_dir "$os_name")"

  TMP_DIR="$(mktemp -d)"
  trap 'if [[ -n "${TMP_DIR:-}" ]]; then rm -rf "$TMP_DIR"; fi' EXIT

  local binary_path="${TMP_DIR}/${BINARY_NAME}"

  if [[ "$BUILD_FROM_SOURCE" -eq 1 ]]; then
    build_main_binary "$TMP_DIR" "$binary_path"
  else
    local release_tag
    release_tag="$(resolve_release_tag "$(normalize_tag "$VERSION_SELECTOR")")"
    download_release_binary "$platform" "$release_tag" "$binary_path"
    verify_release_checksum "$platform" "$release_tag" "$binary_path"
  fi

  install_binary "$binary_path" "$install_dir"

  # fetch shell
  local shell
  shell="$(detect_shell)"
  if [[ -z "$shell" ]]; then
    echo "Warning: Unrecognised shell '${SHELL:-}'. Add 'eval \"\$(typo init <shell>)\"' to your shell config manually." >&2
  elif [[ "$shell" == "fish" ]]; then
    echo "Please add 'typo init fish | source' to your shell configuration file (e.g., ~/.config/fish/config.fish) to enable shell integration. Not forgotten to source it!"
  else
    echo "Please add 'eval \"\$(typo init ${shell})\"' to your shell configuration file (e.g., ~/.${shell}rc) to enable shell integration. Not forgotten to source it!"
  fi
}

main "$@"
