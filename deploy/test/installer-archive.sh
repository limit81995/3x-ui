#!/usr/bin/env bash
# Real archives/checksums only; never run the installer's host-mutating entry point.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
installer=${INSTALLER_UNDER_TEST:-"$repo_root/install.sh"}
test_dir=$(mktemp -d)
trap 'rm -rf -- "$test_dir"' EXIT
load_function() {
    local definition
    definition=$(awk -v name="$1" '$0 == name "() {" {copy=1} copy {print} copy && /^}$/ {exit}' "$installer")
    [[ -n "$definition" ]] || { echo "Missing installer function: $1" >&2; return 1; }
    eval "$definition"
}
load_function prepare_local_archive
arch() { echo amd64; }
export red='' green='' yellow='' plain='' release=debian
mkdir -p "$test_dir/package/x-ui/bin" "$test_dir/install"
printf 'native-panel\n' > "$test_dir/package/x-ui/x-ui"
printf 'native-cli\n' > "$test_dir/package/x-ui/x-ui.sh"
printf 'native-core\n' > "$test_dir/package/x-ui/bin/xray-linux-amd64"
printf 'native-service\n' > "$test_dir/package/x-ui/x-ui.service.debian"
printf 'native-openrc\n' > "$test_dir/package/x-ui/x-ui.rc"
archive="$test_dir/local build.tar.gz"
xui_folder="$test_dir/install/x-ui"
pack() {
    tar -czf "$archive" -C "$test_dir/package" x-ui
    sha256sum "$archive" > "$archive.sha256"
}
assert_rejected() {
    local reason=$1
    printf 'existing-install-cache\n' > "${xui_folder}-linux-amd64.tar.gz"
    if prepare_local_archive "$archive" > "$test_dir/output" 2>&1; then
        echo "Accepted $reason" >&2; exit 1
    fi
    [[ $(cat "${xui_folder}-linux-amd64.tar.gz") == existing-install-cache ]]
    printf 'PASS rejects %s before overwriting files\n' "$reason"
}
pack
prepare_local_archive "$archive"
cmp "$archive" "${xui_folder}-linux-amd64.tar.gz"
# An archive already at the installer's cache path must also work.
cp "$archive.sha256" "${xui_folder}-linux-amd64.tar.gz.sha256"
prepare_local_archive "${xui_folder}-linux-amd64.tar.gz"
printf 'PASS verified local archive, spaces in path and in-place cache\n'
printf 'corruption' >> "$archive"
assert_rejected 'checksum mismatch'
pack
rm "$archive.sha256"
assert_rejected 'missing checksum'
pack
printf 'not-a-hash\n' > "$archive.sha256"
assert_rejected 'malformed checksum'
for member in x-ui x-ui.sh bin/xray-linux-amd64 x-ui.service.debian; do
    mv "$test_dir/package/x-ui/$member" "$test_dir/held"
    pack
    assert_rejected "missing $member"
    mv "$test_dir/held" "$test_dir/package/x-ui/$member"
done
release=alpine
mv "$test_dir/package/x-ui/x-ui.rc" "$test_dir/held"
pack
assert_rejected 'missing OpenRC script'
mv "$test_dir/held" "$test_dir/package/x-ui/x-ui.rc"
pack
prepare_local_archive "$archive"
printf 'PASS Alpine archive\n'
release=debian
printf 'unexpected' > "$test_dir/package/outside"
tar -czf "$archive" -C "$test_dir/package" x-ui outside
sha256sum "$archive" > "$archive.sha256"
assert_rejected 'unexpected archive root'
# Reject traversal and symlinks before extraction into /usr/local.
tar -czf "$archive" -C "$test_dir/package" --transform='s|outside|x-ui/../outside|' x-ui outside
sha256sum "$archive" > "$archive.sha256"
assert_rejected 'parent path traversal'
ln -s /etc "$test_dir/package/x-ui/escape"
pack
assert_rejected 'symbolic link'
printf 'All local archive tests passed\n'
