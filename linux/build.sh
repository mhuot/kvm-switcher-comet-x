#!/bin/sh
# Builds static Linux binaries, per-user install tarballs, and .deb, .rpm and Arch packages
# for amd64 and arm64 into linux/dist/.
#   usage: linux/build.sh [version]   (default: the version in cmd/kvm-switcher/main.go)
set -eu
cd "$(dirname "$0")"

version="${1:-$(sed -n 's/^var version = "\(.*\)"$/\1/p' cmd/kvm-switcher/main.go)}"
nfpm_module="github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0"
rm -rf dist
mkdir -p dist

for architecture in amd64 arm64; do
    stage_directory="dist/kvm-switcher-${version}-linux-${architecture}"
    mkdir -p "$stage_directory"
    echo "Building ${architecture}"
    CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" go build -trimpath \
        -ldflags "-s -w -X main.version=${version}" -o "$stage_directory/kvm-switcher" ./cmd/kvm-switcher
    cp install.sh README.md ../LICENSE "$stage_directory/"
    cp packaging/net.mhuot.KVMSwitcher.desktop "$stage_directory/"
    tar -C dist -czf "${stage_directory}.tar.gz" "$(basename "$stage_directory")"

    mkdir -p dist/package-binary
    cp "$stage_directory/kvm-switcher" dist/package-binary/kvm-switcher
    for packager in deb rpm archlinux; do
        KVM_SWITCHER_VERSION="$version" KVM_SWITCHER_ARCH="$architecture" \
            go run "$nfpm_module" package --config packaging/nfpm.yaml --packager "$packager" \
            --target dist/ >/dev/null
    done
    rm -rf "$stage_directory" dist/package-binary
done
ls -1 dist
