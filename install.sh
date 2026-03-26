#!/usr/bin/env bash

set -euo pipefail

REPO="yuluo-yx/typo"
BINARY_NAME="typo"
VERSION_SELECTOR=""
TMP_DIR=""

usage() {
  cat <<'EOF'
Install typo on macOS or Linux.

Usage:
  install.sh
  install.sh -s latest
  install.sh -s 26.03.24

Options:
  -s VERSION   latest 表示构建 main 分支代码；其余值按 Release 版本安装
  -h           Show help

Environment:
  TYPO_INSTALL_DIR   Override install directory
EOF
}

while getopts ":s:h" opt; do
  case "$opt" in
    s)
      VERSION_SELECTOR="$OPTARG"
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

normalize_tag() {
  local input="$1"

  if [[ -z "$input" ]]; then
    echo ""
    return
  fi

  if [[ "$input" == v* ]]; then
    echo "$input"
    return
  fi

  echo "v$input"
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

  if [[ -z "$tag" ]]; then
    url="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}-${platform}"
  else
    url="https://github.com/${REPO}/releases/download/${tag}/${BINARY_NAME}-${platform}"
  fi

  echo "Downloading ${BINARY_NAME} from ${url}"
  curl -fsSL "$url" -o "$output"
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

  if [[ "$VERSION_SELECTOR" == "latest" ]]; then
    build_main_binary "$TMP_DIR" "$binary_path"
  else
    download_release_binary "$platform" "$(normalize_tag "$VERSION_SELECTOR")" "$binary_path"
  fi

  install_binary "$binary_path" "$install_dir"
  echo "Run: eval \"\$(typo init zsh)\""
}

main "$@"
