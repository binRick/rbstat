#!/usr/bin/env bash
# Build el9 and el10 RPMs for rbstat from the prebuilt static binaries, using
# Rocky Linux containers (so it runs anywhere docker is available). Produces
# x86_64 and aarch64 RPMs for each EL release into ./dist.
#
#   scripts/build-rpms.sh [VERSION]   (default VERSION=0.1.0)
set -euo pipefail

VER="${1:-0.1.0}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
SPEC="$ROOT/packaging/rbstat.spec"

# EL release -> builder image. Rocky Linux 10 is not on Docker Hub yet, so el10
# uses AlmaLinux 10 (also EL10); the static binary makes the build distro
# irrelevant to the payload — only the explicit dist tag matters.
declare -A IMAGES=([el9]="rockylinux:9" [el10]="almalinux:10")
# RPM arch -> static binary GOARCH suffix.
declare -A ARCHES=([x86_64]="amd64" [aarch64]="arm64")

for el in el9 el10; do
  img="${IMAGES[$el]}"
  for rpmarch in x86_64 aarch64; do
    goarch="${ARCHES[$rpmarch]}"
    bin="$DIST/rbstat-${VER}-linux-${goarch}"
    [ -f "$bin" ] || { echo "missing binary: $bin" >&2; exit 1; }
    echo ">>> building rbstat-${VER}-1.${el}.${rpmarch}.rpm"
    docker run --rm \
      -v "$bin:/work/rbstat:ro" \
      -v "$SPEC:/work/rbstat.spec:ro" \
      -v "$DIST:/out" \
      "$img" bash -euc "
        ( command -v rpmbuild >/dev/null || (dnf -y install rpm-build >/dev/null 2>&1) ) ;
        mkdir -p /root/rpmbuild/{SOURCES,SPECS} ;
        cp /work/rbstat /root/rpmbuild/SOURCES/rbstat ;
        cp /work/rbstat.spec /root/rpmbuild/SPECS/ ;
        rpmbuild -bb \
          --define '_rbstat_version ${VER}' \
          --define 'dist .${el}' \
          --target ${rpmarch} \
          /root/rpmbuild/SPECS/rbstat.spec ;
        cp /root/rpmbuild/RPMS/${rpmarch}/rbstat-${VER}-1.${el}.${rpmarch}.rpm /out/
      "
  done
done

echo ">>> built RPMs:"
ls -1 "$DIST"/*.rpm
