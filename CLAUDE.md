# Repository Development Guide

本文是本仓库面向编码代理和贡献者的开发指南。`AGENTS.md` 是指向本文件的符号链接；请修改 `CLAUDE.md` 这一份事实来源，不要把两份文件改成彼此独立的副本。

## 项目概述

本仓库提供网易云音乐 Go API 客户端和命令行工具 `ncmctl`，主要包含登录、每日任务、音乐下载、云盘上传、NCM 文件解密、API 加解密调试和 HTTP(S) 监控代理。

当前最低 Go 版本以 `go.mod` 为准，现为 Go 1.25.0。

## 开始工作前

1. 先运行 `git status --short`，确认暂存区、工作区和未跟踪文件。工作区可能包含用户尚未提交的改动，不要回退、覆盖或顺手格式化无关文件。
2. 阅读离改动最近的源码、测试和文档。不要只根据 README、CLI 表象或远端协议猜测实现。
3. 搜索已有类型、辅助函数和调用模式后再新增代码。优先使用标准库和现有依赖；几行代码可以完成时不要引入新依赖。
4. 保持行为不变，除非任务明确要求改变语义。删除代码前必须证明没有调用方，并清理导出、引用、测试和文档残留。
5. 不忽略错误。返回值应包装足够的上下文；只能在无法继续返回错误的清理或异步边界记录日志。

## 常用命令

```bash
# 构建与安装
make build
make install

# 格式化与静态检查（需要 golangci-lint v2）
make fmt
make lint
make lintfix

# Makefile 的完整测试入口
make test

# Docker
make build-image
make task                 # 使用 IMAGE_VERSION（默认 latest）启动真实账号任务容器
```

`make fmt` 使用 `golangci-lint fmt -c .golangci.yaml ./...`，不等同于只运行 `gofmt`。`make lintfix` 可能改动大量文件；运行前后都要检查 `git diff`。除非任务本身要求调整 lint 策略，不要通过修改 `.golangci.yaml` 来压制源码诊断。

### 测试安全边界

优先从与改动对应的离线包开始，例如：

```bash
go test ./api ./pkg/crypto ./internal/ncmctl ./internal/proxy
go test -race ./internal/proxy
go test -run TestName ./path/to/package
```

不要把 `go test ./...` 或 `make test` 当作无副作用的默认检查：

- `api/weapi` 和 `api/eapi` 中存在未加 build tag 的真实网络测试，会访问网易服务；若 `testdata/cookie.json` 有有效凭据，部分测试会签到、提交音乐合伙人测评或执行其他账号操作。
- `example/` 使用 `integration` build tag，登录、上传和下载示例同样访问真实网络并可能修改账号或本地文件。只有在用户明确允许相应副作用后才运行，例如：

```bash
go test -tags=integration -v -run TestWeapiLoginByQrcode ./example/
```

若没有权限运行真实网络测试，完成离线的目标包测试、`make lint`（适用时）和 `git diff --check`，并在结果中明确未验证的边界。

## 目录与职责

```text
cmd/ncmctl/                         CLI 入口和版本信息
internal/ncmctl/                    Cobra 命令、参数校验和命令编排
internal/proxy/                     HTTP(S) 转发、MITM、捕获、解析与脱敏
api/api.go                          HTTP 客户端、加密模式分派、Cookie 生命周期
api/options.go                      请求方法和 CryptoMode 选项
api/xeapi.go                        XEAPI URL 改写、公钥刷新、会话状态
api/weapi/                          Web/小程序接口
api/eapi/                           PC/移动端接口
api/api/                            明文 API 接口
api/linux/                          Linux API 接口
api/types/                          跨接口共享类型
pkg/crypto/                         WEAPI/EAPI/Linux/XEAPI/NCBL 加解密
pkg/cookie/                         Cookie Jar 和持久化
pkg/cookiecloud/                    CookieCloud 客户端
pkg/database/                       数据库接口和 Badger 实现
pkg/ncm/                            NCM 解析、解密和音频标签
pkg/log/                            日志与滚动文件
config/                             配置结构和嵌入式默认配置
example/                            带 integration tag 的真实服务示例
docs/xeapi.md                       XEAPI 研究记录，不替代源码和黄金向量
skills/ncmctl/                      可分发的 ncmctl 用户 skill
.claude/skills/ncmctl-dev/          本仓库开发 skill
```

## CLI 开发

- 根命令在 `internal/ncmctl/ncmctl.go` 注册子命令。新增命令前先复制最接近的现有命令模式，不要假设所有命令都有完全相同的结构。
- 使用 Cobra 的 `RunE` 传播错误；参数在执行副作用前校验。不要只打印错误后返回成功。
- 运行时客户端使用 `api.NewClient(c.root.Cfg.Network, c.l)`。在 `internal/ncmctl` 内通过 `defer closeAPIClient(ctx, cli)` 关闭客户端，确保 Cookie 最终刷盘且关闭错误不会被静默丢弃。
- 需要登录的命令在产生账号操作前验证登录状态。Token 刷新位置以邻近命令的实际控制流为准，不要套用会漏掉早退路径的通用模板。
- 并发下载、上传、解密和代理代码必须正确处理取消、资源关闭和 goroutine 退出；共享状态需要用原子操作、锁或现有同步抽象保护。
- CLI 语法的事实来源是 Cobra flag 定义和当前构建的 `ncmctl <command> --help` 输出。只有语法、默认值、输出、错误、持久化位置或安全/副作用边界变化时，才同步更新用户文档；行为不变的内部重构不需要制造文档 churn。

## API 开发

`api.Client.Request` 根据 `api.Options` 处理请求。目前的模式定义在 `api/options.go`：

| 模式 | 用途 | 主要实现 |
| --- | --- | --- |
| `CryptoModeWEAPI` | Web/小程序请求，默认模式 | `pkg/crypto.WeApiEncrypt` |
| `CryptoModeEAPI` | PC/移动端请求 | `pkg/crypto.EApiEncrypt` |
| `CryptoModeLinux` | Linux 客户端请求 | `pkg/crypto.LinuxApiEncrypt` |
| `CryptoModeAPI` | 明文 API | `api.Client.Request` |
| `CryptoModeXEAPI` | Aegis/XEAPI 请求与响应 | `api/xeapi.go`、`pkg/crypto` |

当前通用请求层对 `CryptoModeAPI` 不会自动将 `req` 序列化到 query 或 form；`CryptoModeEAPI` 也只直接处理明文 JSON 响应，不透明解密 `e_r=true` 响应。将这些模式用于新 endpoint 前，先补请求/响应路径和测试。

添加或修改接口时：

1. 在匹配的 `api/weapi`、`api/eapi`、`api/api` 或 `api/linux` 包中定义请求、响应和方法；共享响应结构放到 `api/types` 之前先证明它确实跨接口复用。
2. 使用 `opts := api.NewOptions()`；WEAPI 是默认值，其他模式调用 `SetCryptoModeEAPI`、`SetCryptoModeLinux`、`SetCryptoModeAPI` 或 `SetCryptoModeXEAPI`。仓库没有 `api.WithCryptoMode` 函数。
3. 调用 `a.client.Request(ctx, endpoint, req, &reply, opts)`，包装传输错误，并由调用方显式判断业务 `Code`。目前请求层只实现 GET 和 POST；扩展方法前补齐实现和测试。
4. EAPI 摘要依赖正确的原始路由路径；不要为了让请求“看起来一致”随意改写 URL。
5. XEAPI 的明文 envelope 保留 `/api/` 语义，传输路径改写到 `/xeapi/`。公钥刷新、会话头、锁和 `singleflight` 是同一状态机，不能只修改其中一段。协议变更必须优先使用抓包或历史证据，并增加黄金向量；仅做加密后再自行解密的闭环测试不够。

详细示例见 `.claude/skills/ncmctl-dev/references/api-guide.md`。XEAPI 背景材料见 `docs/xeapi.md`，当前客户端行为以 `api/api.go`、`api/xeapi.go`、`api/options.go`、`api/xeapi_test.go` 和 `pkg/crypto/crypto_test.go` 为准。NCBL 是独立的日志正文 wire format，以 `pkg/crypto/ncbl.go` 和 `pkg/crypto/ncbl_test.go` 为准，不属于 XEAPI 会话状态机。

## 代理开发

`internal/proxy` 的首要约束是“观察失败不能改变真实流量”：

- 只对网易目标域名捕获和 HTTPS MITM；其他流量透明转发且不输出。
- 捕获、截断、解压、协议解析、格式化和脱敏只操作观察副本，不得消费、替换或截断真实请求/响应。
- 默认递归脱敏。无法安全结构化处理的正文应输出摘要；只有显式 `--show-sensitive` 才允许原始敏感值。
- 输出使用有界异步队列。stdout 阻塞时报告 `CAPTURE_DROPPED`，不能反压转发链路。
- 被动代理拿不到 WEAPI 随机密钥或 XEAPI 会话密钥时必须标记 `unsupported`，不能把密文或猜测伪装成明文。
- 自动生成的 CA 私钥在 POSIX 上保持 `0600`，目录保持 `0700`；Windows 使用当前用户 ACL。不要使用 goproxy 内置的公开 CA 私钥。
- 修改连接、队列、证书缓存或关闭流程后，至少运行目标测试；涉及共享状态时运行 `go test -race ./internal/proxy`。

## 配置与持久化

- 不传 `--config` 时，程序使用嵌入的 `config/config.yaml`；它不会自动读取 `~/.ncmctl/config.yaml`。
- `--config <file>` 的设计契约是由 Viper 读取指定文件并应用 `NCMCTL_` 前缀的环境变量覆盖；嵌套键用下划线表示，例如 `NCMCTL_LOG_LEVEL`。但当前实现会在 `config.New` 的 `UnmarshalExact` 阶段报 `invalid decode hook signature`；修复并补回归测试前，不要把自定义配置或相应环境变量标记为已验证可用。
- 全局 `--home` 替换配置中的 `${HOME}`，默认运行数据位于 `<home>/.ncmctl/`。默认 Cookie、日志和数据库路径分别来自 `config/config.yaml`。
- Cookie 目录和文件分别以 `0700`、`0600` 创建；`Client.Close` 会触发最终持久化。Cookie API 的 `GetCookies` 和 `SetCookies` 都需要明确的 `*url.URL`。
- 修复配置加载后，自定义配置应从 `config/config.yaml` 复制完整结构再修改；`viper.UnmarshalExact` 应拒绝未知字段，并应增加覆盖文件加载、环境变量和 `${HOME}` 替换的测试。

## 测试与代码风格

- 测试使用 Go 惯例 `*_test.go`，断言统一复用 `github.com/stretchr/testify/assert` 和 `require`。
- 网络协议、加密和编码优先添加固定输入/固定输出的黄金向量，并覆盖畸形输入、边界长度和错误传播。
- 并发测试避免依赖时间碰运气；使用 channel、context、fake transport 或 `httptest` 建立可控同步点。
- 不提交凭据、Cookie、真实手机号、CA 私钥、抓包敏感正文、下载音乐或生成的运行文件。
- 保留现有 SPDX/版权头格式。注释说明原因、协议约束或不直观的并发行为，不重复代码字面意思。

## 文档与 skill 同步

- `AGENTS.md -> CLAUDE.md`，`.agents/skills/ncmctl-dev -> ../../.claude/skills/ncmctl-dev`；提交前用 `readlink` 确认链接未被替换成副本。
- `CLAUDE.md` 维护仓库级开发规则；`.claude/skills/ncmctl-dev` 维护按任务渐进加载的开发流程；`skills/ncmctl` 只面向 ncmctl 安装与使用，不承担仓库开发约定。
- 用户可见的语法、默认值、输出、错误、持久化位置或安全/副作用边界变化时，同步更新 `README.md` 和 `skills/ncmctl`。开发流程、调用约定或协议不变量变化时更新开发 skill；只修正本任务直接影响的文档，范围外漂移只报告。
- 文档中的命令必须能由当前 Cobra help、Makefile、`go.mod` 或源码验证；不要记录尚未实现的交互、环境变量、退出码或协议能力。

## 完成检查

1. 查看 `git diff HEAD -- <本次文件>`，确认暂存和未暂存变化都在任务范围内；需要分辨两层时再分别查看 `git diff --cached` 和 `git diff`。
2. 运行最小充分的目标测试和静态检查；不要越过上面的真实网络/账号边界。
3. 运行 `git diff --check`，检查 diff 中的空白错误和冲突标记。
4. 修改了 guidance、链接或 skill 时，用 `readlink`/`test -L` 检查符号链接，用 `test -e` 逐个确认本次改动的相对目标，并对每个改动的 skill 运行 validator；这些不属于 `git diff --check` 的能力。
5. 最终说明已验证内容，以及因网络、凭据、平台或副作用而未运行的检查。
