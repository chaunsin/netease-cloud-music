# 🎵 netease-cloud-music

[![GoDoc](https://godoc.org/github.com/chaunsin/netease-cloud-music?status.svg)](https://godoc.org/github.com/chaunsin/netease-cloud-music) [![Go Report Card](https://goreportcard.com/badge/github.com/chaunsin/netease-cloud-music)](https://goreportcard.com/report/github.com/chaunsin/netease-cloud-music) [![ci](https://github.com/chaunsin/netease-cloud-music/actions/workflows/ci.yml/badge.svg)](https://github.com/chaunsin/netease-cloud-music/actions/workflows/ci.yml) [![deploy image](https://github.com/chaunsin/netease-cloud-music/actions/workflows/deploy_image.yml/badge.svg)](https://github.com/chaunsin/netease-cloud-music/actions/workflows/deploy_image.yml)

> 🚀 网易云音乐 Golang API 接口 + 命令行工具套件 + 一键完成每日任务

---

## ⚠️ 重要声明

> **📅 2025-06-03 更新：**
> 目前风控极为严格，刷歌功能存在较高封号风险，不建议使用。如执意使用并收到 [非法挂机行为警告](https://github.com/chaunsin/netease-cloud-music/issues/34)
> ，请立即终止，否则后果自负！

- **本项目仅供个人学习使用，切勿用于商业用途或非法用途！**
- **使用本项目遇到封号等问题概不负责，使用前请谨慎考虑！**
- **如有侵权请联系删除！**
- **理性star，切勿盲目跟风！**

---

## ✨ 功能特性

命令行工具 (ncmctl) 提供以下功能

### 🔐 登录方式

- [x] 扫码登录
- [x] Cookie 方式登录
- [x] [CookieCloud](https://github.com/easychen/CookieCloud/blob/master/README_cn.md) 方式登录
- [x] ~~短信登录~~ (存在风控问题)
- [x] ~~手机号密码登录~~ (存在风控问题)

### 📋 每日任务

- [x] 一键完成每日任务（音乐合伙人、云贝签到、黑胶乐签、刷歌 300 首）
- [x] 每日推歌挑战赛, 支持自动发布动态、参与报名、抽奖(可配是否自动删除动态以及抽奖)
- [x] 云贝签到（支持自动领取签到奖励）
- [x] "音乐合伙人"自动测评
  - 5 首基础歌曲 + 2~7 首随机额外歌曲测评（不包含"歌曲推荐"测评）
  - 2025 年 3 月 [公告](https://music.163.com/#/event?id=30336457500&uid=7872690377) | [规则](https://y.music.163.com/g/yida/9fecf6a378be49a7a109ae9befb1b8d3)
- [x] 每日刷歌 300 首（支持去重功能）
- [x] 黑胶乐签（无需有效 VIP 权益）

### ☁️ 云盘功能

- [x] 云盘上传（支持并行批量上传）

### 🎶 音乐处理

- [x] 解密 `.ncm` 文件为 `.mp3`/`.flac` 可播放格式（支持并行批量解析）
- [x] 音乐下载，支持多种品质(标准、较高、极高、无损、Hi-Res)

### 🛠️ 调试工具

- [x] `crypto` 子命令 - 本地加密 WEAPI/EAPI/Linux API 参数；解密 EAPI 请求及 XEAPI 请求/响应
- [x] `curl` 子命令 - 按导出的 Go 方法名调用 API wrapper；是否需要登录及是否修改账号取决于具体接口
  - [ ] 支持动态链接请求
- [x] `proxy` 子命令 - 监控网易云音乐 HTTP(S) 接口请求与响应

### 🔜 计划中

- [ ] VIP 日常任务完成（待考虑）
- [ ] "音乐人"任务自动完成（待考虑）

---

## 💻 环境要求

| 依赖 | 版本要求 | 必需 |
| -------- | -------- | --- |
| Golang | = 1.25.0 | 是 |
| Makefile | - | 可选 |
| Git | - | 可选 |
| Docker | - | 可选 |

---

## 🔨 安装

### 方式一：下载预编译版本

直接从 [Releases](https://github.com/chaunsin/netease-cloud-music/releases) 页面下载对应平台的二进制文件。

### 方式二：源码安装

```shell
# 直接安装
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest

# 或者克隆后安装
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make install
```

> 📂 默认安装路径：`$GOPATH/bin`

### 方式三：Docker 安装

```shell
# Docker Hub
docker pull chaunsin/ncmctl:latest

# GitHub Container Registry
docker pull ghcr.io/chaunsin/ncmctl:latest
```

> 📖 Docker 使用文档：[https://hub.docker.com/r/chaunsin/ncmctl](https://hub.docker.com/r/chaunsin/ncmctl)

**自行编译镜像：**

```shell
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make build-image
```

> ⚠️ 自行编译需安装 Docker 环境，国内网络建议使用代理。

### 方式四：青龙面板

详见 👉 [青龙脚本安装指南](docs/qinglong.md)

---

## 🚀 使用指南

相关登录、任务执行、歌曲下载等参考: [usage](docs/usage.md)

---

## 📚 API 接口

本仓库提供了一些API接口,可引用此仓库作为sdk进行二次开发,覆盖以下接口内容

| 接口包 | 当前覆盖 |
| ------------ | -------------------------------------------------- |
| `api/weapi/` | 接口最完整，优先使用 |
| `api/eapi/` | 包含 PC/移动端接口，覆盖范围小于 WEAPI |
| `api/api/` | 仅有少量试验性 wrapper；通用 `CryptoModeAPI` 请求层尚不会序列化 `req` |
| `api/linux/` | 目前只有构造器，没有具体 endpoint wrapper |

> 注意: 接口后续可能出现重大变更，需斟酌风险。

### 接口示例

| 功能 | 示例文件 | 说明 |
| ---- | -------------------------------------------------------------------- | --- |
| 登录 | [example_login_test.go](example/example_login_test.go) | - |
| 云盘上传 | [example_cloud_upload_test.go](example/example_cloud_upload_test.go) | 需登录 |
| 音乐下载 | [example_download_test.go](example/example_download_test.go) | 需登录 |

这些示例都带有 `integration` build tag，会访问真实网易服务，并可能登录、上传、下载或写入本地文件。确认相应副作用后再运行，例如：

```bash
go test -tags=integration -v -run TestWeapiLoginByQrcode ./example/
```

---

## 🤖 AI 助手技能

本仓库提供也提供了两类职责不同的 skill：

- `skills/ncmctl/` 是可分发的 ncmctl 命令行使用指南，面向安装、登录、任务执行、命令参数等。
- `.claude/skills/ncmctl-dev/` 是代码开发指南，面向 Go 源码、测试、API/加密和代理实现。

### 安装技能

使用 `skills` 命令安装可分发的用户 skill：

```bash
npx skills add chaunsin/netease-cloud-music --skill ncmctl -g
```

安装后，向 AI 助手询问 ncmctl 的安装和使用问题时会触发用户 skill。

---

## ❤️ 致谢

### 贡献者

- [sjpqxuzdly03646](https://github.com/sjpqxuzdly03646) - "音乐合伙人"功能支持
- [stkevintan](https://github.com/stkevintan) - CookieCloud 登录方式

### 参考项目

- [NeteaseCloudMusicApi](https://github.com/Binaryify/NeteaseCloudMusicApi)
- [api-enhanced](https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced)
- [pyncm](https://github.com/mos9527/pyncm)
- [musicdump](https://github.com/naruto2o2o/musicdump)
- [crontab.guru](https://crontab.guru)

感谢所有依赖的开源项目以及贡献者！
