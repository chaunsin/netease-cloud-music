#!/usr/bin/env bash

################################################################################
# MIT License
#
# Copyright (c) 2024 chaunsin
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
#
################################################################################

# name: ncmctl安装
# cron: 0 1 * * *

set -o pipefail
set -e
set -u

# 安装路径
INSTALL_DIR="/usr/local/bin"
# 程序名称
BINARY_NAME="ncmctl"
# 完整路径
BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
# 仓库
REPO="chaunsin/netease-cloud-music"
# 临时目录
TEMP_DIR="/tmp/ncmctl_upgrade"
# 系统架构
ARCH="$(uname -m)"
# 系统类型
OS="$(uname -s)"
# 最新版本
#LATEST_VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
LATEST_VERSION=$(curl -s "https://gitee.com/api/v5/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

# 系统架构和下载文件映射
map_architecture() {
    case "$ARCH" in
        x86_64|amd64) ARCH="x86_64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l) ARCH="armv6" ;;
        mips64) ARCH="mips64" ;;
        mips64el) ARCH="mips64le" ;;
        ppc64le) ARCH="ppc64le" ;;
        riscv64) ARCH="riscv64" ;;
        loongarch64) ARCH="loong64" ;;
        i386 | i686) ARCH="i386" ;;
        s390x) ARCH="s390x" ;;
        *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
}

# 获取最新版本号
get_latest_version() {
    echo "Fetching latest version from GitHub..."
    if [[ -z "$LATEST_VERSION" ]]; then
        echo "Failed to fetch the latest version. Please check your network."
        exit 1
    fi
    echo "Latest version: $LATEST_VERSION"
}

# 检查是否已安装
is_installed() {
    if [[ -f "$BINARY_PATH" ]]; then
        echo "$BINARY_NAME is already installed at $BINARY_PATH."
        INSTALLED_VERSION=$($BINARY_PATH --version 2>/dev/null | awk '{print $NF}')
        if [[ "$INSTALLED_VERSION" == "$LATEST_VERSION" ]]; then
            echo "$BINARY_NAME is up-to-date (version: $INSTALLED_VERSION). No need to upgrade."
            exit 0
        else
            echo "Installed version: $INSTALLED_VERSION. A newer version ($LATEST_VERSION) is available."
        fi
    fi
}

# 下载和解压程序
download_and_extract() {
    echo "Downloading the latest version..."
    #local download_url="https://github.com/$REPO/releases/download/$LATEST_VERSION/${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
    local download_url="https://gitee.com/$REPO/releases/download/$LATEST_VERSION/${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
    local retry_count=0
    local tar_file="$TEMP_DIR/$BINARY_NAME.tar.gz"

    echo "Download URL: $download_url"
    mkdir -p "$TEMP_DIR"

    # 带重试的下载
    while (( retry_count < MAX_RETRIES )); do
        if curl -LfS --progress-bar "$download_url" -o "$tar_file"; then
            echo "Download successful."
            break
        else
            retry_count=$((retry_count + 1))
            echo "Download failed. Retrying ($retry_count/$MAX_RETRIES)..."
            sleep 2
        fi
    done

    if (( retry_count >= MAX_RETRIES )); then
        echo "Download failed after $MAX_RETRIES attempts. Exiting."
        exit 1
    fi

    # 检查文件是否合法
    if ! tar -tzf "$tar_file" &>/dev/null; then
        echo "Downloaded file is corrupted or invalid. Verify the URL: $download_url"
        exit 1
    fi

    echo "Extracting files..."
    tar -xzf "$tar_file" -C "$TEMP_DIR" || { echo "Extraction failed. Exiting."; exit 1; }
}

# 安装程序
install_binary() {
    # 检查当前是否有运行中的程序实例
    if pgrep -x "$BINARY_NAME" > /dev/null; then
        echo "Warning: $BINARY_NAME is currently running. Proceeding with cautious upgrade."
        exit 1;
    fi
    echo "Installing $BINARY_NAME..."
    mv "$TEMP_DIR/$BINARY_NAME" "$BINARY_PATH" || { echo "Installation failed. Exiting."; exit 1; }
    chmod +x "$BINARY_PATH"
    # 测试安装情况
    $BINARY_PATH --version
    echo "$BINARY_NAME installed successfully at $BINARY_PATH (version: $LATEST_VERSION)."
}

# 清理临时文件
cleanup() {
    echo "Cleaning up temporary files..."
    rm -rf "$TEMP_DIR"
}

# 主函数
main() {
    map_architecture
    # 获取最新版本号
    get_latest_version
    # 检查是否已安装
    is_installed
    # 下载和解压程序
    download_and_extract
    # 安装程序
    install_binary
    # 清理临时文件
    cleanup
}

main

