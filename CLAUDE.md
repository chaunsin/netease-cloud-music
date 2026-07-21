# Repository Development Guide

本文是本仓库面向编码代理和贡献者的开发指南。`AGENTS.md` 是指向本文件的符号链接；请修改 `CLAUDE.md` 这一份事实来源，不要把两份文件改成彼此独立的副本。

## 项目概述

本仓库提供网易云音乐 Go API 客户端和命令行工具 `ncmctl`，主要包含登录、每日任务、音乐下载、云盘上传、NCM 文件解密、API 加解密调试和 HTTP(S) 监控代理。

当前最低 Go 版本以 `go.mod` 为准，现为 Go 1.25.0。

## 核心原则

在代码开始编写前你需要遵守以下规则

1. 这东西真的有必要存在吗？如果只是推测需要，那就直接省略，一句话说明就好。（YAGNI）
2. 能否复用。编写代码前先查看代码库中已有的辅助函数、工具函数、类型或模式。重复实现功能是最低效做法。
3. 标准库能做到吗？用它就行了。
4. 如果已安装的依赖项可以解决问题，那就用它。几行代码就能解决的问题，千万别添加新的依赖项。
5. 简单至上(Simplicity): 可以只写一行吗？一行。
6. 代码可读性(Readability): 应注重阅读速度，而不是打字速度
7. 高内聚低耦合。强相关的放在一起，减少不必要的依赖。方法、变量不能琐碎零散。
8. 单一职责(SRP):一个模块、类、方法只做一件事情，职责划分清晰。
9. 删除胜于添加，乏味胜于聪明。
10. 快速失败(Fail Fast): 禁止吞掉异常，提前暴露错误，显示错误优于隐式错误；只能在无法继续返回错误的清理或异步边界记录日志。
11. 为维护者编程: 机器可运行的代码不是目的，而是未来的自己、同事，甚至几年后的维护者都能快速理解和安全修改的代码。

## 常用命令

```bash
# 构建与安装
make build
make install

# 格式化与静态检查（需要 golangci-lint v2）
make fmt
make lint
make lintfix # 检测并修复 golangci-lint run --fix ./...

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

## 仓库导航

- `internal/ncmctl/`：Cobra 命令、参数校验和命令编排。
- `internal/proxy/`：HTTP(S) 转发、MITM、捕获、解析与脱敏。
- `api/`、`pkg/crypto/`：接口封装、请求传输和协议加解密。
- `pkg/cookie/`、`pkg/database/`、`pkg/log/`、`config/`：配置与运行数据持久化。
- `pkg/ncm/`：NCM 解析、解密和音频标签。
- `.claude/skills/ncmctl-dev/`：按任务加载的开发流程与专题约束。
- `skills/ncmctl/`：面向最终用户的安装和使用说明，不承载仓库开发约定。

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

1. 先用 `git status --short` 识别暂存、未暂存和未跟踪文件；读取相关未跟踪文件内容，再用 `git diff HEAD -- <本次文件>` 检查已跟踪变化。需要分辨两层时分别查看 `git diff --cached` 和 `git diff`。
2. 运行最小充分的目标测试和静态检查；不要越过上面的真实网络/账号边界。
3. 运行 `git diff --check`，检查 diff 中的空白错误和冲突标记。
4. 修改了 guidance、链接或 skill 时，用 `readlink`/`test -L` 检查符号链接，用 `test -e` 逐个确认本次改动的相对目标，并对每个改动的 skill 运行 validator；这些不属于 `git diff --check` 的能力。
5. 最终说明已验证内容，以及因网络、凭据、平台或副作用而未运行的检查。
