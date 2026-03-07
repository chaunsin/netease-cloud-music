# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

网易云音乐 Golang API 接口 + 命令行工具套件 (ncmctl)。提供网易云音乐 API 的 Go 封装，支持登录、每日任务、云盘上传、NCM 文件解密、音乐下载等功能。

## 常用命令

### 构建和安装
```bash
# 构建二进制文件
make build

# 安装到 $GOPATH/bin
make install

# 运行测试
go test -v ./...

# 运行单个测试
go test -v -run TestWeapiLoginByQrcode ./example/
```

### Docker
```bash
# 构建镜像
make build-image

# 运行任务服务
make task
```

### 命令行工具 (ncmctl)
```bash
# 查看帮助
ncmctl -h

# 登录方式
ncmctl login phone 188xxx8888           # 短信登录
ncmctl login cookie -f cookie.txt       # cookie 登录
ncmctl login cookiecloud -u <user> -p <password> -s <server>  # cookiecloud 登录

# 每日任务
ncmctl task                    # 执行所有任务
ncmctl task --sign --scrobble  # 执行签到和刷歌

# 音乐下载
ncmctl download -l SQ 'https://music.163.com/song?id=xxx'  # 下载无损音质

# 云盘上传
ncmctl cloud '/path/to/music.mp3'

# NCM 解密
ncmctl ncm '/path/to/ncm/files' -o ./output
```

## 架构

### 目录结构
```
cmd/ncmctl/         # CLI 入口点 (main.go)
internal/ncmctl/    # CLI 命令实现 (login, task, cloud, download, ncm 等)
api/                # API 客户端层
  ├── api.go        # 核心 Client，处理请求/响应加解密
  ├── weapi/        # 网页端/小程序 API (推荐使用)
  ├── eapi/         # PC端/移动端 API
  ├── api/          # 基础 API
  ├── linux/        # Linux 客户端 API
  └── types/        # 公共类型定义
pkg/                # 工具库
  ├── crypto/       # 加解密实现 (AES-CBC/ECB, RSA, weapi/eapi 加密)
  ├── cookie/       # Cookie 管理 (持久化、同步)
  ├── cookiecloud/  # CookieCloud 支持
  ├── ncm/          # NCM 文件解密和音频标签处理
  ├── database/     # Badger 数据库封装
  ├── log/          # 日志模块
  └── utils/        # 通用工具函数
config/             # 配置结构和默认配置
example/            # API 使用示例测试
```

### API 层设计

核心客户端 `api.Client` 位于 `api/api.go`，负责：
- HTTP 请求发送（使用 resty 库）
- 请求参数加密 / 响应解密
- Cookie 管理（自动持久化到文件）

**加密模式** (`api/options.go`):
- `CryptoModeWEAPI`: 网页端 API，使用 AES-CBC 双重加密 + RSA 加密密钥
- `CryptoModeEAPI`: PC/移动端 API，使用 AES-ECB 加密
- `CryptoModeLinux`: Linux API，使用 AES-ECB 加密
- `CryptoModeAPI`: 无加密

**API 调用流程**:
```go
// 1. 创建客户端
cfg := &api.Config{...}
cli := api.New(cfg)

// 2. 调用 API
weapiClient := weapi.New(cli)
resp, err := weapiClient.GetUserInfo(ctx, &weapi.GetUserInfoReq{})

// 3. 关闭客户端 (确保 Cookie 刷盘)
cli.Close(ctx)
```

### 加密实现 (`pkg/crypto/crypto.go`)

关键函数：
- `WeApiEncrypt(object)`: weapi 请求加密
- `EApiEncrypt(url, object)`: eapi 请求加密
- `LinuxApiEncrypt(object)`: linux api 请求加密
- `EApiDecrypt(ciphertext, encode)`: eapi 响应解密

### 配置系统 (`config/config.go`)

- 支持环境变量覆盖 (前缀 `NCmctl_`)
- 支持魔法变量 `${HOME}` 替换
- 配置文件路径: `~/.ncmctl/` (cookie.json, database/, log/)

### NCM 文件解密 (`pkg/ncm/`)

支持解析网易云音乐加密的 `.ncm` 文件，输出 MP3/FLAC 格式。核心逻辑在 `pkg/ncm/ncm.go`。

## 开发注意事项

### Go 版本
要求 Go >= 1.24

### 测试
测试文件遵循 Go 惯例，`*_test.go`。示例测试在 `example/` 目录。

运行测试前需要设置 cookie 文件或跳过需要登录的测试。

### API 添加新接口
1. 在 `api/weapi/` 或 `api/eapi/` 下创建新文件
2. 定义请求/响应结构体
3. 实现调用方法，使用 `a.client.Request(ctx, url, req, resp, opts)`
4. 设置正确的 `CryptoMode` (默认 weapi)

### Cookie 处理
- Cookie 自动持久化到配置文件指定路径
- 使用 `api.Client.SetCookies()` / `GetCookies()` 管理
- 间隔刷盘，默认 3 秒

## 关键依赖

- `github.com/spf13/cobra` - CLI 框架
- `github.com/go-resty/resty/v2` - HTTP 客户端
- `github.com/dgraph-io/badger/v4` - 本地数据库
- `github.com/robfig/cron/v3` - 定时任务
- `github.com/spf13/viper` - 配置管理