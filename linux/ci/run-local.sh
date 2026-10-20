#!/usr/bin/env bash
# Builds the Linux packages, then runs linux/ci/distro-test.sh in Docker for every
# distribution CI covers.
#   usage: linux/ci/run-local.sh [image ...]
set -u
cd "$(dirname "$0")/../.." || exit 1

if (( $# > 0 )); then
    distro_images=("$@")
else
    distro_images=(ubuntu:22.04 ubuntu:24.04 debian:12 debian:13 fedora:latest
        archlinux:latest opensuse/tumbleweed:latest)
fi

./linux/build.sh >/dev/null || exit 1

failed_images=()
for distro_image in "${distro_images[@]}"; do
    # Arch publishes amd64 images only. IPC_LOCK lets gnome-keyring-daemon lock its memory.
    platform_arguments=()
    [[ "$distro_image" == archlinux* ]] && platform_arguments=(--platform linux/amd64)
    # The ${array[@]+...} form keeps empty arrays safe under set -u in macOS's bash 3.2.
    if ! docker run --rm --cap-add IPC_LOCK ${platform_arguments[@]+"${platform_arguments[@]}"} \
        -v "$PWD":/repo:ro "$distro_image" sh /repo/linux/ci/distro-test.sh /repo/linux/dist; then
        failed_images+=("$distro_image")
    fi
done

if (( ${#failed_images[@]} > 0 )); then
    echo "FAILED: ${failed_images[*]}"
    exit 1
fi
echo "All distributions passed."
