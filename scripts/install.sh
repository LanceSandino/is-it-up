#!/usr/bin/env bash
set -euo pipefail

# Installs the latest or a pinned Linux release, with optional systemd scheduling.
REPOSITORY=${IS_IT_UP_REPO:-LanceSandino/is-it-up}
OS_NAME=${IS_IT_UP_OS:-$(uname -s | tr '[:upper:]' '[:lower:]')}
ARCH_NAME=${IS_IT_UP_ARCH:-$(uname -m)}
VERSION="latest"
ROOT_PREFIX=""
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/is-it-up"
SYSTEMD_DIR="/etc/systemd/system"
INTERVAL="1min"
INSTALL_SERVICE=false
SKIP_SYSTEMCTL=false

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --version VERSION       Install a specific semantic version (default: latest)
  --install-dir PATH      Binary directory (default: /usr/local/bin)
  --service               Install and enable a systemd timer
  --interval DURATION     Timer interval such as 30s, 5min, 1h (default: 1min)
  --root PATH             Stage absolute paths under PATH (testing/packaging)
  --skip-systemctl        Write service files without invoking systemctl
  -h, --help              Show this help

Set GH_TOKEN for private GitHub repositories. Set IS_IT_UP_DOWNLOAD_BASE_URL
to an alternate directory or URL containing the release assets.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      VERSION=${2:?--version requires a value}
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR=${2:?--install-dir requires a value}
      shift 2
      ;;
    --service)
      INSTALL_SERVICE=true
      shift
      ;;
    --interval)
      INTERVAL=${2:?--interval requires a value}
      shift 2
      ;;
    --root)
      ROOT_PREFIX=${2:?--root requires a value}
      shift 2
      ;;
    --skip-systemctl)
      SKIP_SYSTEMCTL=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$OS_NAME" in
  linux) ;;
  *) echo "unsupported operating system: $OS_NAME (Linux only)" >&2; exit 2 ;;
esac

case "$ARCH_NAME" in
  x86_64|amd64) ARCH_NAME=amd64 ;;
  aarch64|arm64) ARCH_NAME=arm64 ;;
  *) echo "unsupported architecture: $ARCH_NAME" >&2; exit 2 ;;
esac

if [[ ! "$INTERVAL" =~ ^[1-9][0-9]*(s|min|h|d)$ ]]; then
  echo "invalid interval: $INTERVAL" >&2
  exit 2
fi
if [ "$VERSION" != "latest" ]; then
  VERSION=${VERSION#v}
  if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]]; then
    echo "invalid semantic version: $VERSION" >&2
    exit 2
  fi
fi
if [[ "$INSTALL_DIR" != /* ]]; then
  echo "--install-dir must be an absolute path" >&2
  exit 2
fi
if [ -n "$ROOT_PREFIX" ] && [[ "$ROOT_PREFIX" != /* ]]; then
  echo "--root must be an absolute path" >&2
  exit 2
fi
if $INSTALL_SERVICE && [ -z "$ROOT_PREFIX" ] && [ "$(id -u)" -ne 0 ]; then
  echo "--service requires root; rerun with sudo" >&2
  exit 1
fi

rooted() {
  if [ -n "$ROOT_PREFIX" ] && [[ "$1" == /* ]]; then
    printf '%s%s' "$ROOT_PREFIX" "$1"
  else
    printf '%s' "$1"
  fi
}

INSTALL_DIR=$(rooted "$INSTALL_DIR")
CONFIG_DIR=$(rooted "$CONFIG_DIR")
SYSTEMD_DIR=$(rooted "$SYSTEMD_DIR")
asset="is-it-up_linux_${ARCH_NAME}.tar.gz"

if [ -n "${IS_IT_UP_DOWNLOAD_BASE_URL:-}" ]; then
  base=${IS_IT_UP_DOWNLOAD_BASE_URL%/}
elif [ "$VERSION" = "latest" ]; then
  base="https://github.com/$REPOSITORY/releases/latest/download"
else
  base="https://github.com/$REPOSITORY/releases/download/v$VERSION"
fi

curl_args=(-fsSL --retry 3 --retry-delay 1)
if [ -n "${GH_TOKEN:-}" ]; then
	if [[ ! "$GH_TOKEN" =~ ^[A-Za-z0-9_.-]+$ ]]; then
		echo "GH_TOKEN contains unsupported characters" >&2
		exit 2
	fi
fi

download() {
	if [ -n "${GH_TOKEN:-}" ]; then
		printf 'header = "Authorization: Bearer %s"\n' "$GH_TOKEN" | env -u GH_TOKEN curl --config - "${curl_args[@]}" "$@"
	else
		curl "${curl_args[@]}" "$@"
	fi
}

work=$(mktemp -d "${TMPDIR:-/tmp}/is-it-up-install.XXXXXX")
trap 'rm -rf "$work"' EXIT
download "$base/$asset" -o "$work/$asset"
download "$base/SHA256SUMS" -o "$work/SHA256SUMS"

expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$work/SHA256SUMS")
if [ -z "$expected" ]; then
  echo "checksum entry missing for $asset" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$work/$asset" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$work/$asset" | awk '{print $1}')
fi
if [ "$actual" != "$expected" ]; then
  echo "checksum verification failed for $asset" >&2
  exit 1
fi

mkdir -p "$work/unpack"
while IFS= read -r member; do
  member=${member#./}
  case "$member" in
    ""|.)
      continue
      ;;
    /*|../*|*/../*|*/..)
      echo "unsafe archive member: $member" >&2
      exit 1
      ;;
  esac
done < <(tar -tzf "$work/$asset")
tar -xzf "$work/$asset" -C "$work/unpack"
test -x "$work/unpack/is-it-up"
if find "$work/unpack" -type l -print -quit | grep -q .; then
  echo "release archive must not contain symbolic links" >&2
  exit 1
fi
install -d -m 0755 "$INSTALL_DIR"
install -m 0755 "$work/unpack/is-it-up" "$INSTALL_DIR/is-it-up"

if $INSTALL_SERVICE; then
  if [ -z "$ROOT_PREFIX" ]; then
    if ! getent group is-it-up >/dev/null 2>&1; then
      groupadd --system is-it-up
    fi
    if ! id is-it-up >/dev/null 2>&1; then
      useradd --system --gid is-it-up --home-dir /nonexistent --shell /usr/sbin/nologin is-it-up
    fi
  fi

  install -d -m 0750 "$CONFIG_DIR"
  if [ -z "$ROOT_PREFIX" ]; then
    chown root:is-it-up "$CONFIG_DIR"
  fi
  if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    install -m 0640 "$work/unpack/config.example.yaml" "$CONFIG_DIR/config.yaml"
    if [ -z "$ROOT_PREFIX" ]; then
      chown root:is-it-up "$CONFIG_DIR/config.yaml"
    fi
  fi
  install -d -m 0755 "$SYSTEMD_DIR"

  escaped_install=${INSTALL_DIR//&/\\&}
  escaped_config=${CONFIG_DIR//&/\\&}
  escaped_interval=${INTERVAL//&/\\&}
  sed -e "s|@BINDIR@|$escaped_install|g" -e "s|@CONFIG@|$escaped_config/config.yaml|g" \
    "$work/unpack/packaging/systemd/is-it-up.service" >"$work/is-it-up.service"
  sed -e "s|@INTERVAL@|$escaped_interval|g" \
    "$work/unpack/packaging/systemd/is-it-up.timer" >"$work/is-it-up.timer"
  install -m 0644 "$work/is-it-up.service" "$SYSTEMD_DIR/is-it-up.service"
  install -m 0644 "$work/is-it-up.timer" "$SYSTEMD_DIR/is-it-up.timer"

  if ! $SKIP_SYSTEMCTL; then
    systemctl daemon-reload
    systemctl enable --now is-it-up.timer
  fi
fi

echo "installed $INSTALL_DIR/is-it-up"
if $INSTALL_SERVICE; then
  echo "installed systemd timer with interval $INTERVAL"
fi
