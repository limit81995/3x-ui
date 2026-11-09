#!/usr/bin/env bash
# Load only source selection and download helpers; never run installation entry points.
# Invalid configuration cases deliberately use subshells to contain exit and env changes.
# shellcheck disable=SC2030,SC2031
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/../.." && pwd)
test_dir=$(mktemp -d)
server_pid=''
trap '[[ -z "$server_pid" ]] || kill "$server_pid" 2>/dev/null || true; rm -rf -- "$test_dir"' EXIT

load_source() {
    local definition
    definition=$(awk '/^# Keep installers,/ {copy=1} copy && /^$/ {exit} copy {print}' "$repo_root/$1")
    [[ -n "$definition" ]]
    eval "$definition"
}
load_function() {
    local definition
    definition=$(awk -v name="$1" '$0 == name "() {" {copy=1} copy {print} copy && /^}$/ {exit}' "$repo_root/x-ui.sh")
    [[ -n "$definition" ]]
    eval "$definition"
}
for script in install.sh update.sh x-ui.sh; do
    (
        unset XUI_RELEASE_REPO XUI_RELEASE_REF
        load_source "$script"
        [[ "$XUI_RELEASE_REPO" == limit81995/3x-ui && "$XUI_RELEASE_REF" == main ]]
        bash -c '[[ "$XUI_RELEASE_REPO" == limit81995/3x-ui && "$XUI_RELEASE_REF" == main ]]'
    )
    if (export XUI_RELEASE_REPO='owner/repo?escape'; load_source "$script") > /dev/null 2>&1; then
        echo "Accepted invalid repository in $script" >&2; exit 1
    fi
    if (export XUI_RELEASE_REF='../main'; load_source "$script") > /dev/null 2>&1; then
        echo "Accepted invalid script ref in $script" >&2; exit 1
    fi
done

mkdir -p "$test_dir/www/limit81995/3x-ui/main" "$test_dir/www/limit81995/3x-ui/v3.7.0" "$test_dir/www/example/panel/release/test"
for location in limit81995/3x-ui/main example/panel/release/test; do
    for script in install.sh update.sh; do
        cat > "$test_dir/www/$location/$script" <<'SCRIPT'
#!/usr/bin/env bash
printf '%s\n' "$XUI_RELEASE_REPO" "$XUI_RELEASE_REF" "${XUI_UPDATE_TAG:-stable}" "$@"
SCRIPT
    done
done
printf 'menu\n' > "$test_dir/www/limit81995/3x-ui/v3.7.0/x-ui.sh"
touch "$test_dir/www/limit81995/3x-ui/main/empty.sh"
python3 - "$test_dir" <<'PY' > "$test_dir/http.log" 2>&1 &
import functools, http.server, pathlib, sys
root = pathlib.Path(sys.argv[1])
handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(root / 'www'))
server = http.server.HTTPServer(('127.0.0.1', 0), handler)
(root / 'port').write_text(str(server.server_port))
server.serve_forever()
PY
server_pid=$!
for _ in {1..100}; do [[ -s "$test_dir/port" ]] && break; sleep 0.05; done
test_port=$(cat "$test_dir/port")
# Route real curl requests to the local fixture server while recording the original URL.
curl() {
    local arg args=()
    for arg in "$@"; do
        if [[ "$arg" == https://raw.githubusercontent.com/* ]]; then
            printf '%s\n' "$arg" >> "$test_dir/requests"
            arg="http://127.0.0.1:$test_port/${arg#https://raw.githubusercontent.com/}"
        fi
        args+=("$arg")
    done
    command curl --noproxy '*' "${args[@]}"
}
LOGE() { echo "$*" >&2; }
LOGI() { :; }
confirm() { return 0; }
before_show_menu() { :; }
start() { :; }
load_function run_repo_script
load_function installed_script_url
load_function install
load_function update
load_function update_dev
unset XUI_RELEASE_REPO XUI_RELEASE_REF XUI_UPDATE_TAG
load_source x-ui.sh
run_repo_script install.sh v3.7.0 > "$test_dir/actual"
printf '%s\n' limit81995/3x-ui main stable v3.7.0 > "$test_dir/expected"
diff -u "$test_dir/expected" "$test_dir/actual"
XUI_UPDATE_TAG=dev-latest run_repo_script update.sh > "$test_dir/actual"
printf '%s\n' limit81995/3x-ui main dev-latest > "$test_dir/expected"
diff -u "$test_dir/expected" "$test_dir/actual"
update_dev 0 > "$test_dir/actual"
diff -u "$test_dir/expected" "$test_dir/actual"
printf '%s\n' limit81995/3x-ui main stable > "$test_dir/expected"
update 0 > "$test_dir/actual"
diff -u "$test_dir/expected" "$test_dir/actual"
install 0 > "$test_dir/actual"
diff -u "$test_dir/expected" "$test_dir/actual"
if run_repo_script missing.sh > /dev/null 2>&1 || run_repo_script empty.sh > /dev/null 2>&1; then
    echo 'Missing/empty scripts must fail before execution' >&2; exit 1
fi
for entry in install update update_dev; do
    if (export XUI_RELEASE_REF=missing; "$entry" 0) > /dev/null 2>&1; then
        echo "$entry hid a download failure" >&2; exit 1
    fi
done
export XUI_RELEASE_REPO=example/panel XUI_RELEASE_REF=release/test
run_repo_script update.sh > "$test_dir/actual"
printf '%s\n' example/panel release/test stable > "$test_dir/expected"
diff -u "$test_dir/expected" "$test_dir/actual"

export XUI_RELEASE_REPO=limit81995/3x-ui XUI_RELEASE_REF=main
export yellow='' plain=''
xui_folder="$test_dir/panel"
mkdir -p "$xui_folder"
cat > "$xui_folder/x-ui" <<'SCRIPT'
#!/usr/bin/env bash
cat "$(dirname "$0")/version"
SCRIPT
chmod +x "$xui_folder/x-ui"
echo 3.7.0 > "$xui_folder/version"
[[ $(installed_script_url) == https://raw.githubusercontent.com/limit81995/3x-ui/v3.7.0/x-ui.sh ]]
echo dev+12345678 > "$xui_folder/version"
[[ $(installed_script_url) == https://raw.githubusercontent.com/limit81995/3x-ui/main/x-ui.sh ]]
if grep -Eiq '(raw.githubusercontent.com|github.com/repos|github.com)/mhsanaei/3x-ui' "$test_dir/requests"; then
    echo 'A download escaped to upstream' >&2; exit 1
fi
echo 'PASS fork defaults, child environment, stable/dev installs, script failures and menu updates'
