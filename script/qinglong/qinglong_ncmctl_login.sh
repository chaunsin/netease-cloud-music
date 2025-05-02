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

# name: ncmctl登录
# cron: 0 0 1 1 *

set -e

# NCMCTL_QINGLONG_LOGIN_MODE 登录模式 [phone|qrcode|cookie|cookiecloud] 默认cookiecloud
NCMCTL_QINGLONG_LOGIN_MODE=${NCMCTL_QINGLONG_LOGIN_MODE:-cookiecloud}
# NCMCTL_QINGLONG_LOGIN_ACCOUNT 登录账号
NCMCTL_QINGLONG_LOGIN_ACCOUNT=${NCMCTL_QINGLONG_LOGIN_ACCOUNT:-''}
# NCMCTL_QINGLONG_LOGIN_PASSWORD 登录密码
NCMCTL_QINGLONG_LOGIN_PASSWORD=${NCMCTL_QINGLONG_LOGIN_PASSWORD:-''}
# NCMCTL_QINGLONG_LOGIN_COOKIE cookie模式时使用（文件路径或cookie字符串）
NCMCTL_QINGLONG_LOGIN_COOKIE=${NCMCTL_QINGLONG_LOGIN_COOKIE:-''}
# NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_SERVER cookiecloud模式时使用得服务器地址
NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_SERVER=${NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_SERVER:-''}
# NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_HEADERS cookiecloud模式时使用得请求头,逗号分隔键值对 eg: key1:value1,key2:value2
NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_HEADERS=${NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_HEADERS:-''}

login_args=("${NCMCTL_QINGLONG_LOGIN_MODE}")

case "${NCMCTL_QINGLONG_LOGIN_MODE}" in
    qrcode)
        ;;
    phone)
        if [[ -z "${NCMCTL_QINGLONG_LOGIN_ACCOUNT}" ]]; then
            echo "Error: Please set the environment variable for the account" >&2
            exit 1
        fi
        login_args+=("${NCMCTL_QINGLONG_LOGIN_ACCOUNT}")

        if [[ -n "${NCMCTL_QINGLONG_LOGIN_PASSWORD}" ]]; then
            login_args+=(-p "${NCMCTL_QINGLONG_LOGIN_PASSWORD}")
        fi
        ;;

    cookie)
        if [[ -f "${NCMCTL_QINGLONG_LOGIN_COOKIE}" ]]; then
            login_args+=(-f "${NCMCTL_QINGLONG_LOGIN_COOKIE}")
        elif [[ -n "${NCMCTL_QINGLONG_LOGIN_COOKIE}" ]]; then
            login_args+=("${NCMCTL_QINGLONG_LOGIN_COOKIE}")
        else
            echo "Error: Cookie value/file not provided" >&2
            exit 1
        fi
        ;;

    cookiecloud)
        if [[ -z "${NCMCTL_QINGLONG_LOGIN_ACCOUNT}" ]]; then
            echo "Error: Please set the environment variable for the cookiecloud user key uuid" >&2
            exit 1
        fi
        login_args+=(-u "${NCMCTL_QINGLONG_LOGIN_ACCOUNT}")

        if [[ -z "${NCMCTL_QINGLONG_LOGIN_PASSWORD}" ]]; then
            echo "Error: Please set the environment variable for the cookiecloud password" >&2
            exit 1
        fi
        login_args+=(-p "${NCMCTL_QINGLONG_LOGIN_PASSWORD}")

        if [[ -z "${NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_SERVER}" ]]; then
            echo "Error: Please set the environment variable for the cookiecloud server addr" >&2
            exit 1
        fi
        login_args+=(-s "${NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_SERVER}")

        if [[ -n "${NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_HEADERS}" ]]; then
            login_args+=(-H "${NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_HEADERS}")
        fi
        ;;

    *)
        echo "Error: Unsupported login mode: ${NCMCTL_QINGLONG_LOGIN_MODE}" >&2
        exit 1
        ;;
esac

# 获取用户信息
#ncmctl curl -m GetUserInfo

# 登录
echo "Executing: ncmctl login" "${login_args[@]}"
ncmctl login "${login_args[@]}"

