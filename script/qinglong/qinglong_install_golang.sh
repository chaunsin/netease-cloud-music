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

set -o pipefail

# Golang 默认版本
DEFAULT_GO_VERSION="1.21.0"
GO_INSTALL_DIR="/usr/local"
# https://go.dev/dl/go1.22.9.src.tar.gz
#SOURCE="https://go.dev/dl/"
# https://studygolang.com/dl/golang/go1.23.3.src.tar.gz
SOURCE="https://studygolang.com/dl/golang/"

# 检查当前系统类型和架构
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m | tr '[:upper:]' '[:lower:]')

    case $ARCH in
        armv*) ARCH="arm";;
        aarch64) ARCH="arm64";;
        x86_64) ARCH="amd64";;
        i386 | i686) ARCH="386";;
        loongarch64) ARCH="loong64";;
        *) echo "Unsupported architecture: $ARCH"; exit 1;;
    esac

    if [[ $OS == "darwin" || $OS == "linux" || $OS == "freebsd" || $OS == "openbsd" || $OS == "plan9" ]]; then
        echo "Detected platform: $OS-$ARCH"
    else
        echo "Unsupported platform: $OS"; exit 1
    fi
}

# 比较版本号 (>=)，只考虑三位版本号
version_ge() {
    # 使用 'IFS' 以 '.' 为分隔符拆分版本号
    IFS='.'  # 设置分隔符
    set -- $1  # 拆分第一个版本号
    ver1_major=$1
    ver1_minor=${2:-0}  # 默认为0
    ver1_patch=${3:-0}  # 默认为0

    set -- $2  # 拆分第二个版本号
    ver2_major=$1
    ver2_minor=${2:-0}  # 默认为0
    ver2_patch=${3:-0}  # 默认为0

    # 比较每一部分的版本号
    if [[ $ver1_major -gt $ver2_major ]]; then
        return 0  # ver1 >= ver2
    elif [[ $ver1_major -lt $ver2_major ]]; then
        return 1  # ver1 < ver2
    fi

    if [[ $ver1_minor -gt $ver2_minor ]]; then
        return 0  # ver1 >= ver2
    elif [[ $ver1_minor -lt $ver2_minor ]]; then
        return 1  # ver1 < ver2
    fi

    if [[ $ver1_patch -gt $ver2_patch ]]; then
        return 0  # ver1 >= ver2
    elif [[ $ver1_patch -lt $ver2_patch ]]; then
        return 1  # ver1 < ver2
    fi

    return 0  # ver1 == ver2
}

# 检查当前安装的 Go 版本
check_go_version() {
    if command -v go >/dev/null 2>&1; then
        INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        if version_ge "$INSTALLED_VERSION" "$DEFAULT_GO_VERSION"; then
            echo "Go is already installed and meets the version requirement (>= $DEFAULT_GO_VERSION)."
            exit 0
        else
            echo "Go version is outdated: $INSTALLED_VERSION. Installing $DEFAULT_GO_VERSION in parallel..."
        fi
    else
        echo "Go is not installed. Proceeding with installation..."
    fi
}

# 下载并安装 Golang
install_golang() {
    DOWNLOAD_URL="${SOURCE}go${DEFAULT_GO_VERSION}.${OS}-${ARCH}.tar.gz"
    TMP_DIR=$(mktemp -d)
    INSTALL_PATH="$GO_INSTALL_DIR/go${DEFAULT_GO_VERSION}"

    echo "Downloading Golang from $DOWNLOAD_URL..."
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/go.tar.gz"
    if [[ $? -ne 0 ]]; then
        echo "Failed to download Golang. Please check your network connection."
        exit 1
    fi

    echo "Extracting Golang to $INSTALL_PATH..."
    mkdir -p "$INSTALL_PATH"
    tar -C "$INSTALL_PATH" --strip-components=1 -xzf "$TMP_DIR/go.tar.gz"

    echo "Setting up Go environment for version $DEFAULT_GO_VERSION..."
    export PATH="$INSTALL_PATH/bin:$PATH"

    # 将 Go 环境变量添加到 .bashrc 或 .zshrc 文件
    if ! grep -q "$INSTALL_PATH/bin" "$HOME/.bashrc" 2>/dev/null; then
        echo "export PATH=\"$INSTALL_PATH/bin:\$PATH\"" >> "$HOME/.bashrc"
        echo "Added Go to .bashrc"
    fi

    # 如果是 Zsh 用户，检查并更新 .zshrc
    if [[ -n "$ZSH_VERSION" && ! -f "$HOME/.zshrc" ]]; then
        echo "export PATH=\"$INSTALL_PATH/bin:\$PATH\"" >> "$HOME/.zshrc"
        echo "Added Go to .zshrc"
    fi

    # 使用 source 或 . 来立即生效
    if [[ -f "$HOME/.bashrc" ]]; then
        source "$HOME/.bashrc"  # Bash 和 Zsh 支持
        echo "Sourced .bashrc to apply changes"
    elif [[ -f "$HOME/.zshrc" ]]; then
        source "$HOME/.zshrc"  # Bash 和 Zsh 支持
        echo "Sourced .zshrc to apply changes"
    elif [[ -f "$HOME/.profile" ]]; then
        . "$HOME/.profile"  # 适配 sh / dash
        echo "Sourced .profile to apply changes"
    fi

    echo "Go installation completed."
    "$INSTALL_PATH/bin/go" version
}

main() {
    detect_platform
    check_go_version
    install_golang
}

main