#!/usr/bin/env bash
set -euo pipefail

# Exercises the public installer against a local release fixture without touching the host.
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/is-it-up-install.XXXXXX")
trap 'rm -rf "$TEST_ROOT"' EXIT

FIXTURE="$TEST_ROOT/release"
PAYLOAD="$TEST_ROOT/payload"
mkdir -p "$FIXTURE" "$PAYLOAD/packaging/systemd"

printf '#!/bin/sh\necho is-it-up test-fixture\n' >"$PAYLOAD/is-it-up"
chmod 0755 "$PAYLOAD/is-it-up"
printf 'checks:\n  - name: example\n    type: tcp\n    address: example.com:443\n' >"$PAYLOAD/config.example.yaml"
printf '[Unit]\nDescription=is-it-up fixture\n[Service]\nExecStart=@BINDIR@/is-it-up --config @CONFIG@ --fail-on-unhealthy\n' >"$PAYLOAD/packaging/systemd/is-it-up.service"
printf '[Unit]\nDescription=is-it-up fixture timer\n[Timer]\nOnUnitActiveSec=@INTERVAL@\n' >"$PAYLOAD/packaging/systemd/is-it-up.timer"

tar -C "$PAYLOAD" -czf "$FIXTURE/is-it-up_linux_amd64.tar.gz" .
(
  cd "$FIXTURE"
  shasum -a 256 is-it-up_linux_amd64.tar.gz >SHA256SUMS
)

INSTALL_ROOT="$TEST_ROOT/root"
REAL_CURL=$(command -v curl)
FAKE_BIN="$TEST_ROOT/fake-bin"
mkdir -p "$FAKE_BIN"
cat >"$FAKE_BIN/curl" <<EOF
#!/bin/sh
case " \$* " in
  *audit-installer-token*) echo "token appeared in curl argv" >&2; exit 90 ;;
esac
if env | grep -Fq audit-installer-token; then
  echo "token appeared in curl environment" >&2
  exit 91
fi
exec "$REAL_CURL" "\$@"
EOF
chmod 0755 "$FAKE_BIN/curl"
PATH="$FAKE_BIN:$PATH" \
GH_TOKEN=audit-installer-token \
IS_IT_UP_OS=linux \
IS_IT_UP_ARCH=amd64 \
IS_IT_UP_DOWNLOAD_BASE_URL="file://$FIXTURE" \
  /bin/bash "$ROOT_DIR/scripts/install.sh" \
    --root "$INSTALL_ROOT" \
    --service \
    --skip-systemctl \
    --interval 2min

INSTALLED="$INSTALL_ROOT/usr/local/bin/is-it-up"
test -x "$INSTALLED"
test "$($INSTALLED)" = "is-it-up test-fixture"
test -f "$INSTALL_ROOT/etc/is-it-up/config.yaml"
test -f "$INSTALL_ROOT/etc/systemd/system/is-it-up.service"
test -f "$INSTALL_ROOT/etc/systemd/system/is-it-up.timer"
grep -Fq "$INSTALL_ROOT/usr/local/bin/is-it-up" "$INSTALL_ROOT/etc/systemd/system/is-it-up.service"
grep -Fq "$INSTALL_ROOT/etc/is-it-up/config.yaml" "$INSTALL_ROOT/etc/systemd/system/is-it-up.service"
grep -Fq 'OnUnitActiveSec=2min' "$INSTALL_ROOT/etc/systemd/system/is-it-up.timer"

if IS_IT_UP_OS=linux IS_IT_UP_ARCH=amd64 /bin/bash "$ROOT_DIR/scripts/install.sh" --version not-a-version >/dev/null 2>&1; then
  echo "installer accepted an invalid version" >&2
  exit 1
fi
if IS_IT_UP_OS=linux IS_IT_UP_ARCH=amd64 /bin/bash "$ROOT_DIR/scripts/install.sh" --install-dir relative/path >/dev/null 2>&1; then
  echo "installer accepted a relative install directory" >&2
  exit 1
fi

SYMLINK_FIXTURE="$TEST_ROOT/symlink-release"
SYMLINK_PAYLOAD="$TEST_ROOT/symlink-payload"
mkdir -p "$SYMLINK_FIXTURE" "$SYMLINK_PAYLOAD/packaging/systemd"
ln -s /bin/sh "$SYMLINK_PAYLOAD/is-it-up"
cp "$PAYLOAD/config.example.yaml" "$SYMLINK_PAYLOAD/config.example.yaml"
cp "$PAYLOAD/packaging/systemd/"* "$SYMLINK_PAYLOAD/packaging/systemd/"
tar -C "$SYMLINK_PAYLOAD" -czf "$SYMLINK_FIXTURE/is-it-up_linux_amd64.tar.gz" .
(
  cd "$SYMLINK_FIXTURE"
  shasum -a 256 is-it-up_linux_amd64.tar.gz >SHA256SUMS
)
if IS_IT_UP_OS=linux IS_IT_UP_ARCH=amd64 IS_IT_UP_DOWNLOAD_BASE_URL="file://$SYMLINK_FIXTURE" \
  /bin/bash "$ROOT_DIR/scripts/install.sh" --root "$TEST_ROOT/symlink-root" >/dev/null 2>&1; then
  echo "installer accepted a symbolic-link payload" >&2
  exit 1
fi

printf 'installer integration: passed\n'
