# 🎵 netease-cloud-music

[![GoDoc](https://godoc.org/github.com/chaunsin/netease-cloud-music?status.svg)](https://godoc.org/github.com/chaunsin/netease-cloud-music) [![Go Report Card](https://goreportcard.com/badge/github.com/chaunsin/netease-cloud-music)](https://goreportcard.com/report/github.com/chaunsin/netease-cloud-music) [![ci](https://github.com/chaunsin/netease-cloud-music/actions/workflows/ci.yml/badge.svg)](https://github.com/chaunsin/netease-cloud-music/actions/workflows/ci.yml) [![deploy image](https://github.com/chaunsin/netease-cloud-music/actions/workflows/deploy_image.yml/badge.svg)](https://github.com/chaunsin/netease-cloud-music/actions/workflows/deploy_image.yml)

> 🚀 网易云音乐 Golang API 接口 + 命令行工具套件 + 一键完成每日任务

---

## ⚠️ 重要声明

> **📅 2025-06-03 更新：**
> 目前风控极为严格，刷歌功能存在较高封号风险，不建议使用。如执意使用并收到 [非法挂机行为警告](https://github.com/chaunsin/netease-cloud-music/issues/34)
> ，请立即终止，否则后果自负！

- 🚫 **本项目仅供个人学习使用，切勿用于商业用途或非法用途！**
- ⚖️ **使用本项目遇到封号等问题概不负责，使用前请谨慎考虑！**
- 📧 **如有侵权请联系删除！**
- ⭐️ **理性star，切勿盲目跟风！**

---

## ✨ 功能特性

### 🖥️ 命令行工具 (ncmctl)

#### 🔐 登录方式

- [X] 📷 扫码登录
- [X] 🍪 Cookie 方式登录
- [X] ☁️ [CookieCloud](https://github.com/easychen/CookieCloud/blob/master/README_cn.md) 方式登录
- [X] ~~📱 短信登录~~ (存在风控问题)
- [X] ~~🔑 手机号密码登录~~ (存在风控问题)

#### 📋 每日任务

- [X] 🎯 一键完成每日任务（音乐合伙人、云贝签到、VIP 签到、刷歌 300 首）
- [X] 💰 云贝签到（支持自动领取签到奖励）
- [X] 🎤 "音乐合伙人"自动测评
  - 5 首基础歌曲 + 2~7 首随机额外歌曲测评（不包含"歌曲推荐"测评）
  - 📢 2025 年 3
    月 [公告](https://music.163.com/#/event?id=30336457500&uid=7872690377) | [规则](https://y.music.163.com/g/yida/9fecf6a378be49a7a109ae9befb1b8d3)
- [X] 🎧 每日刷歌 300 首（支持去重功能）
- [X] 💎 VIP 每日签到

#### ☁️ 云盘功能

- [X] ☁️ 云盘上传（支持并行批量上传）

#### 🎶 音乐处理

- [X] 🔓 解密 `.ncm` 文件为 `.mp3`/`.flac` 可播放格式（支持并行批量解析）
- [X] 📥 音乐下载，支持多种品质

|  品质  |            别名            | 说明     |
| :----: | :-------------------------: | :------- |
|  标准  |    `standard`、`128`    | 128kbps  |
| 高品质 |     `higher`、`192`     | 192kbps  |
|  极高  | `exhigh`、`HQ`、`320` | 320kbps  |
|  无损  |    `lossless`、`SQ`    | FLAC     |
| Hi-Res |      `hires`、`HR`      | 高解析度 |

#### 🛠️ 调试工具

- [X] 🔐 `crypto` 子命令 - 本地加密 WEAPI/EAPI/Linux API 参数；直接解密当前仅支持 EAPI
- [X] 🌐 `curl` 子命令 - 按导出的 Go 方法名调用 API wrapper；是否需要登录及是否修改账号取决于具体接口
  - [ ] 支持动态链接请求
- [X] 🔎 `proxy` 子命令 - 监控网易云音乐 HTTP(S) 接口请求与响应

#### 🔜 计划中

- [ ] VIP 日常任务完成（待考虑）
- [ ] "音乐人"任务自动完成（待考虑）

### 📦 API 接口覆盖

| 接口包 | 当前覆盖 |
| :--- | :--- |
| `api/weapi/` | 接口最完整，优先使用 |
| `api/eapi/` | 包含 PC/移动端接口，覆盖范围小于 WEAPI |
| `api/api/` | 仅有少量试验性 wrapper；通用 `CryptoModeAPI` 请求层尚不会序列化 `req` |
| `api/linux/` | 目前只有构造器，没有具体 endpoint wrapper |

### 请求加密模式

| 模式 | 当前边界 |
| :--- | :--- |
| `weapi` | 默认模式，请求加密、响应明文 JSON |
| `eapi` | 请求加密；通用客户端目前只直接处理明文 JSON 响应，不会透明解密 `e_r=true` 响应 |
| `api` | 不加密；通用请求参数序列化尚未完成 |
| `linux` | Linux API 的请求/响应加解密，暂无高级 endpoint wrapper |
| `xeapi` | 底层 Aegis/XEAPI 封装，暂无独立 endpoint wrapper，CLI `curl -k` 也不支持；默认 POST 路径只有本地测试验证 |

> 💡 **提示：** XEAPI 的研究背景见 [docs/xeapi.md](docs/xeapi.md)，实际行为以源码和外部协议证据一起验证。如需新增接口可提
> [Issue](https://github.com/chaunsin/netease-cloud-music/issues)。

---

## 💻 环境要求

|   依赖   |  版本要求  |  必需  |
| :------: | :--------: | :-----: |
|  Golang  | \>= 1.25.0 |   是   |
| Makefile |     -     | 可选 |
|   Git   |     -     | 可选 |
|  Docker  |     -     | 可选 |

---

## 🔨 安装指南

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

> 📖 Docker 使用文档：https://hub.docker.com/r/chaunsin/ncmctl

**自行编译镜像：**

```shell
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make build-image
```

> ⚠️ 自行编译需安装 Docker 环境，国内网络建议使用代理。

### 方式四：青龙面板

详见 👉 [青龙脚本安装指南](docs/qinglong.md)

---

## 🤖 AI 助手技能

仓库提供两类职责不同的 skill：`skills/ncmctl/` 是可分发的 ncmctl 使用指南，面向安装、登录、命令参数和安全边界；`.claude/skills/ncmctl-dev/` 是仓库本地开发指南，面向 Go 源码、测试、API/加密和代理实现。仓库级规则由 `AGENTS.md` 提供，它链接到唯一事实来源 `CLAUDE.md`。

### 技能内容

| 文件 | 说明 |
| ---- | ---- |
| `skills/ncmctl/SKILL.md` | 用户 skill 入口、任务路由、命令速查和安全边界 |
| `skills/ncmctl/references/install-and-login.md` | 安装、升级、5 种登录流程、退出和故障排查 |
| `skills/ncmctl/references/commands.md` | 当前命令参数、配置结构、示例和能力限制 |
| `.claude/skills/ncmctl-dev/SKILL.md` | 仓库开发 skill 入口和渐进式参考路由 |
| `CLAUDE.md` / `AGENTS.md` | 仓库架构、开发规则、测试副作用和完成检查 |

### 安装技能

使用 `skills` 命令安装可分发的用户 skill：

```bash
npx skills add chaunsin/netease-cloud-music --skill ncmctl -g
```

也可将 `skills/ncmctl/` 复制到 AI 助手的技能目录：

```bash
# Claude Code
mkdir -p ~/.claude/skills
cp -r skills/ncmctl ~/.claude/skills/

# Codex
mkdir -p ~/.codex/skills
cp -r skills/ncmctl ~/.codex/skills/
```

安装后，向 AI 助手询问 ncmctl 的安装和使用问题时会触发用户 skill。参与本仓库开发时无需安装该副本，应使用仓库自带的 `AGENTS.md` 和 `ncmctl-dev` skill。

---

## 🚀 使用指南

### 命令速查

当前构建的 `ncmctl <command> --help` 是命令语法、默认值和限制的事实来源。位置参数和输出参数不要混用；例如 `ncm` 的所有位置参数都是输入，输出目录必须通过 `-o`/`--output` 指定。

| 命令 | 登录 | 用途与主要影响 |
| :--- | :---: | :--- |
| `ncmctl login <method>` | 否 | 通过手机、Cookie、CookieCloud 或二维码登录，并持久化 Cookie |
| `ncmctl logout` | 已有会话 | 调用远端退出接口并删除默认持久化 Cookie |
| `ncmctl task [flags]` | 是 | 按 cron 长期调度 `sign`、`partner`、`scrobble`；无选择器时调度全部任务 |
| `ncmctl sign [flags]` | 是 | 立即执行一次云贝及符合条件的 VIP 签到 |
| `ncmctl partner [flags]` | 是 | 立即上报播放并提交音乐合伙人测评，会修改账号状态 |
| `ncmctl scrobble [flags]` | 是 | 提交播放日志并在本地去重，封号风险较高 |
| `ncmctl download <id-or-url> [id-or-url...]` | 是 | 下载歌曲、专辑、歌手或歌单，并在 MD5 校验后写入本地文件 |
| `ncmctl cloud <file-or-directory>` | 是 | 上传一个本地音乐文件或递归扫描一个目录，修改账号云盘 |
| `ncmctl ncm <input> [input...]` | 否 | 本地解密一个或多个 `.ncm` 文件/目录；使用 `--output` 指定输出目录 |
| `ncmctl crypto <encrypt-or-decrypt>` | 否 | 本地调试旧版 API 加密格式；直接请求解密当前仅支持 EAPI |
| `ncmctl curl [method]` | 取决于接口 | 按导出的 Go API 方法名发起真实请求，登录要求和副作用由所选接口决定 |
| `ncmctl proxy [flags]` | 否 | 启动 HTTP(S) 代理并管理本地 CA，默认脱敏捕获网易相关流量 |
| `ncmctl completion <shell>` | 否 | 将 bash、fish、PowerShell 或 zsh 补全脚本写到标准输出 |

全局 `--debug` 会把已脱敏的运行诊断和网络元数据写到 stderr 及配置的滚动日志文件；请求/响应正文会省略，Cookie、Authorization 等非安全请求头会脱敏。调试日志仍可能包含接口路径、资源 ID 和本地文件路径，请按敏感运行数据保护。

### 📱 一、登录

支持 5 种登录方式，详情如下：

<details>
<summary>🔐 点击展开登录方式详情</summary>

#### 1️⃣ 短信登录

```shell
ncmctl login phone 188xxx8888
```

成功发送短信后会提示：

```shell
send sms success
please input sms captcha:
```

输入收到的短信验证码即可完成登录。

> ⚠️ **注意事项：**
>
> 1. 短信发送每日有限制，请勿频繁登录以免触发风控
> 2. 若长时间未收到短信，可能是运营商延迟，可尝试重新发送或稍后再试
> 3. `--timeout` 是登录网络请求的截止时间，不能中断终端中正在等待的验证码输入

---

#### 2️⃣ 手机号密码登录

需先在网易云音乐中开启手机号密码登录权限。

```shell
ncmctl login phone 188xxx8888 -p 123456
```

> ⚠️ 此方式可能触发 `8821 需要行为验证码验证` 错误，仅作备选方案。
>
> 🔒 当前命令通过 `-p` 参数接收密码，没有隐藏式密码输入；参数可能出现在 shell 历史和进程列表中。请勿在不可信环境使用或泄露密码。

---

#### 3️⃣ Cookie 登录

当正常登录失败时，Cookie 登录可作为保底方案。

可通过浏览器插件获取
Cookie，推荐 [Cookie Editor](https://chromewebstore.google.com/detail/cookie-editor/ookdjilphngeeeghgngjabigmpepanpl)。

```shell
# 方式一：直接导入 Cookie 字符串
ncmctl login cookie 'MUSIC_U=<浏览器导出的值>; __csrf=<浏览器导出的值>'

# 方式二：从文件导入
ncmctl login cookie -f cookie.txt
```

> 🔒 Cookie 字符串可能进入 shell 历史和进程参数。优先使用权限为 `0600` 的文件并通过 `-f` 导入。

**支持的文件格式：**

- `header` 格式
- `json` 格式
- [netscape 格式](https://docs.cyotek.com/cyowcopy/1.10/netscapecookieformat.html)

> 📖 详细说明请查看 `ncmctl login cookie -h`

---

#### 4️⃣ CookieCloud 登录

[CookieCloud](https://github.com/easychen/CookieCloud/blob/master/README_cn.md) 是一款浏览器 Cookie 管理插件，支持自动同步
Cookie 到云端并加密存储。

**操作流程：**

1. 📥 安装 CookieCloud 浏览器插件
2. ⚙️ 完成插件配置
3. 🎵 在网页端登录网易云音乐
4. 🔄 点击【手动同步】按钮同步到云端
5. 🖥️ 执行登录命令

```shell
ncmctl login cookiecloud -u <UUID> -p <密码> -s http://127.0.0.1:8088
```

> ⚠️ **注意事项：**
>
> 1. 请确保服务端地址、账号、密码正确
> 2. 若出现 Cookie 找不到错误，请在插件中手动同步或重新登录后重试
> 3. 使用第三方服务器请自行评估安全风险
> 4. 当前命令要求通过 `-u`、`-p` 传入凭据，没有内置交互式密码提示或专用凭据环境变量；参数可能出现在 shell 历史和进程列表中

---

#### 5️⃣ 二维码登录

使用手机网易云音乐 App 扫码登录。

```shell
ncmctl login qrcode
```

运行命令后：

1. 在当前目录生成二维码图片（`qrcode.png`）
2. 二维码同时会打印在终端中
3. 打开手机网易云音乐 App，扫描该二维码
4. 在手机上确认登录
5. 登录自动完成

| 参数              | 默认值 | 说明                                          |
| ----------------- | ------ | --------------------------------------------- |
| `-t, --timeout` | 5m     | 登录超时时间                                  |
| `-d, --dir`     | `./` | 二维码图片输出目录                            |
| `-l, --level`   | 1      | 二维码容错等级：0→7%、1→15%、2→25%、3→30% |

**二维码状态码说明：**

| 状态码 | 含义                 |
| ------ | -------------------- |
| 800    | 二维码已过期或已取消 |
| 801    | 等待扫码             |
| 802    | 已扫码，等待确认     |
| 803    | 授权登录成功         |

> 💡 若二维码过期（状态码 800），重新运行命令即可生成新的二维码。

</details>

---

### 📋 二、每日任务

**注册所有每日任务并持续运行调度服务：**

```shell
ncmctl task
```

**默认包含的任务：**

|     任务     | 说明                |
| :----------: | :------------------ |
|   `sign`   | 云贝签到 + VIP 签到 |
| `partner` | 音乐合伙人          |
| `scrobble` | 刷歌 300 首         |

**选择性执行任务：**

```shell
# 仅执行签到
ncmctl task --sign

# 执行签到和刷歌（无音乐合伙人资格时）
ncmctl task --sign --scrobble
```

**自定义执行时间：**

```shell
# 设置刷歌任务在每天 20:00 执行
ncmctl task --scrobble --scrobble.cron "0 20 * * *"
```

> 💡 **提示：**
>
> - 需要先登录
> - 本命令以服务方式持续运行，退出请按 `Ctrl+C`
> - 采用标准 [crontab](https://zh.wikipedia.org/wiki/Cron) 表达式，[在线编写工具](https://crontab.guru/)

> ⚠️ **警告：** 签到任务默认关闭自动领取奖励功能（存在封号风险），如需开启请添加 `--sign.automatic` 参数。

---

### 📥 三、音乐下载

#### 下载单曲

```shell
# 通过分享链接下载 Hi-Res 品质
ncmctl download -l hires 'https://music.163.com/song?id=1820944399'

# 通过歌曲 ID 下载
ncmctl download -l hires '1820944399'

# 下载无损品质到指定目录
ncmctl download -l SQ 'https://music.163.com/song?id=1820944399' -o ./download/
```

#### 批量下载

```shell
# 下载整张专辑（并发数 5，最大 20）
ncmctl download -p 5 'https://music.163.com/#/album?id=34608111'

# 下载歌手所有歌曲（严格模式：无对应品质则跳过）
ncmctl download --strict 'https://music.163.com/#/artist?id=33400892'

# 下载歌单
ncmctl download 'https://music.163.com/playlist?id=593617579'
```

> 💡 **提示：**
>
> - 默认下载到 `./download` 目录，音质为无损 (SQ)
> - `--strict` 严格模式下，无指定品质则跳过；否则会降级下载
> - 历史 `download --tag` 参数仅为兼容保留，当前不写入音频标签

---

### ☁️ 四、云盘上传

```shell
# 上传单个文件
ncmctl cloud '/path/to/music.mp3'

# 批量上传目录
ncmctl cloud '/path/to/music/'
```

**参数说明：**

|  参数  | 默认值 | 最大值 | 说明       |
| :----: | :----: | :----: | :--------- |
| `-p` |   3   |   10   | 并发上传数 |

> ⚠️ 目录深度不能超过 3 层。更多过滤条件请查看 `ncmctl cloud -h`。

---

### 🔓 五、NCM 文件解密

将加密的 `.ncm` 文件转换为可播放的 `.mp3`/`.flac` 格式。

```shell
# 批量解析目录
ncmctl ncm '/path/to/ncm/files' -o ./output

# 设置并发数
ncmctl ncm '/path/to/ncm/files' -o ./output -p 10
```

> ℹ️ 所有位置参数都会被当作输入路径扫描；输出目录只能通过 `-o`/`--output` 指定。例如输出到当前目录应使用 `ncmctl ncm '/path/to/ncm/files' -o .`。
>
> 不存在的路径或显式传入的非 `.ncm` 文件会在创建输出目录前报错退出。
>
> ⚠️ 目录深度不能超过 3 层。音频标签默认写入；历史参数 `--tag` 的语义是关闭标签写入，而不是开启。

---

### 🌐 六、HTTP(S) 接口监控代理

`proxy` 子命令用于调试自己设备上的网易云音乐流量。它只记录网易相关域名，其他 HTTP(S) 流量正常转发但不会输出。请求和响应会使用同一会话 ID 分块打印到终端，默认对 Cookie、Token、手机号、邮箱、设备标识和密码等敏感字段脱敏；无法安全结构化脱敏的正文只输出摘要。

```shell
# 默认只监听本机 127.0.0.1:9000
ncmctl proxy

# 将抓包正文保存到文件；启动提示和错误仍显示在终端
ncmctl proxy > capture.log

# 允许局域网设备连接（仅限可信网络）
ncmctl proxy --listen 0.0.0.0:9000

# 使用已有 CA，证书和私钥必须同时提供
ncmctl proxy --ca-cert ./ca.crt --ca-key ./ca.key

# 改变生成 CA 和运行数据的根目录
ncmctl --home /srv/ncmctl proxy
```

首次启动会生成用户专属 CA：

- 证书：`<home>/.ncmctl/proxy/ca.crt`
- 私钥：`<home>/.ncmctl/proxy/ca.key`

其中 `<home>` 是全局 `--home` 值，默认是操作系统用户主目录。启动信息会在 stderr 打印证书路径和 SHA-256 指纹。将 `ca.crt` 安装并设为受信任证书后，再把网易云客户端或设备的 HTTP/HTTPS 代理设置为监听地址；程序不会自动修改系统信任库。请妥善保护 `ca.key`，不要复制到其他设备或提交到代码仓库。

| 参数 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `--listen` | `127.0.0.1:9000` | 代理监听地址 |
| `--ca-cert` | 自动生成 | 已有 CA 证书路径，必须与 `--ca-key` 同时使用 |
| `--ca-key` | 自动生成 | 已有 CA 私钥路径，必须与 `--ca-cert` 同时使用 |
| `--max-body` | `1MB` | 单个请求或响应最多打印的正文大小，不影响真实转发 |
| `--show-sensitive` | `false` | 关闭脱敏并打印凭据等敏感值，请谨慎使用 |
| 全局 `--debug` | `false` | 打印代理内部连接诊断 |

> ⚠️ **安全与兼容性说明：**
>
> - `0.0.0.0` 会向局域网开放无认证代理，只应在可信网络和防火墙保护下临时使用。
> - HTTPS 监控依赖客户端信任生成的 CA；证书固定、Android 用户 CA 限制、QUIC/HTTP3 或绕过系统代理的连接可能无法捕获。
> - 首版按 CONNECT/Host 域名筛选目标；客户端若以 IP 地址作为 CONNECT 目标，即使 TLS SNI 是网易域名，也可能只会透明转发而不记录。
> - WEAPI 的随机请求密钥和新版 XEAPI 的会话密钥无法由被动代理完整还原，程序会标记为不支持并打印脱敏后的原始字段；EAPI、Linux API 和明文 API 会尽可能解析。
> - 音视频、图片、multipart 以及未知长度的流式请求正文只打印摘要；当前不解析 WebSocket 帧。
> - 输出端被慢终端、FIFO 或磁盘阻塞时，代理会优先保持真实流量可用；有界记录队列满时会输出 `CAPTURE_DROPPED` 标记，表示部分捕获块未写出。

按 `Ctrl+C` 可平滑停止代理。

### 🛠️ 七、其他命令

```shell
# 查看帮助
ncmctl -h

# 生成 shell 补全（以 zsh 为例）
ncmctl completion zsh
```

---

## 📚 API 使用示例

|   功能   | 示例文件                                                          | 说明   |
| :------: | :---------------------------------------------------------------- | :----- |
|   登录   | [example_login_test.go](example/example_login_test.go)               | -      |
| 云盘上传 | [example_cloud_upload_test.go](example/example_cloud_upload_test.go) | 需登录 |
| 音乐下载 | [example_download_test.go](example/example_download_test.go)         | 需登录 |

这些示例都带有 `integration` build tag，会访问真实网易服务，并可能登录、上传、下载或写入本地文件。确认相应副作用后再运行，例如：

```bash
go test -tags=integration -v -run TestWeapiLoginByQrcode ./example/
```

---

## ❓ 常见问题

### Q1: 下载无损音乐品质不准确？

当指定 `-l lossless` 时，可能会下载到 Hi-Res 品质。若歌曲不支持 Hi-Res，则会正常下载无损。此问题仍在排查中。

### Q2: 每日刷歌为什么达不到 300 首？

`scrobble` 支持去重功能，会在 `$HOME/.ncmctl/database/` 记录已听歌曲。

**可能原因：**

1. 使用本程序前已听过的歌曲未记录，导致重复播放不计数
2. Top 榜单歌曲数量有限，新歌更新不及时

> ⚠️ **建议：不要清理 `$HOME/.ncmctl/database/` 目录下的数据！**

### Q3: `task` 和 `scrobble`、`sign`、`partner` 子命令有什么区别？

|               命令               |   类型   | 说明                                       |
| :-------------------------------: | :------: | :----------------------------------------- |
|             `task`             |   服务   | 包含所有子命令，定时执行，适合部署到服务器 |
| `scrobble`/`sign`/`partner` | 单次任务 | 立即执行并返回结果                         |

---

## ❤️ 致谢

### 👥 贡献者

- [sjpqxuzdly03646](https://github.com/sjpqxuzdly03646) - "音乐合伙人"功能支持
- [stkevintan](https://github.com/stkevintan) - CookieCloud 登录方式

### 📦 参考项目

- [NeteaseCloudMusicApi](https://github.com/Binaryify/NeteaseCloudMusicApi)
- [pyncm](https://github.com/mos9527/pyncm)
- [musicdump](https://github.com/naruto2o2o/musicdump)
- [crontab.guru](https://crontab.guru)

感谢所有依赖的开源项目！
