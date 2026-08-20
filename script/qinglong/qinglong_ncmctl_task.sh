#!/usr/bin/env bash

# Copyright (c) 2024-2026 chaunsin
# SPDX-License-Identifier: MIT

# name: ncmctl一键任务执行
# cron: 0 10 * * *

########################################################################
# 注意:                                                                 #
#  1.需要提前安装`ncmctl`可执行文件                                        #
#  2.执行前需要保证登录状态,也就是走完登录流程                                #
#  3.已完成的任务，或不再需要执行的任务则建议关闭相应的任务，避免风控被风控。      #
#########################################################################

set -e

# 是否开启每日签到任务,默认开启
NCMCTL_QINGLONG_SIGN=${NCMCTL_QINGLONG_SIGN:-true}
# 每日签到任务是否自动领取奖励，默认关闭
NCMCTL_QINGLONG_SIGN_AUTOMATIC=${NCMCTL_QINGLONG_SIGN_AUTOMATIC:-false}
# 是否开启刷歌功能，默认关闭
NCMCTL_QINGLONG_SCROBBLE=${NCMCTL_QINGLONG_SCROBBLE:-false}
# 是否开启音乐合伙人签到功能，默认开启
NCMCTL_QINGLONG_PARTNER=${NCMCTL_QINGLONG_PARTNER:-true}

# 将变量值转换为小写
to_lower() {
  echo "$1" | tr '[:upper:]' '[:lower:]'
}

# 执行每日签到任务
if [[ "$(to_lower "${NCMCTL_QINGLONG_SIGN}")" == "true" ]]; then
  echo ">>> 执行每日签到任务 <<<"
  ncmctl sign "--automatic=$(to_lower "${NCMCTL_QINGLONG_SIGN_AUTOMATIC}")"
  echo "--- 执行每日签到任务完成 ---"
fi

# 执行刷歌任务,注意如果已经刷到了满级则需要关闭此功能，不然会出现封号风险。
if [[ "$(to_lower "${NCMCTL_QINGLONG_SCROBBLE}")" == "true" ]]; then
  echo ">>> 执行刷歌任务 <<<"
  ncmctl scrobble
  echo "--- 执行刷歌任务完成 ---"
fi

# 执行音乐合伙人签到任务,注意如果有没有此功能权限则设置为false，不然会出现错误。
if [[ "$(to_lower "${NCMCTL_QINGLONG_PARTNER}")" == "true" ]]; then
  echo ">>> 执行音乐合伙人签到任务 <<<"
  ncmctl partner
  echo "--- 执行音乐合伙人签到任务完成 ---"
fi

