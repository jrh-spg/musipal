#!/usr/bin/env bash
# Packages the prebuilt Go binary into an .rpm or .deb inside a distro
# container. Invoked by .github/workflows/package.yml via `docker run`.
set -euo pipefail

: "${VERSION:?VERSION env var required}"
: "${TARGET:?TARGET env var required}"
: "${PKG_TYPE:?PKG_TYPE env var required}"

cd /work

mkdir -p pkgroot/usr/bin dist-pkg
install -m0755 dist/musipal pkgroot/usr/bin/musipal

case "$PKG_TYPE" in
  rpm)
    dnf install -y rpm-build ruby rubygems ruby-devel gcc make
    ;;
  deb)
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq dpkg-dev ruby ruby-dev build-essential
    ;;
  *)
    echo "Unknown PKG_TYPE: $PKG_TYPE" >&2
    exit 1
    ;;
esac

gem install --no-document fpm

common_args=(
  -s dir -n musipal -v "$VERSION"
  --license MIT
  --description "CLI/TUI music player"
  --url "https://github.com/jake/musipal"
)

if [[ "$PKG_TYPE" == "rpm" ]]; then
  : "${RPM_DIST:?RPM_DIST env var required for rpm packaging}"
  fpm "${common_args[@]}" -t rpm --architecture x86_64 --rpm-dist "$RPM_DIST" \
    -p "dist-pkg/musipal-${VERSION}.${RPM_DIST}.x86_64.rpm" \
    -C pkgroot usr/bin/musipal
else
  fpm "${common_args[@]}" -t deb --architecture amd64 \
    -p "dist-pkg/musipal_${VERSION}_${TARGET}_amd64.deb" \
    -C pkgroot usr/bin/musipal
fi
