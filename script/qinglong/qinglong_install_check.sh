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

# 检查命令是否可用的函数
check_command() {
    command -v "$1" >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo "错误: $1 未安装，无法使用。"
        exit 1
    fi
}

# 检测系统类型并选择包管理器
detect_package_manager() {
    if [ -f /etc/debian_version ]; then
        echo "检测到系统: Debian/Ubuntu"
        PACKAGE_MANAGER="apt"
    elif [ -f /etc/redhat-release ]; then
        echo "检测到系统: Red Hat/CentOS"
        if command -v yum >/dev/null 2>&1; then
            PACKAGE_MANAGER="yum"
        elif command -v dnf >/dev/null 2>&1; then
            PACKAGE_MANAGER="dnf"
        fi
    elif [ -f /etc/arch-release ]; then
        echo "检测到系统: Arch Linux"
        PACKAGE_MANAGER="pacman"
    elif [ -f /etc/alpine-release ]; then
        echo "检测到系统: Alpine Linux"
        PACKAGE_MANAGER="apk"
    elif grep -i "photon" /etc/os-release >/dev/null 2>&1; then
        echo "检测到系统: VMware Photon OS"
        PACKAGE_MANAGER="tdnf"
    elif grep -i "amazon linux" /etc/os-release >/dev/null 2>&1; then
        echo "检测到系统: Amazon Linux"
        PACKAGE_MANAGER="yum"
    elif [ "$(uname)" == "Darwin" ]; then
        echo "检测到系统: macOS"
        if ! command -v brew &>/dev/null; then
            echo "错误: Homebrew 未安装，请手动安装 Homebrew 然后重试。"
            exit 1
        fi
        PACKAGE_MANAGER="brew"
    elif [ -f /etc/os-release ]; then
        echo "检测到未知的 Linux 系统，尝试使用常见包管理器..."
        if command -v apt >/dev/null 2>&1; then
            PACKAGE_MANAGER="apt"
        elif command -v yum >/dev/null 2>&1; then
            PACKAGE_MANAGER="yum"
        elif command -v apk >/dev/null 2>&1; then
            PACKAGE_MANAGER="apk"
        elif command -v pacman >/dev/null 2>&1; then
            PACKAGE_MANAGER="pacman"
        else
            echo "无法确定包管理器，请手动安装必要工具后重试。"
            exit 1
        fi
    else
        echo "未知的系统类型，请手动安装必要工具后重试。"
        exit 1
    fi
}

# 安装工具函数，根据系统包管理器
install_dependencies() {
    local package=$1
    case "$PACKAGE_MANAGER" in
    apt)
        if command -v sudo >/dev/null 2>&1; then
            sudo apt update && sudo apt install -y "$package"
        else
            apt update && apt install -y "$package"
        fi
        ;;
    yum)
        sudo yum install -y "$package"
        ;;
    dnf)
        sudo dnf install -y "$package"
        ;;
    pacman)
        sudo pacman -Syu --noconfirm "$package"
        ;;
    apk)
        sudo apk add --no-cache "$package"
        ;;
    tdnf)
        sudo tdnf install -y "$package"
        ;;
    brew)
        brew install "$package"
        ;;
    *)
        echo "未知的包管理器，无法自动安装 $package"
        exit 1
        ;;
    esac
}

# 主逻辑函数
main() {
    # 检测系统类型和包管理器
    detect_package_manager

    # 检查所需命令并尝试安装
    REQUIRED_COMMANDS=("grep" "mkdir" "tar" "curl" "mktemp")
    for cmd in "${REQUIRED_COMMANDS[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            echo "检测到 $cmd 未安装，尝试安装..."
            install_dependencies "$cmd"
        fi
    done

    echo "所有必要命令均已安装，可以继续执行。"
}

# 执行主程序
main
