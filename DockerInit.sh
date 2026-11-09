#!/bin/sh
set -e

# Every network step has a finite retry and timeout budget so one transient
# GitHub failure does not abort a multi-platform image build.
fetch() {
    curl --fail --silent --show-error --location \
        --retry 5 --retry-all-errors --retry-delay 3 --retry-max-time 600 \
        --connect-timeout 15 --max-time 300 "$@"
}

# $1 is Docker's TARGETARCH and $2 its TARGETVARIANT (v6/v7 on arm). The
# platform names match the release archives and the mtg-multi packages.
case $1 in
    amd64 | "")
        PLATFORM="amd64"
        ;;
    arm64 | armv8 | aarch64)
        PLATFORM="arm64"
        ;;
    arm | arm32 | armv7)
        PLATFORM="armv7"
        if [ "$2" = "v6" ]; then
            PLATFORM="armv6"
        fi
        ;;
    armv6)
        PLATFORM="armv6"
        ;;
    386 | i386)
        PLATFORM="386"
        ;;
    *)
        echo "DockerInit: unsupported architecture '$1'" >&2
        exit 1
        ;;
esac
case $PLATFORM in
    amd64) XRAY_ZIP="64" ;;
    arm64) XRAY_ZIP="arm64-v8a" ;;
    armv7) XRAY_ZIP="arm32-v7a" ;;
    armv6) XRAY_ZIP="arm32-v6" ;;
    386) XRAY_ZIP="32" ;;
esac
# The panel looks binaries up by GOARCH; on arm Xray's is "arm32"
# (internal/xray/process.go) and mtg's plain "arm", as install.sh renames them.
case $PLATFORM in
    armv7 | armv6)
        XRAY_BIN="xray-linux-arm32"
        MTG_BIN="mtg-linux-arm"
        ;;
    *)
        XRAY_BIN="xray-linux-${PLATFORM}"
        MTG_BIN="mtg-linux-${PLATFORM}"
        ;;
esac
# Resolve the public release redirect instead of spending an unauthenticated
# GitHub API request for every build platform.
MTG_RELEASE_URL=$(fetch --head --output /dev/null --write-out '%{url_effective}' \
    "https://github.com/mhsanaei/mtg-multi/releases/latest")
case "$MTG_RELEASE_URL" in
    https://github.com/mhsanaei/mtg-multi/releases/tag/*)
        MTG_MULTI_VER=${MTG_RELEASE_URL##*/}
        ;;
    *) MTG_MULTI_VER="" ;;
esac
case "$MTG_MULTI_VER" in
    "" | *[!A-Za-z0-9._-]*)
        echo "DockerInit: could not resolve the latest mtg-multi release tag" >&2
        exit 1
        ;;
esac
mkdir -p build/bin
cd build/bin
fetch -RO "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-${XRAY_ZIP}.zip"
unzip "Xray-linux-${XRAY_ZIP}.zip"
rm -f "Xray-linux-${XRAY_ZIP}.zip" geoip.dat geosite.dat
mv xray "${XRAY_BIN}"
# mtg-multi (MTProto sidecar) ships prebuilt release binaries for every target
# we package, so download and unpack the matching one instead of compiling.
MTG_PKG="mtg-multi-${MTG_MULTI_VER#v}-linux-${PLATFORM}"
fetch -RO "https://github.com/mhsanaei/mtg-multi/releases/download/${MTG_MULTI_VER}/${MTG_PKG}.tar.gz"
tar -xzf "${MTG_PKG}.tar.gz"
mv "${MTG_PKG}/mtg-multi" "${MTG_BIN}"
rm -rf "${MTG_PKG}" "${MTG_PKG}.tar.gz"
chmod +x "${MTG_BIN}"
fetch -RO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
fetch -RO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
fetch -Ro geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
fetch -Ro geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
fetch -Ro geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
fetch -Ro geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
