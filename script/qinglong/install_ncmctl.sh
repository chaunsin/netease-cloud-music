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

#!/bin/bash

# 脚本执行出错时立即退出
set -e

# 检查是否安装了Golang
if ! command -v go &>/dev/null; then
    echo "Golang 未安装，正在运行 install_golang.sh..."
    if [ -f "./install_golang.sh" ]; then
        bash ./install_golang.sh
    else
        echo "找不到 install_golang.sh 脚本，退出..."
        exit 1
    fi
else
    echo "Golang 已安装"
fi

# 检查是否安装了 Git
if ! command -v git &>/dev/null; then
    echo "Git 未安装，正在尝试安装..."

    # 根据操作系统选择安装方法
    if [ -f /etc/debian_version ]; then
        # Ubuntu/Debian 系列
        sudo apt update && sudo apt install -y git
    elif [ -f /etc/redhat-release ]; then
        # Red Hat/CentOS 系列
        sudo yum install -y git || sudo dnf install -y git
    elif [ -f /etc/arch-release ]; then
        # Arch Linux 系列
        sudo pacman -Syu --noconfirm git
    elif [ "$(uname)" == "Darwin" ]; then
        # macOS 系统
        if ! command -v brew &>/dev/null; then
            echo "Homebrew 未安装，请手动安装 Homebrew 然后重试"
            exit 1
        fi
        brew install git
    else
        echo "未知的系统类型，请手动安装 Git"
        exit 1
    fi
else
    echo "Git 已安装"
fi

# 拉取代码仓库
REPO_URL="https://github.com/chaunsin/netease-cloud-music"
CLONE_DIR="netease-cloud-music"

if [ ! -d "$CLONE_DIR" ]; then
    echo "正在从 $REPO_URL 克隆代码库..."
    git clone "$REPO_URL" "$CLONE_DIR"
else
    echo "代码库已存在，正在更新..."
    cd "$CLONE_DIR" && git pull && cd ..
fi

# 编译代码
echo "正在编译代码..."
BUILD_DIR="$CLONE_DIR/cmd/ncmctl"
OUTPUT_BIN="ncmctl"

if [ -f "$BUILD_DIR/main.go" ]; then
    cd "$BUILD_DIR"
    go build -o "$OUTPUT_BIN" main.go
    cd -
    echo "编译完成，生成的可执行文件为 $BUILD_DIR/$OUTPUT_BIN"
else
    echo "未找到 $BUILD_DIR/main.go 文件，退出..."
    exit 1
fi

# 将可执行文件放到全局可执行文件目录
GLOBAL_BIN_DIR="/usr/local/bin"

if [ ! -w "$GLOBAL_BIN_DIR" ]; then
    echo "需要权限将文件移动到 $GLOBAL_BIN_DIR，正在尝试使用 sudo..."
    sudo mv "$BUILD_DIR/$OUTPUT_BIN" "$GLOBAL_BIN_DIR/"
else
    mv "$BUILD_DIR/$OUTPUT_BIN" "$GLOBAL_BIN_DIR/"
fi

if [ -f "$GLOBAL_BIN_DIR/$OUTPUT_BIN" ]; then
    echo "$OUTPUT_BIN 已成功放置到 $GLOBAL_BIN_DIR 目录下"
else
    echo "无法将 $OUTPUT_BIN 放置到 $GLOBAL_BIN_DIR，退出..."
    exit 1
fi

echo "脚本执行完成，您可以通过执行 $OUTPUT_BIN 使用编译后的程序"
