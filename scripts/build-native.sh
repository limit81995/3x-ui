#!/usr/bin/env bash
# Build this checkout and the release-shaped installation archive without Docker.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
[[ $(uname -s) == Linux ]] || { echo 'Build on Linux with Go, Node.js and a C compiler.' >&2; exit 1; }
for dependency in go npm curl unzip tar sha256sum; do
    command -v "$dependency" >/dev/null || { echo "Missing build dependency: $dependency" >&2; exit 1; }
done
export CC="${XUI_NATIVE_CC:-musl-gcc}"
command -v "$CC" >/dev/null || { echo "Missing $CC; install musl-tools or set XUI_NATIVE_CC to a static musl compiler." >&2; exit 1; }
[[ $(go env GOOS) == linux ]] || { echo "GOOS must be linux for a native installation package." >&2; exit 1; }
platform=$(go env GOARCH)
variant=''
case "$platform" in
    amd64 | arm64 | 386) ;;
    arm)
        variant="v$(go env GOARM | cut -d, -f1)"
        case "$variant" in v6 | v7) platform="arm${variant}" ;; *) echo "Unsupported GOARM: $variant" >&2; exit 1 ;; esac
        ;;
    *) echo "Unsupported native package architecture: $platform" >&2; exit 1 ;;
esac
output=${1:-"$repo_root/dist/native"}
mkdir -p "$output"
output=$(cd "$output" && pwd)
build_dir=$(mktemp -d)
trap 'rm -rf -- "$build_dir"' EXIT
package="$build_dir/x-ui"
mkdir -p "$package"

cd "$repo_root/frontend"
npm ci
npm run build
cd "$repo_root"
ldflags="-w -s -linkmode external -extldflags '-static'"
if command -v git >/dev/null && commit=$(git rev-parse --short=8 HEAD 2>/dev/null); then
    ldflags+=" -X github.com/mhsanaei/3x-ui/v3/internal/config.buildCommit=$commit"
    ldflags+=" -X github.com/mhsanaei/3x-ui/v3/internal/config.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi
CGO_ENABLED=1 go build -trimpath -ldflags "$ldflags" -o "$package/x-ui" .

# DockerInit only downloads runtime assets; it does not invoke Docker.
(cd "$build_dir" && sh "$repo_root/DockerInit.sh" "$(go env GOARCH)" "$variant")
mv "$build_dir/build/bin" "$package/bin"
case "$platform" in
    armv6 | armv7)
        mv "$package/bin/xray-linux-arm32" "$package/bin/xray-linux-$platform"
        mv "$package/bin/mtg-linux-arm" "$package/bin/mtg-linux-$platform"
        ;;
esac
cp x-ui.sh x-ui.rc x-ui.service.debian x-ui.service.arch x-ui.service.rhel "$package/"
mkdir -p "$package/internal/web"
cp -R internal/web/translation "$package/internal/web/"
archive="x-ui-linux-$platform.tar.gz"
tar --owner=0 --group=0 -czf "$output/$archive" -C "$build_dir" x-ui
(cd "$output" && sha256sum "$archive" > "$archive.sha256")
cp install.sh "$output/install.sh"
printf 'Built: %s\nChecksum: %s\nInstaller: %s\n' "$output/$archive" "$output/$archive.sha256" "$output/install.sh"
printf 'Install with root: XUI_INSTALL_ARCHIVE=%q bash %q\n' "$output/$archive" "$output/install.sh"
