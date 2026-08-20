#!/usr/bin/env bash

# Copyright (c) 2024-2026 chaunsin
# SPDX-License-Identifier: MIT

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="$SCRIPT_DIR/../qinglong_ncmctl_install.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ncmctl_install_test.XXXXXX")"
ORIGINAL_PATH="$PATH"
FAKE_BIN="$TEST_ROOT/bin"
FIXTURE_DIR="$TEST_ROOT/fixture"
PASS_COUNT=0

cleanup_test() {
    rm -rf -- "$TEST_ROOT"
}
trap cleanup_test EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

assert_equal() {
    local expected="$1"
    local actual="$2"

    [[ "$actual" == "$expected" ]] || fail "expected <$expected>, got <$actual>"
}

assert_contains() {
    local needle="$1"
    local file="$2"

    grep -F -- "$needle" "$file" >/dev/null || fail "$file does not contain: $needle"
}

assert_not_contains() {
    local needle="$1"
    local file="$2"

    if grep -F -- "$needle" "$file" >/dev/null; then
        fail "$file unexpectedly contains: $needle"
    fi
}

assert_no_temporary_files() {
    local case_dir="$1"
    local temporary

    temporary="$(find "$case_dir/install" -maxdepth 1 -name '.ncmctl.*' -print -quit)"
    [[ -z "$temporary" ]] || fail "installation staging file was not cleaned: $temporary"
    temporary="$(find "$case_dir/tmp" -mindepth 1 -print -quit)"
    [[ -z "$temporary" ]] || fail "download working directory was not cleaned: $temporary"
}

sha256_fixture() {
    local file="$1"

    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file" | awk '{print $1}'
    else
        shasum -a 256 "$file" | awk '{print $1}'
    fi
}

write_version_binary() {
    local path="$1"
    local version="$2"

    printf '%s\n' \
        '#!/usr/bin/env bash' \
        'set -u' \
        'if [[ -n "${FAKE_VERSION_EXEC_LOG:-}" ]]; then' \
        '    printf '\''%s\n'\'' "$0" >> "$FAKE_VERSION_EXEC_LOG"' \
        'fi' \
        'if [[ -n "${FAKE_NOEXEC_PREFIX:-}" && "$0" == "$FAKE_NOEXEC_PREFIX"/* ]]; then' \
        '    echo "simulated noexec filesystem: $0" >&2' \
        '    exit 126' \
        'fi' \
        'if [[ "${1:-}" == "--version" ]]; then' \
        "    printf 'ncmctl\\n Version: \\t$version\\n Go version: \\tgo1.25.0\\n Git commit: \\ttest\\n OS/Arch: \\tlinux/amd64\\n Build time: \\ttest\\n'" \
        'fi' > "$path"
    chmod 0755 "$path"
}

create_fixture() {
    local payload_dir="$FIXTURE_DIR/payload"
    local wrong_payload_dir="$FIXTURE_DIR/wrong-payload"
    local archive="$FIXTURE_DIR/ncmctl_Linux_x86_64.tar.gz"
    local wrong_archive="$FIXTURE_DIR/wrong_ncmctl_Linux_x86_64.tar.gz"
    local checksum wrong_checksum

    mkdir -p "$payload_dir" "$wrong_payload_dir"
    write_version_binary "$payload_dir/ncmctl" "1.2.3"
    write_version_binary "$wrong_payload_dir/ncmctl" "9.9.9"
    tar -czf "$archive" -C "$payload_dir" ncmctl
    tar -czf "$wrong_archive" -C "$wrong_payload_dir" ncmctl
    checksum="$(sha256_fixture "$archive")"
    wrong_checksum="$(sha256_fixture "$wrong_archive")"
    printf '%s  %s\n' "$checksum" "ncmctl_Linux_x86_64.tar.gz" > "$FIXTURE_DIR/checksums.txt"
    printf '%s  %s\n' "$wrong_checksum" "ncmctl_Linux_x86_64.tar.gz" > "$FIXTURE_DIR/wrong-checksums.txt"
}

create_fake_commands() {
    mkdir -p "$FAKE_BIN"
    cat > "$FAKE_BIN/curl" <<'EOF'
#!/usr/bin/env bash
set -u

if [[ "${1:-}" == "--retry-connrefused" && "${2:-}" == "--version" ]]; then
    [[ "${FAKE_CURL_RETRY_SUPPORTED:-true}" == "true" ]] || exit 2
    printf '%s\n' 'curl 8.0.0 fake'
    exit 0
fi

output=""
head_request=false
retry_count=0
max_time=""
retry_max_time=""
url=""

while (( $# > 0 )); do
    case "$1" in
        --output|--write-out|--connect-timeout|--max-time|--proto|--proto-redir|--retry-max-time)
            if [[ "$1" == "--output" ]]; then
                output="$2"
            elif [[ "$1" == "--max-time" ]]; then
                max_time="$2"
            elif [[ "$1" == "--retry-max-time" ]]; then
                retry_max_time="$2"
            fi
            shift 2
            ;;
        --retry)
            retry_count="$2"
            shift 2
            ;;
        --head)
            head_request=true
            shift
            ;;
        --fail|--location|--silent|--show-error|--retry-connrefused)
            shift
            ;;
        --*)
            echo "unexpected curl option: $1" >&2
            exit 99
            ;;
        *)
            url="$1"
            shift
            ;;
    esac
done

attempt=1
max_attempts=$((retry_count + 1))
while ((attempt <= max_attempts)); do
    printf '%s|%s|retry=%s|attempt=%s|max-time=%s|retry-max-time=%s\n' \
        "$head_request" "$url" "$retry_count" "$attempt" "$max_time" "$retry_max_time" >> "$FAKE_CURL_LOG"

    # fake curl 在内部模拟 curl 对超时的有限重试，非瞬时 HTTP 错误仍只请求一次。
    if [[ "$head_request" == true && "$url" == https://transient.example/* && "$attempt" -lt "$max_attempts" ]]; then
        attempt=$((attempt + 1))
        continue
    fi
    break
done

if [[ "$head_request" == true ]]; then
    case "${FAKE_MODE:-success}:$url" in
        all_metadata_fail:*|*:https://bad401.example/*|*:https://unauthorized.example/*)
            exit 22
            ;;
        *:https://timeout.example/*)
            exit 28
            ;;
        *:https://parked.example/*)
            printf '%s' "$url"
            exit 0
            ;;
        *:https://foreign.example/*)
            printf '%s' 'https://github.com/attacker/project/releases/tag/v9.9.9'
            exit 0
            ;;
        *:https://query.example/*)
            printf '%s' "${url%/latest}/tag/$FAKE_RELEASE_VERSION?next=/releases/tag/v9.9.9"
            exit 0
            ;;
        *:https://path.example/*)
            printf '%s' "${url%/latest}/tag/$FAKE_RELEASE_VERSION/notes"
            exit 0
            ;;
    esac
    printf '%s' "${url%/latest}/tag/$FAKE_RELEASE_VERSION"
    exit 0
fi

case "$url" in
    *_checksums.txt)
        if [[ "${FAKE_MODE:-success}" == "wrong_version_first" && "$url" == https://wrong.example/* ]]; then
            cp "$FAKE_WRONG_CHECKSUM" "$output"
        else
            cp "$FAKE_CHECKSUM" "$output"
        fi
        ;;
    *.tar.gz)
        if [[ "${FAKE_MODE:-success}" == "all_archives_fail" ]]; then
            exit 22
        fi
        if [[ "${FAKE_MODE:-success}" == "corrupt_first" && "$url" == https://corrupt.example/* ]]; then
            printf 'corrupt archive' > "$output"
        elif [[ "${FAKE_MODE:-success}" == "wrong_version_first" && "$url" == https://wrong.example/* ]]; then
            cp "$FAKE_WRONG_ARCHIVE" "$output"
        else
            cp "$FAKE_ARCHIVE" "$output"
        fi
        ;;
    *)
        echo "unexpected download URL: $url" >&2
        exit 98
        ;;
esac
EOF
    chmod 0755 "$FAKE_BIN/curl"

    cat > "$FAKE_BIN/pgrep" <<'EOF'
#!/usr/bin/env bash
exit "${FAKE_PGREP_STATUS:-1}"
EOF
    chmod 0755 "$FAKE_BIN/pgrep"
}

prepare_case() {
    local case_dir="$1"

    mkdir -p "$case_dir/install" "$case_dir/tmp"
    : > "$case_dir/curl.log"
    : > "$case_dir/version-exec.log"
}

run_main() {
    local case_dir="$1"
    local proxies="$2"
    local mode="$3"

    (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_CURL_LOG="$case_dir/curl.log"
        export FAKE_RELEASE_VERSION="v1.2.3"
        export FAKE_CHECKSUM="$FIXTURE_DIR/checksums.txt"
        export FAKE_ARCHIVE="$FIXTURE_DIR/ncmctl_Linux_x86_64.tar.gz"
        export FAKE_WRONG_CHECKSUM="$FIXTURE_DIR/wrong-checksums.txt"
        export FAKE_WRONG_ARCHIVE="$FIXTURE_DIR/wrong_ncmctl_Linux_x86_64.tar.gz"
        export FAKE_VERSION_EXEC_LOG="$case_dir/version-exec.log"
        export FAKE_NOEXEC_PREFIX="$case_dir/tmp"
        export FAKE_PGREP_STATUS="${FAKE_PGREP_STATUS:-1}"
        export FAKE_MODE="$mode"
        source "$INSTALL_SCRIPT"
        INSTALL_DIR="$case_dir/install"
        BINARY_PATH="$INSTALL_DIR/ncmctl"
        TEMP_ROOT="$case_dir/tmp"
        ARCH="x86_64"
        OS="Linux"
        GITHUB_PROXIES="$proxies"
        MAX_ATTEMPTS=1
        main
    )
}

test_default_and_custom_routes() {
    local actual expected

    actual="$(
        unset NCMCTL_QINGLONG_GITHUB_PROXIES NCMCTL_QINGLONG_MAX_ATTEMPTS MAX_RETRIES
        source "$INSTALL_SCRIPT"
        configure_routes
        for route in "${ROUTES[@]}"; do
            printf '%s\n' "${route:-direct}"
        done
    )"
    expected=$'https://ghproxy.net/\nhttps://ghfast.top/\nhttps://gh-proxy.com/\ndirect'
    assert_equal "$expected" "$actual"

    actual="$(
        export NCMCTL_QINGLONG_GITHUB_PROXIES=$'https://one.example/\nhttps://two.example/path'
        source "$INSTALL_SCRIPT"
        configure_routes
        for route in "${ROUTES[@]}"; do
            printf '%s\n' "${route:-direct}"
        done
    )"
    expected=$'https://one.example/\nhttps://two.example/path/\ndirect'
    assert_equal "$expected" "$actual"

    actual="$(
        export NCMCTL_QINGLONG_GITHUB_PROXIES=''
        source "$INSTALL_SCRIPT"
        configure_routes
        printf '%s:%s' "${#ROUTES[@]}" "${ROUTES[0]:-direct}"
    )"
    assert_equal "1:direct" "$actual"
}

test_invalid_proxy_fails_fast() {
    local invalid_proxy

    for invalid_proxy in \
        "http://insecure.example/" \
        "https:///missing-host" \
        "https://:" \
        "https://@/" \
        "https://example.com:0/" \
        "https://example.com:65536/" \
        "https://example.com:not-a-port/"; do
        if (
            source "$INSTALL_SCRIPT"
            GITHUB_PROXIES="$invalid_proxy"
            configure_routes
        ) >/dev/null 2>&1; then
            fail "invalid proxy was accepted: $invalid_proxy"
        fi
    done
}

test_proxy_logs_are_redacted() {
    local case_dir="$TEST_ROOT/proxy-redaction"
    local actual

    prepare_case "$case_dir"
    actual="$(
        source "$INSTALL_SCRIPT"
        route_name "https://user:secret@proxy.example:8443/private/token?access=hidden"
    )"
    assert_equal "HTTPS proxy proxy.example:8443" "$actual"

    (
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES="https://proxy.example/private-token/"
        configure_routes
        route_name "${ROUTES[0]}"
    ) > "$case_dir/output.log"
    assert_equal "HTTPS proxy proxy.example" "$(cat "$case_dir/output.log")"
    assert_not_contains "private-token" "$case_dir/output.log"

    if (
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES="https://user:invalid-secret@proxy.example/"
        configure_routes
    ) > "$case_dir/invalid.log" 2>&1; then
        fail "credential-bearing invalid proxy was accepted"
    fi
    assert_contains "Invalid HTTPS GitHub proxy at position 1" "$case_dir/invalid.log"
    assert_not_contains "invalid-secret" "$case_dir/invalid.log"
}

test_release_architecture_matrix() {
    local uname_arch asset_arch actual

    while IFS='|' read -r uname_arch asset_arch; do
        actual="$(
            source "$INSTALL_SCRIPT"
            ARCH="$uname_arch"
            OS="Linux"
            map_architecture
            printf '%s' "$ASSET_NAME"
        )"
        assert_equal "ncmctl_Linux_${asset_arch}.tar.gz" "$actual"
    done <<'EOF'
x86_64|x86_64
amd64|x86_64
aarch64|arm64
arm64|arm64
armv6l|armv6
armv7l|armv6
armv8l|armv6
mips|mips
mipsel|mipsle
mipsle|mipsle
mips64|mips64
mips64el|mips64le
mips64le|mips64le
ppc64|ppc64
powerpc64|ppc64
ppc64le|ppc64le
powerpc64le|ppc64le
riscv64|riscv64
loongarch64|loong64
loong64|loong64
386|i386
i386|i386
i486|i386
i586|i386
i686|i386
s390x|s390x
EOF
}

test_release_url_validation() {
    local prefix="https://proxy.example/"

    (
        source "$INSTALL_SCRIPT"
        assert_equal "v1.2.3" "$(version_from_release_url \
            "${prefix}https://github.com/chaunsin/netease-cloud-music/releases/tag/v1.2.3" "$prefix")"
        assert_equal "v1.2.3-alpha.4" "$(version_from_release_url \
            "https://github.com/chaunsin/netease-cloud-music/releases/tag/v1.2.3-alpha.4" "$prefix")"

        if version_from_release_url \
            "${prefix}https://github.com/attacker/project/releases/tag/v9.9.9" "$prefix" >/dev/null; then
            fail "foreign repository release URL was accepted"
        fi
        if version_from_release_url \
            "${prefix}https://github.com/chaunsin/netease-cloud-music/releases/tag/v1.2.3/notes" "$prefix" >/dev/null; then
            fail "release URL with an extra path was accepted"
        fi
        if version_from_release_url \
            "${prefix}https://github.com/chaunsin/netease-cloud-music/releases/tag/v1.2.3?next=v9.9.9" "$prefix" >/dev/null; then
            fail "release URL with a query string was accepted"
        fi
    )
}

test_metadata_failover() {
    local case_dir="$TEST_ROOT/metadata"

    prepare_case "$case_dir"
    (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_CURL_LOG="$case_dir/curl.log"
        export FAKE_RELEASE_VERSION="v1.2.3"
        export FAKE_MODE="success"
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES=$'https://bad401.example/\nhttps://timeout.example/\nhttps://parked.example/\nhttps://foreign.example/\nhttps://query.example/\nhttps://path.example/\nhttps://good.example/'
        MAX_ATTEMPTS=1
        configure_routes
        get_latest_version > "$case_dir/output.log" 2>&1
        printf '%s' "$LATEST_VERSION" > "$case_dir/version"
    )

    assert_equal "v1.2.3" "$(cat "$case_dir/version")"
    assert_equal "7" "$(wc -l < "$case_dir/curl.log" | tr -d ' ')"
    assert_contains "https://parked.example/https://github.com/chaunsin/netease-cloud-music/releases/latest" "$case_dir/curl.log"
    assert_contains "https://good.example/https://github.com/chaunsin/netease-cloud-music/releases/latest" "$case_dir/curl.log"
}

test_retry_configuration() {
    local case_dir="$TEST_ROOT/retry"
    local actual

    prepare_case "$case_dir"
    actual="$(
        unset NCMCTL_QINGLONG_MAX_ATTEMPTS
        export MAX_RETRIES=3
        source "$INSTALL_SCRIPT"
        printf '%s' "$MAX_ATTEMPTS"
    )"
    assert_equal "3" "$actual"

    actual="$(
        unset NCMCTL_QINGLONG_MAX_ATTEMPTS
        export MAX_RETRIES=''
        source "$INSTALL_SCRIPT"
        printf '%s' "$MAX_ATTEMPTS"
    )"
    assert_equal "3" "$actual"

    actual="$(
        export NCMCTL_QINGLONG_MAX_ATTEMPTS=2
        export MAX_RETRIES=4
        source "$INSTALL_SCRIPT"
        printf '%s' "$MAX_ATTEMPTS"
    )"
    assert_equal "2" "$actual"

    (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_CURL_LOG="$case_dir/curl.log"
        export FAKE_RELEASE_VERSION="v1.2.3"
        export FAKE_CURL_RETRY_SUPPORTED=true
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES="https://transient.example/"
        MAX_ATTEMPTS=3
        configure_routes
        curl_effective_url "https://transient.example/https://github.com/chaunsin/netease-cloud-music/releases/latest" >/dev/null
    )
    assert_contains "retry=2" "$case_dir/curl.log"
    assert_contains "attempt=3|max-time=30" "$case_dir/curl.log"
    assert_contains "retry-max-time=90" "$case_dir/curl.log"
    assert_equal "3" "$(wc -l < "$case_dir/curl.log" | tr -d ' ')"

    : > "$case_dir/curl.log"
    (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_CURL_LOG="$case_dir/curl.log"
        export FAKE_ARCHIVE="$FIXTURE_DIR/ncmctl_Linux_x86_64.tar.gz"
        export FAKE_CURL_RETRY_SUPPORTED=true
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES="https://good.example/"
        MAX_ATTEMPTS=3
        configure_routes
        curl_download "https://good.example/ncmctl_Linux_x86_64.tar.gz" "$case_dir/archive.tar.gz" "$DOWNLOAD_TIMEOUT"
    )
    assert_contains "max-time=300|retry-max-time=900" "$case_dir/curl.log"

    : > "$case_dir/curl.log"
    if (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_CURL_LOG="$case_dir/curl.log"
        export FAKE_RELEASE_VERSION="v1.2.3"
        export FAKE_CURL_RETRY_SUPPORTED=true
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES="https://unauthorized.example/"
        MAX_ATTEMPTS=3
        configure_routes
        curl_effective_url "https://unauthorized.example/https://github.com/chaunsin/netease-cloud-music/releases/latest" >/dev/null
    ); then
        fail "non-transient HTTP failure was retried as success"
    fi
    assert_equal "1" "$(wc -l < "$case_dir/curl.log" | tr -d ' ')"

    if (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_CURL_RETRY_SUPPORTED=false
        source "$INSTALL_SCRIPT"
        GITHUB_PROXIES="https://good.example/"
        MAX_ATTEMPTS=2
        configure_routes
    ) >/dev/null 2>&1; then
        fail "unsupported curl accepted retry configuration"
    fi
}

test_semver_comparison() {
    (
        source "$INSTALL_SCRIPT"
        assert_equal "0" "$(compare_semver v1.2.3 1.2.3+build.7)"
        assert_equal "1" "$(compare_semver 2.0.0 1.99.99)"
        assert_equal "1" "$(compare_semver 1.2.3 1.2.3-rc.1)"
        assert_equal "-1" "$(compare_semver 1.2.3-alpha.2 1.2.3-alpha.10)"
    )
}

test_up_to_date_skips_assets() {
    local case_dir="$TEST_ROOT/up-to-date"

    prepare_case "$case_dir"
    write_version_binary "$case_dir/install/ncmctl" "1.2.3"
    run_main "$case_dir" "https://good.example/" "success" > "$case_dir/output.log" 2>&1

    assert_equal "1" "$(wc -l < "$case_dir/curl.log" | tr -d ' ')"
    assert_contains "is up-to-date" "$case_dir/output.log"
    assert_no_temporary_files "$case_dir"
}

test_newer_installation_skips_downgrade() {
    local case_dir="$TEST_ROOT/newer"
    local before after

    prepare_case "$case_dir"
    write_version_binary "$case_dir/install/ncmctl" "2.0.0"
    before="$(sha256_fixture "$case_dir/install/ncmctl")"
    run_main "$case_dir" "https://good.example/" "success" > "$case_dir/output.log" 2>&1
    after="$(sha256_fixture "$case_dir/install/ncmctl")"

    assert_equal "$before" "$after"
    assert_equal "1" "$(wc -l < "$case_dir/curl.log" | tr -d ' ')"
    assert_contains "skipping downgrade" "$case_dir/output.log"
    assert_no_temporary_files "$case_dir"
}

test_noexec_temp_root_stages_before_execution() {
    local case_dir="$TEST_ROOT/noexec-temp"

    prepare_case "$case_dir"
    run_main "$case_dir" "https://good.example/" "success" > "$case_dir/output.log" 2>&1

    assert_equal "1.2.3" "$(
        "$case_dir/install/ncmctl" --version | awk '/^[[:space:]]*Version:/ { print $2; exit }'
    )"
    assert_contains "$case_dir/install/.ncmctl." "$case_dir/version-exec.log"
    assert_not_contains "$case_dir/tmp/" "$case_dir/version-exec.log"
    assert_not_contains "simulated noexec filesystem" "$case_dir/output.log"
    assert_no_temporary_files "$case_dir"
}

test_concurrent_upgrade_does_not_downgrade() {
    local case_dir="$TEST_ROOT/concurrent-upgrade"
    local stale_candidate

    prepare_case "$case_dir"
    stale_candidate="$case_dir/install/.ncmctl.stale-candidate"
    write_version_binary "$case_dir/install/ncmctl" "0.9.0"
    write_version_binary "$stale_candidate" "1.2.3"

    (
        export PATH="$FAKE_BIN:$ORIGINAL_PATH"
        export FAKE_VERSION_EXEC_LOG="$case_dir/version-exec.log"
        export FAKE_PGREP_STATUS=1
        source "$INSTALL_SCRIPT"
        INSTALL_DIR="$case_dir/install"
        BINARY_PATH="$INSTALL_DIR/ncmctl"
        INSTALL_TEMP="$stale_candidate"
        STAGED_BINARY="$stale_candidate"
        LATEST_VERSION="v1.2.3"
        trap cleanup EXIT

        # Installer A made its initial decision, then installer B completed a newer upgrade.
        if is_up_to_date > "$case_dir/initial-check.log" 2>&1; then
            fail "older installation was unexpectedly treated as current"
        fi
        write_version_binary "$BINARY_PATH" "2.0.0"
        install_binary "$STAGED_BINARY"
    ) > "$case_dir/output.log" 2>&1

    assert_equal "2.0.0" "$(
        "$case_dir/install/ncmctl" --version | awk '/^[[:space:]]*Version:/ { print $2; exit }'
    )"
    assert_contains "skipping downgrade" "$case_dir/output.log"
    assert_no_temporary_files "$case_dir"
}

test_install_lock_contention_fails_closed() {
    local case_dir="$TEST_ROOT/lock-contention"
    local lock_dir

    prepare_case "$case_dir"
    lock_dir="$case_dir/install/.ncmctl.install.lock"
    mkdir "$lock_dir"
    if (
        source "$INSTALL_SCRIPT"
        INSTALL_DIR="$case_dir/install"
        acquire_install_lock
    ) > "$case_dir/output.log" 2>&1; then
        fail "installer unexpectedly acquired an existing lock"
    fi

    assert_contains "another installer may be running" "$case_dir/output.log"
    [[ -d "$lock_dir" ]] || fail "installer removed a lock it did not own"
}

test_corrupt_archive_switches_route() {
    local case_dir="$TEST_ROOT/corrupt"

    prepare_case "$case_dir"
    run_main "$case_dir" "https://corrupt.example/ https://good.example/" "corrupt_first" > "$case_dir/output.log" 2>&1

    [[ -x "$case_dir/install/ncmctl" ]] || fail "binary was not installed"
    assert_equal "1.2.3" "$(
        "$case_dir/install/ncmctl" --version | awk '/^[[:space:]]*Version:/ { print $2; exit }'
    )"
    assert_contains "SHA-256 verification failed via HTTPS proxy corrupt.example" "$case_dir/output.log"
    assert_contains "https://corrupt.example/https://github.com/chaunsin/netease-cloud-music/releases/download/v1.2.3/ncmctl_1.2.3_checksums.txt" "$case_dir/curl.log"
    assert_contains "https://good.example/https://github.com/chaunsin/netease-cloud-music/releases/download/v1.2.3/ncmctl_Linux_x86_64.tar.gz" "$case_dir/curl.log"
    assert_contains "$case_dir/install/.ncmctl." "$case_dir/version-exec.log"
    assert_not_contains "$case_dir/tmp/ncmctl_upgrade" "$case_dir/version-exec.log"
    assert_no_temporary_files "$case_dir"
}

test_wrong_binary_version_switches_route() {
    local case_dir="$TEST_ROOT/wrong-version"

    prepare_case "$case_dir"
    run_main "$case_dir" "https://wrong.example/ https://good.example/" "wrong_version_first" > "$case_dir/output.log" 2>&1

    assert_equal "1.2.3" "$(
        "$case_dir/install/ncmctl" --version | awk '/^[[:space:]]*Version:/ { print $2; exit }'
    )"
    assert_contains "Downloaded binary version 9.9.9 does not match release v1.2.3 via HTTPS proxy wrong.example" "$case_dir/output.log"
    assert_contains "https://good.example/https://github.com/chaunsin/netease-cloud-music/releases/download/v1.2.3/ncmctl_1.2.3_checksums.txt" "$case_dir/curl.log"
    assert_no_temporary_files "$case_dir"
}

test_process_check_failure_preserves_existing_binary() {
    local case_dir status before after

    for status in 0 2; do
        case_dir="$TEST_ROOT/pgrep-$status"
        prepare_case "$case_dir"
        write_version_binary "$case_dir/install/ncmctl" "0.9.0"
        before="$(sha256_fixture "$case_dir/install/ncmctl")"
        if FAKE_PGREP_STATUS="$status" run_main "$case_dir" "https://good.example/" "success" > "$case_dir/output.log" 2>&1; then
            fail "installation unexpectedly ignored pgrep status $status"
        fi
        after="$(sha256_fixture "$case_dir/install/ncmctl")"
        assert_equal "$before" "$after"
        if [[ "$status" == 0 ]]; then
            assert_contains "is currently running" "$case_dir/output.log"
        else
            assert_contains "pgrep exit: 2" "$case_dir/output.log"
        fi
        assert_no_temporary_files "$case_dir"
    done
}

test_all_fail_preserves_existing_binary() {
    local case_dir="$TEST_ROOT/all-fail"
    local before after

    prepare_case "$case_dir"
    write_version_binary "$case_dir/install/ncmctl" "0.9.0"
    before="$(sha256_fixture "$case_dir/install/ncmctl")"
    if run_main "$case_dir" "https://one.example/ https://two.example/" "all_archives_fail" > "$case_dir/output.log" 2>&1; then
        fail "installation unexpectedly succeeded"
    fi
    after="$(sha256_fixture "$case_dir/install/ncmctl")"

    assert_equal "$before" "$after"
    assert_contains "Unable to download a valid ncmctl_Linux_x86_64.tar.gz" "$case_dir/output.log"
    assert_no_temporary_files "$case_dir"
}

run_test() {
    local name="$1"
    shift

    # 测试函数必须作为普通命令执行；放进 if 条件会让 Bash 忽略其调用链中的 set -e。
    "$@"
    PASS_COUNT=$((PASS_COUNT + 1))
    echo "ok $PASS_COUNT - $name"
}

mkdir -p "$FIXTURE_DIR"
create_fixture
create_fake_commands

run_test "default, custom, and direct-only routes" test_default_and_custom_routes
run_test "invalid proxy fails before network access" test_invalid_proxy_fails_fast
run_test "proxy route logs redact credentials and paths" test_proxy_logs_are_redacted
run_test "uname aliases cover the GoReleaser architecture matrix" test_release_architecture_matrix
run_test "release redirects are bound to the target repository" test_release_url_validation
run_test "metadata failures and invalid redirects switch routes" test_metadata_failover
run_test "transient retry policy and curl support are enforced" test_retry_configuration
run_test "semantic versions are ordered without downgrades" test_semver_comparison
run_test "current installations skip asset downloads" test_up_to_date_skips_assets
run_test "newer installations are not downgraded" test_newer_installation_skips_downgrade
run_test "candidate execution avoids a simulated noexec download root" test_noexec_temp_root_stages_before_execution
run_test "concurrent upgrades cannot replace a newer installation" test_concurrent_upgrade_does_not_downgrade
run_test "installation lock contention fails closed" test_install_lock_contention_fails_closed
run_test "checksum failure switches to the next route" test_corrupt_archive_switches_route
run_test "wrong binary versions switch to the next route" test_wrong_binary_version_switches_route
run_test "running-process checks fail closed" test_process_check_failure_preserves_existing_binary
run_test "failed downloads preserve the installed binary" test_all_fail_preserves_existing_binary

echo "$PASS_COUNT tests passed"
