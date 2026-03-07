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

---

## ✨ 功能特性

### 🖥️ 命令行工具 (ncmctl)

#### 🔐 登录方式

- [x] 📷 扫码登录
- [x] 🍪 Cookie 方式登录
- [x] ☁️ [CookieCloud](https://github.com/easychen/CookieCloud/blob/master/README_cn.md) 方式登录
- [x] ~~📱 短信登录~~ (存在风控问题)
- [x] ~~🔑 手机号密码登录~~ (存在风控问题)

#### 📋 每日任务

- [x] 🎯 一键完成每日任务（音乐合伙人、云贝签到、VIP 签到、刷歌 300 首）
- [x] 💰 云贝签到（支持自动领取签到奖励）
- [x] 🎤 "音乐合伙人"自动测评
    - 5 首基础歌曲 + 2~7 首随机额外歌曲测评（不包含"歌曲推荐"测评）
    - 📢 2025 年 3
      月 [公告](https://music.163.com/#/event?id=30336457500&uid=7872690377) | [规则](https://y.music.163.com/g/yida/9fecf6a378be49a7a109ae9befb1b8d3)
- [x] 🎧 每日刷歌 300 首（支持去重功能）
- [x] 💎 VIP 每日签到

#### ☁️ 云盘功能

- [x] ☁️ 云盘上传（支持并行批量上传）

#### 🎶 音乐处理

- [x] 🔓 解密 `.ncm` 文件为 `.mp3`/`.flac` 可播放格式（支持并行批量解析）
- [x] 📥 音乐下载，支持多种品质

|   品质   |         别名          | 说明      |
|:------:|:-------------------:|:--------|
|   标准   |  `standard`、`128`   | 128kbps |
|  高品质   |   `higher`、`192`    | 192kbps |
|   极高   | `exhigh`、`HQ`、`320` | 320kbps |
|   无损   |   `lossless`、`SQ`   | FLAC    |
| Hi-Res |    `hires`、`HR`     | 高解析度    |

#### 🛠️ 调试工具

- [x] 🔐 `crypto` 子命令 - 接口参数加解密，便于调试
- [x] 🌐 `curl` 子命令 - 调用网易云音乐 API，无需关心参数加解密
    - [ ] 支持动态链接请求

#### 🔜 计划中

- [ ] VIP 日常任务完成（待考虑）
- [ ] "音乐人"任务自动完成（待考虑）
- [ ] 🌐 Proxy 代理支持

### 📦 API 接口

|   类型    | 适用场景        |
|:-------:|:------------|
| `weapi` | 网页端、小程序（推荐） |
| `eapi`  | PC 端、移动端    |

> 💡 **提示：** 目前主要实现了 `weapi`
> ，接口相对较全，推荐使用。如需其他接口可提 [Issue](https://github.com/chaunsin/netease-cloud-music/issues)。

---

## 💻 环境要求

|    依赖    |   版本要求   |  必需  |
|:--------:|:--------:|:----:|
|  Golang  | \>= 1.24 |  ✅   |
| Makefile |    -     | ❌ 可选 |
|   Git    |    -     | ❌ 可选 |
|  Docker  |    -     | ❌ 可选 |

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

## 🚀 使用指南

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
> 1. 短信发送每日有限制，请勿频繁登录以免触发风控
> 2. 若长时间未收到短信，可能是运营商延迟，可尝试重新发送或稍后再试

---

#### 2️⃣ 手机号密码登录

需先在网易云音乐中开启手机号密码登录权限。

```shell
ncmctl login phone 188xxx8888 -p 123456
```

> ⚠️ 此方式可能触发 `8821 需要行为验证码验证` 错误，仅作备选方案。
>
> 🔒 **请勿泄露密码！**

---

#### 3️⃣ Cookie 登录

当正常登录失败时，Cookie 登录可作为保底方案。

可通过浏览器插件获取
Cookie，推荐 [Cookie Editor](https://chromewebstore.google.com/detail/cookie-editor/ookdjilphngeeeghgngjabigmpepanpl)。

```shell
# 方式一：直接导入 Cookie 字符串
ncmctl login cookie 'cookie字符串内容'

# 方式二：从文件导入
ncmctl login cookie -f cookie.txt
```

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
ncmctl login cookiecloud -u <用户名> -p <密码> -s http://0.0.0.0:8088
```

> ⚠️ **注意事项：**
> 1. 请确保服务端地址、账号、密码正确
> 2. 若出现 Cookie 找不到错误，请在插件中手动同步或重新登录后重试
> 3. 使用第三方服务器请自行评估安全风险

---

#### 5️⃣ ~~二维码登录~~（已弃用）

> ⚠️ 由于网易云风控严重，扫码登录会出现 `8821 需要行为验证码验证`
> 错误，暂不支持。详见 [Issue #26](https://github.com/chaunsin/netease-cloud-music/issues/26)

```shell
ncmctl login qrcode
```

</details>

---

### 📋 二、每日任务

**一键执行所有每日任务：**

```shell
ncmctl task
```

**默认包含的任务：**

|     任务     | 说明            |
|:----------:|:--------------|
|   `sign`   | 云贝签到 + VIP 签到 |
| `partner`  | 音乐合伙人         |
| `scrobble` | 刷歌 300 首      |

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
ncmctl task --scrobble.cron "0 20 * * *"
```

> 💡 **提示：**
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
> - 默认下载到 `./download` 目录，音质为无损 (SQ)
> - `--strict` 严格模式下，无指定品质则跳过；否则会降级下载

---

### ☁️ 四、云盘上传

```shell
# 上传单个文件
ncmctl cloud '/path/to/music.mp3'

# 批量上传目录
ncmctl cloud '/path/to/music/'
```

**参数说明：**

|  参数  | 默认值 | 最大值 | 说明    |
|:----:|:---:|:---:|:------|
| `-p` |  3  | 10  | 并发上传数 |

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

> ⚠️ 目录深度不能超过 3 层。

---

### 🛠️ 六、其他命令

```shell
# 查看帮助
ncmctl -h
```

---

## 📚 API 使用示例

|  功能  | 示例文件                                                                 | 说明  |
|:----:|:---------------------------------------------------------------------|:----|
|  登录  | [example_login_test.go](example/example_login_test.go)               | -   |
| 云盘上传 | [example_cloud_upload_test.go](example/example_cloud_upload_test.go) | 需登录 |
| 音乐下载 | [example_download_test.go](example/example_download_test.go)         | 需登录 |

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

|             命令              |  类型  | 说明                    |
|:---------------------------:|:----:|:----------------------|
|           `task`            |  服务  | 包含所有子命令，定时执行，适合部署到服务器 |
| `scrobble`/`sign`/`partner` | 单次任务 | 立即执行并返回结果             |

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