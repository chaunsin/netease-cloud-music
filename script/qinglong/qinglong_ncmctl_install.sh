#!/usr/bin/env bash

# Copyright (c) 2024-2026 chaunsin
# SPDX-License-Identifier: MIT

# name: ncmctl安装
# cron: 0 1 * * *

set -euo pipefail

# 安装位置和 GitHub 唯一事实源；代理只负责转发完整 GitHub URL，不参与版本定义。
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ncmctl"
BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
REPO="chaunsin/netease-cloud-music"
GITHUB_REPO_URL="https://github.com/$REPO"

# NCMCTL_QINGLONG_GITHUB_PROXIES 未设置时使用默认镜像；显式设为空字符串时仅直连 GitHub。
# 多个代理前缀可用空格、制表符或换行分隔，configure_routes 会校验并统一补齐末尾斜杠。
DEFAULT_GITHUB_PROXIES="https://ghproxy.net/ https://ghfast.top/ https://gh-proxy.com/"
GITHUB_PROXIES="${NCMCTL_QINGLONG_GITHUB_PROXIES-$DEFAULT_GITHUB_PROXIES}"

# NCMCTL_QINGLONG_MAX_ATTEMPTS 表示每个 URL 的总尝试次数；新变量优先，MAX_RETRIES 仅用于兼容旧配置。
if [[ -n "${NCMCTL_QINGLONG_MAX_ATTEMPTS+x}" ]]; then
    MAX_ATTEMPTS="$NCMCTL_QINGLONG_MAX_ATTEMPTS"
elif [[ -n "${MAX_RETRIES+x}" ]]; then
    # 旧脚本把显式空值也当成默认 3 次，保留该细节以免已有青龙环境行为突变。
    MAX_ATTEMPTS="${MAX_RETRIES:-3}"
else
    MAX_ATTEMPTS=1
fi

# 元数据与归档分别限制总耗时，连接超时用于尽快跳过不可达的公益镜像。
CONNECT_TIMEOUT=10
METADATA_TIMEOUT=30
DOWNLOAD_TIMEOUT=300
TEMP_ROOT="${TMPDIR:-/tmp}"

# Release tag 使用 SemVer；该表达式同时供最终 URL 校验和防降级比较复用。
SEMVER_PATTERN='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

# 以下状态由主流程逐步填充，并由 EXIT trap 统一清理未完成的临时文件。
ARCH="$(uname -m)"
OS="$(uname -s)"
LATEST_VERSION=""
ASSET_NAME=""
EXPECTED_SHA256=""
TEMP_DIR=""
INSTALL_TEMP=""
STAGED_BINARY=""
INSTALL_LOCK_DIR=""
ROUTES=()
CURL_ARGS=()

# 输出致命错误并立即终止，避免在状态不完整时继续安装。
die() {
    echo "Error: $*" >&2
    exit 1
}

# 清理由工作目录和安装目录产生的临时文件；已完成的正式二进制不会被删除。
cleanup() {
    if [[ -n "$INSTALL_TEMP" ]]; then
        rm -f -- "$INSTALL_TEMP" || echo "Warning: unable to remove installation staging file: $INSTALL_TEMP" >&2
    fi
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf -- "$TEMP_DIR" || echo "Warning: unable to remove download working directory: $TEMP_DIR" >&2
    fi
    if ! release_install_lock; then
        echo "Warning: unable to release installation lock: $INSTALL_LOCK_DIR" >&2
    fi
}

# 检查安装流程依赖的外部命令，SHA-256 工具缺失时不降级为弱校验。
require_commands() {
    local command_name

    for command_name in curl tar awk mktemp pgrep cp mv chmod mkdir rmdir rm; do
        command -v "$command_name" >/dev/null 2>&1 || die "Required command not found: $command_name"
    done
    if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
        die "Required SHA-256 tool not found: install sha256sum or shasum"
    fi
}

# 校验代理 URL 的 authority，拒绝用户信息、空主机和越界端口等 curl 可能宽松接受的写法。
validate_https_proxy() {
    local proxy="$1"
    local authority host port=""
    local label
    local https_url_pattern='^https://([^/[:space:]?#]+)(/[^[:space:]?#]*)?$'
    local -a labels=()

    [[ "$proxy" =~ $https_url_pattern ]] || return 1
    authority="${BASH_REMATCH[1]}"
    [[ "$authority" != *"@"* ]] || return 1

    if [[ "$authority" == \[* ]]; then
        [[ "$authority" =~ ^\[([0-9A-Fa-f:.]+)\](:([0-9]+))?$ ]] || return 1
        host="${BASH_REMATCH[1]}"
        port="${BASH_REMATCH[3]:-}"
        [[ "$host" == *:* ]] || return 1
    else
        [[ "$authority" != *:*:* ]] || return 1
        if [[ "$authority" == *:* ]]; then
            host="${authority%%:*}"
            port="${authority#*:}"
        else
            host="$authority"
        fi
        IFS='.' read -r -a labels <<< "$host"
        ((${#labels[@]} > 0)) || return 1
        for label in "${labels[@]}"; do
            [[ "$label" =~ ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$ ]] || return 1
        done
    fi

    if [[ -n "$port" ]]; then
        [[ "$port" =~ ^[0-9]{1,5}$ ]] || return 1
        ((10#$port >= 1 && 10#$port <= 65535)) || return 1
    elif [[ "$authority" == *: ]]; then
        return 1
    fi
}

# curl 7.52.0 才支持连接拒绝重试；用户启用多次尝试时先检查能力，避免运行到一半才失败。
curl_supports_retry_connrefused() {
    curl --retry-connrefused --version >/dev/null 2>&1
}

# 校验并规范化代理路由，同时构造所有 curl 请求共享的安全参数。
configure_routes() {
    local proxy
    local proxy_index=0
    local proxy_config="$GITHUB_PROXIES"
    local -a proxies=()

    [[ "$MAX_ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || die "NCMCTL_QINGLONG_MAX_ATTEMPTS must be a positive integer"

    # 将环境变量允许的空格、制表符和换行统一为 shell 数组可解析的分隔符。
    proxy_config="${proxy_config//$'\n'/ }"
    proxy_config="${proxy_config//$'\r'/ }"
    proxy_config="${proxy_config//$'\t'/ }"
    ROUTES=()
    if [[ "$proxy_config" =~ [^[:space:]] ]]; then
        read -r -a proxies <<< "$proxy_config"
        for proxy in "${proxies[@]}"; do
            proxy_index=$((proxy_index + 1))
            validate_https_proxy "$proxy" || die "Invalid HTTPS GitHub proxy at position $proxy_index"
            while [[ "$proxy" == */ ]]; do
                proxy="${proxy%/}"
            done
            ROUTES+=("$proxy/")
        done
    fi
    # 空前缀代表 GitHub 直连，并固定放在最后作为所有镜像失败后的兜底路由。
    ROUTES+=("")

    # 首次请求和重定向都只允许 HTTPS，防止代理把下载链路降级到明文 HTTP。
    CURL_ARGS=(
        --fail
        --location
        --silent
        --show-error
        --proto '=https'
        --proto-redir '=https'
        --connect-timeout "$CONNECT_TIMEOUT"
    )
    if (( MAX_ATTEMPTS > 1 )); then
        curl_supports_retry_connrefused || die "curl 7.52.0 or newer is required when NCMCTL_QINGLONG_MAX_ATTEMPTS is greater than 1"
        # curl 的 --retry 表示首次请求之后的重试次数，因此需要用总尝试次数减一。
        CURL_ARGS+=(
            --retry "$((MAX_ATTEMPTS - 1))"
            --retry-connrefused
        )
    fi
}

# 将 uname 输出映射为 GoReleaser 的资产架构名，并生成当前平台的归档名称。
map_architecture() {
    case "$ARCH" in
        x86_64|amd64) ARCH="x86_64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv6l|armv7l|armv8l) ARCH="armv6" ;;
        mips) ARCH="mips" ;;
        mipsel|mipsle) ARCH="mipsle" ;;
        mips64) ARCH="mips64" ;;
        mips64el|mips64le) ARCH="mips64le" ;;
        ppc64|powerpc64) ARCH="ppc64" ;;
        ppc64le|powerpc64le) ARCH="ppc64le" ;;
        riscv64) ARCH="riscv64" ;;
        loongarch64|loong64) ARCH="loong64" ;;
        386|i386|i486|i586|i686) ARCH="i386" ;;
        s390x) ARCH="s390x" ;;
        *) die "Unsupported architecture: $ARCH" ;;
    esac
    ASSET_NAME="${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
}

# 将代理前缀与完整 GitHub URL 拼接；空前缀自然得到 GitHub 直连地址。
route_url() {
    local prefix="$1"
    local github_url="$2"

    printf '%s%s' "$prefix" "$github_url"
}

# 日志只展示代理 authority，避免自定义路径、查询参数或用户信息泄露到青龙日志。
route_name() {
    local prefix="$1"
    local authority

    if [[ -z "$prefix" ]]; then
        printf '%s' "GitHub"
        return
    fi

    authority="${prefix#*://}"
    authority="${authority%%/*}"
    authority="${authority%%\?*}"
    authority="${authority%%\#*}"
    authority="${authority##*@}"
    if [[ -z "$authority" || "$authority" =~ [[:space:]] ]]; then
        printf '%s' "configured HTTPS proxy"
        return
    fi
    printf 'HTTPS proxy %s' "$authority"
}

# 通过轻量 HEAD 请求取得重定向后的最终 URL，供 latest tag 严格解析。
curl_effective_url() {
    local url="$1"

    if (( MAX_ATTEMPTS > 1 )); then
        curl "${CURL_ARGS[@]}" \
            --retry-max-time "$((METADATA_TIMEOUT * MAX_ATTEMPTS))" \
            --max-time "$METADATA_TIMEOUT" \
            --head \
            --output /dev/null \
            --write-out '%{url_effective}' \
            "$url"
        return
    fi

    curl "${CURL_ARGS[@]}" \
        --max-time "$METADATA_TIMEOUT" \
        --head \
        --output /dev/null \
        --write-out '%{url_effective}' \
        "$url"
}

# 使用统一的 HTTPS、超时和有限重试策略将响应写入指定临时文件。
curl_download() {
    local url="$1"
    local destination="$2"
    local timeout="$3"

    # 重试总窗口随请求类型计算，避免固定短窗口让 300 秒归档超时后失去下一次尝试机会。
    if (( MAX_ATTEMPTS > 1 )); then
        curl "${CURL_ARGS[@]}" \
            --retry-max-time "$((timeout * MAX_ATTEMPTS))" \
            --max-time "$timeout" \
            --output "$destination" \
            "$url"
        return
    fi

    curl "${CURL_ARGS[@]}" \
        --max-time "$timeout" \
        --output "$destination" \
        "$url"
}

# 只从当前仓库的直连或当前代理 URL 中提取完整 SemVer tag，拒绝外部仓库和额外路径参数。
version_from_release_url() {
    local url="$1"
    local prefix="${2:-}"
    local version=""
    local direct_tag_prefix="$GITHUB_REPO_URL/releases/tag/"
    local routed_tag_prefix

    routed_tag_prefix="$(route_url "$prefix" "$direct_tag_prefix")"
    if [[ "$url" == "$routed_tag_prefix"* ]]; then
        version="${url#"$routed_tag_prefix"}"
    elif [[ -n "$prefix" && "$url" == "$direct_tag_prefix"* ]]; then
        # 部分代理会在完成转发后跳回 github.com，仍只接受同一个目标仓库。
        version="${url#"$direct_tag_prefix"}"
    else
        return 1
    fi

    [[ "$version" == v* && "${version#v}" =~ $SEMVER_PATTERN ]] || return 1
    printf '%s' "$version"
}

# 按“镜像优先、GitHub 直连兜底”的顺序解析 GitHub 最新 Release 版本。
get_latest_version() {
    local prefix url effective_url version
    local latest_url="$GITHUB_REPO_URL/releases/latest"

    echo "Fetching the latest release tag from GitHub..."
    for prefix in "${ROUTES[@]}"; do
        url="$(route_url "$prefix" "$latest_url")"
        echo "Trying $(route_name "$prefix")..."
        # 即使 HTTP 状态为 200，最终 URL 不是 /releases/tag/v... 的停放页也必须视为失败。
        if effective_url="$(curl_effective_url "$url")" && version="$(version_from_release_url "$effective_url" "$prefix")"; then
            LATEST_VERSION="$version"
            echo "Latest version: $LATEST_VERSION"
            return 0
        fi
        echo "Failed to resolve the latest release tag via $(route_name "$prefix")." >&2
    done

    die "Unable to resolve the latest GitHub release from any configured route"
}

# 仅从 ncmctl --version 输出的 Version: 行读取版本，忽略提交号等其他字段。
binary_version() {
    local binary="$1"
    local output version

    output="$("$binary" --version 2>/dev/null)" || return 1
    version="$(printf '%s\n' "$output" | awk '
        /^[[:space:]]*Version:[[:space:]]*/ {
            sub(/^[[:space:]]*Version:[[:space:]]*/, "")
            print
            exit
        }
    ')"
    [[ -n "$version" ]] || return 1
    printf '%s' "$version"
}

# 比较两个任意长度的十进制数字串，避免恶意或异常版本号触发 shell 整数溢出。
compare_decimal() {
    local left="$1"
    local right="$2"

    while [[ "${#left}" -gt 1 && "$left" == 0* ]]; do left="${left#0}"; done
    while [[ "${#right}" -gt 1 && "$right" == 0* ]]; do right="${right#0}"; done
    if ((${#left} != ${#right})); then
        ((${#left} > ${#right})) && printf '1' || printf '%s' '-1'
    elif [[ "$left" == "$right" ]]; then
        printf '0'
    elif [[ "$left" > "$right" ]]; then
        printf '1'
    else
        printf '%s' '-1'
    fi
}

# 按 SemVer 规则比较版本，输出 1、0、-1；构建元数据不参与优先级比较。
compare_semver() {
    local left="${1#v}"
    local right="${2#v}"
    local left_major left_minor left_patch left_pre
    local right_major right_minor right_patch right_pre
    local comparison left_identifier right_identifier
    local index=0
    local LC_ALL=C
    local -a left_core=() right_core=() left_pre_ids=() right_pre_ids=()

    [[ "$left" =~ $SEMVER_PATTERN ]] || return 2
    left_major="${BASH_REMATCH[1]}"
    left_minor="${BASH_REMATCH[2]}"
    left_patch="${BASH_REMATCH[3]}"
    left_pre="${BASH_REMATCH[5]:-}"
    [[ "$right" =~ $SEMVER_PATTERN ]] || return 2
    right_major="${BASH_REMATCH[1]}"
    right_minor="${BASH_REMATCH[2]}"
    right_patch="${BASH_REMATCH[3]}"
    right_pre="${BASH_REMATCH[5]:-}"

    left_core=("$left_major" "$left_minor" "$left_patch")
    right_core=("$right_major" "$right_minor" "$right_patch")
    for index in 0 1 2; do
        comparison="$(compare_decimal "${left_core[index]}" "${right_core[index]}")"
        if [[ "$comparison" != 0 ]]; then
            printf '%s' "$comparison"
            return 0
        fi
    done

    [[ -n "$left_pre" || -n "$right_pre" ]] || { printf '0'; return 0; }
    [[ -n "$left_pre" ]] || { printf '1'; return 0; }
    [[ -n "$right_pre" ]] || { printf '%s' '-1'; return 0; }
    IFS='.' read -r -a left_pre_ids <<< "$left_pre"
    IFS='.' read -r -a right_pre_ids <<< "$right_pre"
    index=0
    while ((index < ${#left_pre_ids[@]} || index < ${#right_pre_ids[@]})); do
        ((index < ${#left_pre_ids[@]})) || { printf '%s' '-1'; return 0; }
        ((index < ${#right_pre_ids[@]})) || { printf '1'; return 0; }
        left_identifier="${left_pre_ids[index]}"
        right_identifier="${right_pre_ids[index]}"
        if [[ "$left_identifier" =~ ^[0-9]+$ && "$right_identifier" =~ ^[0-9]+$ ]]; then
            comparison="$(compare_decimal "$left_identifier" "$right_identifier")"
        elif [[ "$left_identifier" =~ ^[0-9]+$ ]]; then
            comparison=-1
        elif [[ "$right_identifier" =~ ^[0-9]+$ ]]; then
            comparison=1
        elif [[ "$left_identifier" == "$right_identifier" ]]; then
            comparison=0
        elif [[ "$left_identifier" > "$right_identifier" ]]; then
            comparison=1
        else
            comparison=-1
        fi
        if [[ "$comparison" != 0 ]]; then
            printf '%s' "$comparison"
            return 0
        fi
        index=$((index + 1))
    done
    printf '0'
}

# 比较已安装版本和最新 Release；相同或本地更高时提前结束，避免浪费流量或意外降级。
is_up_to_date() {
    local installed_version comparison

    [[ -f "$BINARY_PATH" ]] || return 1
    echo "$BINARY_NAME is already installed at $BINARY_PATH."
    if ! installed_version="$(binary_version "$BINARY_PATH")"; then
        echo "Unable to read the installed version; reinstalling." >&2
        return 1
    fi

    if ! comparison="$(compare_semver "$installed_version" "$LATEST_VERSION")"; then
        echo "Installed version $installed_version is not valid SemVer; reinstalling $LATEST_VERSION." >&2
        return 1
    fi
    case "$comparison" in
        0)
            echo "$BINARY_NAME is up-to-date (version: $installed_version)."
            return 0
            ;;
        1)
            echo "Installed version $installed_version is newer than GitHub Release $LATEST_VERSION; skipping downgrade."
            return 0
            ;;
        *)
            echo "Installed version: $installed_version. A newer version ($LATEST_VERSION) is available."
            return 1
            ;;
    esac
}

# 使用原子 mkdir 提供不依赖 flock 的短时安装互斥；已有锁一律保留并清晰失败，避免误删活跃任务的锁。
acquire_install_lock() {
    local lock_dir="$INSTALL_DIR/.${BINARY_NAME}.install.lock"

    if ! mkdir "$lock_dir"; then
        die "Unable to acquire installation lock $lock_dir; another installer may be running (remove a stale lock only after verifying no installer is active)"
    fi
    INSTALL_LOCK_DIR="$lock_dir"
}

release_install_lock() {
    local lock_dir

    [[ -n "$INSTALL_LOCK_DIR" ]] || return 0
    lock_dir="$INSTALL_LOCK_DIR"
    # 先清除本进程的所有权标记，避免释放后 EXIT trap 误删下一位安装器刚取得的同名锁。
    INSTALL_LOCK_DIR=""
    if ! rmdir "$lock_dir"; then
        INSTALL_LOCK_DIR="$lock_dir"
        return 1
    fi
}

# 从 GoReleaser 校验清单中精确提取当前平台资产对应的 64 位 SHA-256。
checksum_from_manifest() {
    local manifest="$1"

    awk -v asset="$ASSET_NAME" '
        $2 == asset && length($1) == 64 && $1 !~ /[^0-9A-Fa-f]/ {
            print tolower($1)
            exit
        }
    ' "$manifest"
}

# 计算文件 SHA-256，优先使用 Linux 常见的 sha256sum，并兼容 shasum。
sha256_file() {
    local file="$1"

    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file" | awk '{print tolower($1)}'
    else
        shasum -a 256 "$file" | awk '{print tolower($1)}'
    fi
}

# 经指定路由下载校验清单，并拒绝缺少当前资产或哈希格式非法的响应。
download_checksum() {
    local prefix="$1"
    local url checksum
    local checksum_name="${BINARY_NAME}_${LATEST_VERSION#v}_checksums.txt"
    local github_url="$GITHUB_REPO_URL/releases/download/$LATEST_VERSION/$checksum_name"
    local manifest="$TEMP_DIR/$checksum_name"
    local partial="$manifest.part"

    rm -f -- "$partial" "$manifest"
    url="$(route_url "$prefix" "$github_url")"
    echo "Downloading release checksums via $(route_name "$prefix")..."
    if ! curl_download "$url" "$partial" "$METADATA_TIMEOUT"; then
        rm -f -- "$partial"
        echo "Failed to download release checksums via $(route_name "$prefix")." >&2
        return 1
    fi

    checksum="$(checksum_from_manifest "$partial")"
    if [[ -z "$checksum" ]]; then
        rm -f -- "$partial"
        echo "Checksum entry for $ASSET_NAME was not found via $(route_name "$prefix")." >&2
        return 1
    fi

    mv "$partial" "$manifest"
    EXPECTED_SHA256="$checksum"
}

# 检查归档目录中是否存在名称完全匹配的 ncmctl，拒绝错误页面和错误资产。
archive_contains_binary() {
    local archive="$1"

    tar -tzf "$archive" | awk -v binary="$BINARY_NAME" '
        $0 == binary { found = 1 }
        END { exit !found }
    '
}

# 将归档中的候选程序复制到安装目录临时文件后执行，兼容 TMPDIR 挂载 noexec 的青龙环境。
extract_and_validate() {
    local archive="$1"
    local prefix="$2"
    local staged_version
    local extracted_binary="$TEMP_DIR/$BINARY_NAME"

    rm -f -- "$extracted_binary"
    if ! tar -xzf "$archive" -C "$TEMP_DIR" "$BINARY_NAME"; then
        echo "Failed to extract $BINARY_NAME via $(route_name "$prefix")." >&2
        return 1
    fi
    if [[ ! -f "$extracted_binary" || -L "$extracted_binary" ]]; then
        echo "Extracted $BINARY_NAME is not a regular file via $(route_name "$prefix")." >&2
        return 1
    fi

    cp "$extracted_binary" "$INSTALL_TEMP" || die "Unable to stage $BINARY_NAME in $INSTALL_DIR"
    chmod 0755 "$INSTALL_TEMP" || die "Unable to make staged $BINARY_NAME executable"
    [[ -x "$INSTALL_TEMP" ]] || die "Staged $BINARY_NAME is not executable: $INSTALL_TEMP"
    if ! staged_version="$(binary_version "$INSTALL_TEMP")"; then
        echo "Downloaded binary cannot report its version via $(route_name "$prefix")." >&2
        return 1
    fi
    if [[ "${staged_version#v}" != "${LATEST_VERSION#v}" ]]; then
        echo "Downloaded binary version $staged_version does not match release $LATEST_VERSION via $(route_name "$prefix")." >&2
        return 1
    fi

    STAGED_BINARY="$INSTALL_TEMP"
}

# 逐路由下载校验清单和归档，并在同一路由内完成哈希、tar 及程序版本的全部校验。
download_archive() {
    local prefix url actual_sha256
    local github_url="$GITHUB_REPO_URL/releases/download/$LATEST_VERSION/$ASSET_NAME"
    local archive="$TEMP_DIR/$ASSET_NAME"
    local partial="$archive.part"

    for prefix in "${ROUTES[@]}"; do
        rm -f -- "$partial" "$archive"
        # 校验清单和归档必须经过同一路由；任一步失败或内容异常都切换到下一路由。
        if ! download_checksum "$prefix"; then
            continue
        fi
        url="$(route_url "$prefix" "$github_url")"
        echo "Downloading $ASSET_NAME via $(route_name "$prefix")..."
        if ! curl_download "$url" "$partial" "$DOWNLOAD_TIMEOUT"; then
            rm -f -- "$partial"
            echo "Download failed via $(route_name "$prefix")." >&2
            continue
        fi

        # 归档始终先写入 .part，完成 HTTP、哈希及 tar 结构校验后才改为正式临时文件名。
        actual_sha256="$(sha256_file "$partial")"
        if [[ "$actual_sha256" != "$EXPECTED_SHA256" ]]; then
            rm -f -- "$partial"
            echo "SHA-256 verification failed via $(route_name "$prefix")." >&2
            continue
        fi
        if ! archive_contains_binary "$partial"; then
            rm -f -- "$partial"
            echo "Archive does not contain $BINARY_NAME via $(route_name "$prefix")." >&2
            continue
        fi

        mv "$partial" "$archive"
        if extract_and_validate "$archive" "$prefix"; then
            return 0
        fi
        rm -f -- "$archive"
    done

    rm -f -- "$partial" "$archive"
    die "Unable to download a valid $ASSET_NAME from any configured route"
}

# 在确认 pgrep 明确报告“未运行”后，原子替换正式二进制；pgrep 自身错误不能按未运行处理。
install_binary() {
    local staged_binary="$1"
    local pgrep_status

    acquire_install_lock
    # 下载期间目标可能已被另一个任务升级；锁内重读并拒绝覆盖相同或更高版本。
    if is_up_to_date; then
        release_install_lock || die "Unable to release installation lock: $INSTALL_LOCK_DIR"
        return 0
    fi

    if pgrep -x "$BINARY_NAME" >/dev/null 2>&1; then
        die "$BINARY_NAME is currently running; stop it before upgrading"
    else
        pgrep_status=$?
    fi
    [[ "$pgrep_status" -eq 1 ]] || die "Unable to determine whether $BINARY_NAME is running (pgrep exit: $pgrep_status)"

    [[ "$staged_binary" == "$INSTALL_TEMP" && -f "$staged_binary" && -x "$staged_binary" ]] || die "Staged binary is not ready for installation"
    # 候选文件已位于目标目录，最终 mv 不跨文件系统，可避免升级中途留下半成品。
    mv -f "$staged_binary" "$BINARY_PATH"
    INSTALL_TEMP=""

    "$BINARY_PATH" --version
    release_install_lock || die "Unable to release installation lock: $INSTALL_LOCK_DIR"
    echo "$BINARY_NAME installed successfully at $BINARY_PATH (version: $LATEST_VERSION)."
}

# 按依赖检查、版本解析、完整性验证和原子安装的顺序编排升级流程。
main() {
    require_commands
    configure_routes
    map_architecture
    TEMP_DIR="$(mktemp -d "$TEMP_ROOT/ncmctl_upgrade.XXXXXX")"
    # 无论正常结束还是收到中断信号，都通过 EXIT trap 清理下载和安装临时文件。
    trap cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM

    get_latest_version
    if is_up_to_date; then
        return 0
    fi

    [[ -d "$INSTALL_DIR" ]] || die "Install directory does not exist: $INSTALL_DIR"
    INSTALL_TEMP="$(mktemp "$INSTALL_DIR/.${BINARY_NAME}.XXXXXX")" || die "Unable to create an installation staging file in $INSTALL_DIR"
    download_archive
    install_binary "$STAGED_BINARY"
}

# 离线回归测试会 source 本脚本复用函数，只有直接执行时才进入真实安装流程。
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
