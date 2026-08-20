# SPEC: ncmctl update 自更新命令

> Technical specification derived from: [docs/prd/prd-001-update.md](../prd/prd-001-update.md)
> Generated: 2026-08-18 | Target branch: main

## 1. Summary

### 1.1 What This SPEC Covers

在 `internal/ncmctl` 中新增 `ncmctl update` 子命令，将青龙安装脚本（`script/qinglong/qinglong_ncmctl_install.sh`）中已验证的升级流程移植为 Go 实现：latest release 解析、多代理路由（镜像优先 + GitHub 直连兜底）、SHA-256 checksums 校验、防降级、原子替换。命令只跟随 latest release，不做后台主动更新；校验以 SHA-256 为准，不做签名验证。

### 1.2 PRD Reference

- Source: [docs/prd/prd-001-update.md](../prd/prd-001-update.md)
- User Stories covered: US-001, US-002, US-003, US-004
- Functional Requirements covered: FR-01, FR-02, FR-03, FR-04, FR-05

### 1.3 Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| 实现位置 | `internal/ncmctl/update.go` 单文件 + `update_test.go` | 与仓库命令布局一致（`download.go` 18KB 先例），命令逻辑仅 CLI 使用，无需下沉 `pkg/` |
| SemVer 比较 | 手写 `compareSemver`，镜像脚本 `compare_semver` 语义 | go.mod 无 semver 依赖；仓库原则"几行代码能解决不添加新依赖"；行为与脚本事实源逐向量对齐 |
| 本地版本获取 | 执行当前二进制 `--version` 解析 `Version:` 行 | PRD FR-01 明确要求与脚本 `binary_version` 一致；候选二进制复核同样需要 exec，两条路径复用同一函数 |
| 安装位置 | `os.Executable()` 经 `filepath.EvalSymlinks` 解析后所在目录 | PRD "原子替换当前可执行文件"；解析符号链接避免替换链接本身 |
| 锁 | 目标目录内原子 `mkdir` 锁目录，命名复用脚本的 `.ncmctl.install.lock` | 与青龙脚本同名使两者指向同一目录时互相排斥；脚本方案已生产验证（不依赖 flock） |
| 运行进程检查（脚本 pgrep） | 不移植 | 自更新场景当前进程就是 ncmctl，检查恒真；Unix rename 可覆盖运行中文件，Windows 走 `.old` 方案 |
| 单路由内重试（脚本 MAX_ATTEMPTS） | 不移植 | 脚本默认 `MAX_ATTEMPTS=1`；PRD 未要求；路由级回退已覆盖容错需求 |
| HTTP 客户端 | 独立 `net/http.Client`，不复用 `api.Client` | update 无 Cookie/网易 API 语义；需要独立的逐请求超时（连接 10s/元数据 30s/归档 300s）与 HTTPS 强制 |
| 非 SemVer 本地版本 | 提示后允许重装 latest | 用户已确认（2026-08-18 澄清）；PRD AC-003 中"分支名"措辞为笔误，按 FR-01 与风险表"已定"执行 |
| 归档内条目名 | 二进制名 `ncmctl`，Windows 为 `ncmctl.exe` | GoReleaser 对 windows 产物追加 `.exe`，zip 内条目名随之变化；脚本仅覆盖 tar.gz（Linux）场景 |

---

## 2. Architecture

### 2.1 System Context

```
用户 ── ncmctl update ──┬── HEAD github.com/chaunsin/netease-cloud-music/releases/latest (经路由)
                       ├── GET  ncmctl_<去v>_checksums.txt          (经同一路由)
                       ├── GET  ncmctl_<OS>_<ARCH>.(tar.gz|zip)     (经同一路由)
                       └── 本地: os.Executable() → 目录内 staging → rename 原子替换
```

命令完全离线可诊断（本地版本读取不需要网络）；网络仅用于 release 解析与资产下载。不读取配置文件、不触碰 `.ncmctl` 状态目录（与现有 `download` 等命令的账号侧行为隔离）。

### 2.2 Component Design

新增 `Update` 命令类型，遵循仓库命令模式（`NewXxx(root *Root, l *log.Logger)` + `Command()` + `Add()`，参考 `logout.go`）：

```go
type UpdateOpts struct {
    Proxy string // --proxy，空格分隔的 HTTPS 代理前缀；显式空值 = 仅直连
}

type Update struct {
    root *Root
    cmd  *cobra.Command
    l    *log.Logger
    opts UpdateOpts
}
```

逻辑层拆为可单测的纯函数（全部无网络/无副作用依赖）：

| 函数 | 职责 | 对应脚本 |
|------|------|----------|
| `assetName(goos, goarch, goarm string) (string, error)` | 平台 → GoReleaser 资产名 | `map_architecture` |
| `parseVersionLine(output string) (string, error)` | `--version` 输出 → `Version:` 行 | `binary_version` |
| `compareSemver(left, right string) (int, error)` | SemVer 比较，返回 -1/0/1 | `compare_semver` |
| `validateProxy(prefix string) error` | HTTPS 代理前缀校验 | `validate_https_proxy` |
| `routeName(prefix string) string` | 日志脱敏显示（authority） | `route_name` |
| `checksumFromManifest(r io.Reader, asset string) (string, error)` | checksums 清单 → 目标资产 64 位 hex | `checksum_from_manifest` |

有状态编排保留在 `Update` 方法内（`execute`、`resolveLatestVersion`、`downloadAndVerify`、`install`），各方法共享一个 `updateState` 结构体（routes、latest、asset、staging 路径等），便于 defer 统一清理。

### 2.3 Module Interactions

主流程（对应 PRD mermaid 图）：

```
execute(ctx):
  ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM); defer stop()
  defer cleanup()                      // 临时目录 / staging / 锁，与脚本 EXIT trap 等价
  routes = buildRoutes(opts)           // 校验失败 → 快速失败
  latest = resolveLatestVersion(ctx, routes)   // FR-01；全部失败 → 报错退出
  if upToDate(localVersion, latest):   // 相等/更高 → 提示退出；非 SemVer/读不出 → 允许重装
      return nil
  staged = downloadAndVerify(ctx, routes, latest) // FR-03 + FR-04 解压复核；全部失败 → 报错退出
  install(ctx, staged, latest)         // FR-04 锁内复查 → 原子替换 → 输出新版本
```

命令注册：`ncmctl.go` `New()` 中追加 `c.Add(NewUpdate(c, &c.l).Command())`。

### 2.4 File Structure

```
internal/ncmctl/
├── ncmctl.go        [modified]  New() 注册 NewUpdate 子命令
├── ncmctl_test.go   [modified]  TestCommandHelpContract / TestCommandPositionalArgumentContract /
│                                TestCommandFlagDescriptionsExplainConstraints 追加 update 条目
├── update.go        [new]      Update 命令 + 编排 + 纯函数（资产映射/SemVer/代理校验/checksums 解析）
├── update_replace_unix.go   [new]  build !windows：os.Rename 直接覆盖
├── update_replace_windows.go [new] build windows：三步替换 + 回滚
└── update_test.go   [new]      单元 + httptest 集成测试（见 §9）
docs/usage.md        [modified]  命令副作用清单表补充 update 行（来源 GitHub Releases、替换本地可执行文件）
```

---

## 3. Data Model

### 3.1 Schema Changes

无。命令不引入任何持久化状态、数据库表或配置文件改动。

### 3.2 Entity Definitions

N/A（无新实体；`updateState` 为进程内临时结构，见 §2.2）。

### 3.3 Relationships

N/A。

### 3.4 Migration Plan

N/A。

---

## 4. API Design

本命令不暴露对外 API；本章定义其对外部世界的**依赖契约**（GitHub Releases 交互），实现与测试以此为准。

### 4.1 External Dependencies

| 资源 | URL 模式 | 方法 | 用途 |
|------|----------|------|------|
| latest 重定向 | `https://github.com/chaunsin/netease-cloud-music/releases/latest` | HEAD | 经重定向最终 URL 解析 tag（元数据 30s 超时） |
| checksums 清单 | `https://github.com/chaunsin/netease-cloud-music/releases/download/<tag>/ncmctl_<tag去v>_checksums.txt` | GET | 期望 SHA-256（元数据 30s 超时） |
| 资产归档 | `https://github.com/chaunsin/netease-cloud-music/releases/download/<tag>/ncmctl_<OS>_<ARCH>.(tar.gz|zip)` | GET | 安装包（归档 300s 超时） |

所有 URL 均经路由前缀拼接：`routeURL(prefix, githubURL) = prefix + githubURL`（空前缀即直连）。连接超时 10s 作用于全部请求（Transport DialContext）。

### 4.2 Response Formats

- **latest 重定向**：最终 URL 必须匹配 `<路由前缀>https://github.com/chaunsin/netease-cloud-music/releases/tag/vX.Y.Z` 或（代理跳回直连时）直连 tag 前缀；tag 需满足 `v` 前缀 + SemVer（`MAJOR.MINOR.PATCH`，可选 `-pre` / `+build`）。拒绝外部仓库、额外路径段、query 参数（对齐脚本 `version_from_release_url` 与测试向量）。
- **checksums**：GoReleaser 标准格式，每行 `<64位hex>  <资产名>`；解析规则：`fields[1] == 资产名 && len(fields[0]) == 64 && 全 hex`，取小写。缺条目或格式非法 = 该路由失败。
- **归档**：tar.gz（archive/tar + compress/gzip）或 zip（archive/zip），根目录直接包含二进制条目（GoReleaser 默认不包目录）：`ncmctl` / `ncmctl.exe`。

### 4.3 HTTP 行为约束

- 仅 HTTPS：代理前缀强制 `https://`；重定向目标非 HTTPS 直接拒绝（`CheckRedirect` 校验），最终 URL scheme 复核
- 请求级超时：连接 10s / 元数据 30s / 归档 300s（脚本 `CONNECT_TIMEOUT/METADATA_TIMEOUT/DOWNLOAD_TIMEOUT` 约定）
- HTTP 非 2xx 视为失败（对齐 curl `--fail`）
- 归档下载写入 `<tmp>/<asset>.part`，全部校验通过后才改名为正式文件；失败即删除 `.part`

### 4.4 Breaking Changes

无（新增子命令，不影响既有命令）。

---

## 5. Business Logic

### 5.1 Core Algorithms

**5.1.1 资产命名映射**（对应 `.goreleaser.yaml` name_template 与脚本 `map_architecture` 全矩阵）

```
OS: linux→Linux, darwin→Darwin, windows→Windows, freebsd→Freebsd,
    openbsd→Openbsd, netbsd→Netbsd；其余 → error "unsupported GOOS: <x>"
ARCH: amd64→x86_64, 386→i386, arm→armv6(goarm=6), arm64→arm64,
      s390x/ppc64/ppc64le/riscv64/mips/mipsle/mips64/mips64le/loong64 → 原名；
      其余 → error "unsupported GOARCH: <x>"
扩展名: windows→.zip，其余→.tar.gz
资产名: ncmctl_<OS>_<ARCH>.<ext>     例: ncmctl_Linux_x86_64.tar.gz / ncmctl_Windows_arm64.zip
```

**5.1.2 版本行解析**（对齐脚本 `binary_version`）

扫描 `--version` 输出行，匹配 `^\s*Version:\s*` 前缀，取该行剩余内容并 `TrimSpace`；无匹配行 → error。

**5.1.3 SemVer 比较**（镜像脚本 `compare_semver`，语义逐条对齐）

```
输入剥 v 前缀；非法 SemVer → error（调用方按"非 SemVer → 重装"处理）
1. MAJOR.MINOR.PATCH 按十进制数值比较（Go 直接用整数解析，天然无溢出问题）
2. 核心相同：无预发布 > 有预发布；同为预发布时逐标识符比较：
   数字标识符按数值比较；数字 < 字母数字；字母数字按 ASCII 字典序
3. 构建元数据 (+build) 不参与比较（v1.2.3 == 1.2.3+build.7）
返回 -1(左<右) / 0 / 1(左>右)
```

黄金向量（来自脚本 `install_test.sh` `test_semver_comparison`，必须原样通过）：
`v1.2.3 vs 1.2.3+build.7 → 0`；`2.0.0 vs 1.99.99 → 1`；`1.2.3 vs 1.2.3-rc.1 → 1`；`1.2.3-alpha.2 vs 1.2.3-alpha.10 → -1`。

**5.1.4 代理校验**（对齐脚本 `validate_https_proxy`）

```
对每个 --proxy 值按空白拆分（strings.Fields）：
- url.Parse 成功且 scheme == "https"
- 无 userinfo（拒绝 http://user:pass@host）
- Host 非空；端口（如有）为 1..65535 的十进制数字；hostname 无空白字符
- 允许路径前缀（如 https://proxy.example/private-token/），拼接 URL 时保留
- 规范化：TrimRight(prefix, "/") + "/" 后再拼接（对齐脚本"统一补齐末尾斜杠"）
任一非法 → 快速失败："invalid HTTPS proxy at position N: <原因>"（原因不包含用户输入原文中的敏感段）
```

**5.1.5 路由构造**（对齐脚本 `configure_routes`）

```
routes = []
if flag Changed("proxy"):   prefixes = Fields(opts.Proxy)          // 空串 → 无前缀
else:                       prefixes = DEFAULT_GITHUB_PROXIES      // 与脚本一致：
                            // https://ghproxy.net/ https://ghfast.top/ https://gh-proxy.com/
每个前缀经 5.1.4 校验后追加
routes 固定追加 ""（GitHub 直连兜底，永远最后）
```

**5.1.6 latest 版本解析**（对齐脚本 `get_latest_version`）

```
for route in routes:
    HEAD routeURL(route, "https://github.com/chaunsin/netease-cloud-music/releases/latest")
    从最终响应 URL（resp.Request.URL）提取 tag：
      前缀 = route + "https://github.com/chaunsin/netease-cloud-music/releases/tag/"
      或（route 非空时）直连 tag 前缀（代理跳回 github.com 场景）
      tag 需匹配 ^v + SemVer；失败 → 下一路由
全部失败 → error "Unable to resolve the latest GitHub release from any configured route"
```

日志顺序对齐脚本：`Fetching the latest release tag from GitHub...` → 逐路由 `Trying <routeName>...` → `Latest version: vX.Y.Z`。

**5.1.7 下载与校验**（对齐脚本 `download_archive` + `download_checksum` + `extract_and_validate`）

```
for route in routes:
    1. 经 route 下载 checksums 清单到 <tmp>/<name>.part → checksumFromManifest
       缺条目/非法 → 删 .part，下一路由
    2. 经 route 下载归档到 <tmp>/<asset>.part（归档 300s 超时）
    3. sha256(.part) == 期望值？否 → 删 .part，下一路由（"SHA-256 verification failed via <route>"）
    4. 归档必须包含二进制条目：名称完全匹配（Windows 为 ncmctl.exe）、
       常规文件、非符号链接、路径穿越防护（entry name Clean 后不得含 .. 或为绝对路径）
       不满足 → 删 .part，下一路由
    5. 解压二进制到目标目录 staging 文件（os.CreateTemp(dir, ".ncmctl.update-*")，
       同文件系统，兼容 TMPDIR noexec 环境；对齐脚本把候选文件放进 INSTALL_DIR）
       chmod 0755 → 执行 staging --version → parseVersionLine
       版本去 v 后与目标 release 不一致 → 删 staging，下一路由
    6. 返回 staging 路径
全部路由失败 → error "Unable to download a valid <asset> from any configured route"
```

关键不变式：checksums 与归档**必须经同一路由**获取（PRD 关键决策点 2，防拼接攻击）。

**5.1.8 安装**（对齐脚本 `acquire_install_lock` + `install_binary`）

```
target = EvalSymlinks(os.Executable())（失败用原值）；dir = filepath.Dir(target)
lockDir = dir + "/.ncmctl.install.lock"
1. os.Mkdir(lockDir)：已存在 → error "Unable to acquire installation lock <lockDir>;
   another installer may be running (remove a stale lock only after verifying no installer is active)"
2. 锁内复查 upToDate(local, latest)（staging 期间目标可能已被并发升级）：
   已不落后 → 释放锁，成功退出（对齐脚本 test_concurrent_upgrade_does_not_downgrade）
3. replaceExecutable(target, staged)：
   Unix (!windows):  os.Rename(staged, target)          // 同目录原子覆盖，运行中可覆盖
   Windows:          target 缺失（上次替换中途失败的自愈）→ 直接 rename staged→target
                     os.Rename(target, target+".old")
                     os.Rename(staged, target)
                     os.Remove(target+".old")            // 运行中必然失败 → 告警降级
                     任一步失败：若 target 缺失且 .old 存在 → rename 回滚 .old；
                     仍失败 → error 并给出手动升级指引
   .old 删除失败：新二进制已就位，仅告警 "The previous binary could not be removed
                 (<target>.old); the leftover will be replaced by the next update."
                 视为成功；残留文件由下次升级覆盖
4. rmdir 释放锁（defer 兜底：仅当锁由本进程持有且未释放）
5. 输出 "ncmctl installed successfully at <target> (version: <latest>)."
   + "Restart ncmctl to use the new version."（对齐脚本措辞 + 重启提示）
```

staging 与 target 同目录 ⇒ rename 不跨文件系统（PRD 关键决策点 4）。

**5.1.9 防降级判定**（对齐脚本 `is_up_to_date`）

```
local = exec(当前二进制, "--version") → parseVersionLine
读取失败     → 提示 "Unable to read the installed version; reinstalling." → 允许重装
compareSemver(local, latest) error → 提示 "Installed version <local> is not valid SemVer;
                                      reinstalling <latest>." → 允许重装      [澄清确认]
== 0 → "ncmctl is up-to-date (version: <local>)." → 成功退出（零下载流量）
> 0  → "Installed version <local> is newer than GitHub Release <latest>; skipping downgrade."
       → 成功退出
< 0  → 进入下载流程，提示 "Installed version: <local>. A newer version (<latest>) is available."
```

### 5.2 Validation Rules

| 输入 | 规则 | 失败行为 |
|------|------|----------|
| `--proxy` 每个前缀 | §5.1.4 HTTPS 代理校验 | 快速失败，不发起任何网络请求 |
| 重定向最终 URL | 必须为本仓库 tag 停放页（含路由前缀变体） | 该路由失败，切下一路由 |
| tag 格式 | `v` + SemVer | 该路由失败 |
| checksums 条目 | `fields[1]==资产名 && 64 位 hex` | 该路由失败 |
| 归档条目 | 名称精确匹配、常规文件、非符号链接、无路径穿越 | 该路由失败 |
| 候选二进制 | 可执行、能报告版本、版本 == release | 该路由失败 |

### 5.3 State Machine

路由级状态机（下载阶段）：

```
for each route (含最后的直连):
  ┌─ checksums 获取失败 ─┐
  ▼                      │
  START → [获取checksums] → [获取归档] → [SHA-256] → [归档结构] → [版本复核] → 成功(staged)
                ▲            ▲            ▲            ▲            ▲
                └────────────┴────────────┴────────────┴────────────┘ 任一失败 → 下一路由
最后一路由失败 → 终止：清理全部临时文件，报错退出（不安装）
```

安装阶段：`未锁定 → 已锁定(锁内复查) → 替换 → 解锁`；任何错误路径 defer 释放锁并清理 staging。

### 5.4 Edge Cases

| 场景 | 处理 |
|------|------|
| 本地版本非 SemVer（`make build` 注入分支名） | 提示后允许重装 latest（§5.1.9，用户已确认） |
| 本地二进制 `--version` 执行失败/无 Version 行 | 提示后允许重装 |
| 代理跳回 github.com（转发后 302） | 最终 URL 按直连 tag 前缀校验，仍只接受本仓库 |
| 代理返回停靠页/外部仓库/带 query 的 tag URL | 视为解析失败，切路由 |
| 归档含篡改内容但 SHA-256 恰好匹配 | 归档结构校验 + 候选版本复核兜底（双层防线） |
| staging 校验期间另一进程完成升级 | 锁内复查版本，拒绝覆盖更新版本 |
| 锁目录残留（进程被杀） | 快速失败 + 提示人工确认后清理（对齐脚本措辞） |
| `os.Executable()` 为符号链接（macOS brew 等） | `EvalSymlinks` 解析后替换真实文件，链接保持有效 |
| 中断（INT/TERM） | context 取消中止下载；defer 清理 `.part`/staging/临时目录/锁 |
| 目标目录只读 / staging 创建失败 | 报错退出，提示 Releases 手动下载指引（`https://github.com/chaunsin/netease-cloud-music/releases`），不产生半成品 |
| Windows 替换中途失败 | 回滚 `.old`；无法回滚则报错 + 手动指引 |
| Windows target 缺失（上次替换中途失败残留） | 跳过移开步骤直接换入，自愈并清掉 `.old` 阻塞 |
| Windows 运行中升级 `.old` 删除失败 | 新二进制已就位，仅告警并视为成功，残留由下次升级覆盖 |
| 下载/归档条目超过大小上限 | 拒绝该路由（checksums 1 MiB、归档及条目 256 MiB） |
| 归档路径穿越条目（`../`、绝对路径） | 拒绝该路由 |

---

## 6. Error Handling

### 6.1 Error Taxonomy

用户可见错误消息与脚本对齐（便于维护者对照青龙日志排查），统一经 `RunE` 返回由根命令 `PrintErrln` 输出：

| 场景 | 错误消息（措辞对齐脚本） |
|------|--------------------------|
| latest 全部路由失败 | `Unable to resolve the latest GitHub release from any configured route` |
| 代理配置非法 | `invalid HTTPS proxy at position <N>: <原因>`（原因脱敏，不包含用户输入原文） |
| 单个路由失败（继续回退） | 前缀 + `via <routeName>`，如 `SHA-256 verification failed via HTTPS proxy ghproxy.net` |
| checksums 缺资产条目 | `Checksum entry for <asset> was not found via <routeName>` |
| 归档全部路由失败 | `Unable to download a valid <asset> from any configured route` |
| 锁占用 | `Unable to acquire installation lock <dir>; another installer may be running (remove a stale lock only after verifying no installer is active)` |
| 目标目录不可写/替换失败 | 具体错误 + `Manual upgrade: https://github.com/chaunsin/netease-cloud-music/releases` |
| 不支持平台 | `unsupported GOOS/GOARCH: <x>`（快速失败，发生在资产映射阶段） |

路由失败日志：非致命失败记录到 stderr/logger 后继续（对齐脚本 `echo ... >&2`）；命令终止时 `RunE` 返回聚合错误。

### 6.2 Retry Strategy

- 无单路由内重试（脚本默认 `MAX_ATTEMPTS=1` 语义）
- 路由级回退即重试策略：每条路由独立尝试，全部失败才终止
- 超时参数：连接 10s（DialContext）/ 元数据请求 30s / 归档请求 300s

### 6.3 Failure Modes

| 依赖 | 失败表现 | 降级 |
|------|----------|------|
| GitHub 不可达 | 路由逐条失败 | 切镜像 → 最终直连；全失败报错退出，不破坏现有安装 |
| 公益镜像返回坏数据 | checksums/哈希/结构/版本任一校验不过 | 丢弃并切下一路由，绝不降级为弱校验（PRD 5.2） |
| 本地二进制异常 | 版本读取失败 | 允许重装（不尝试替换异常二进制自身） |
| 替换阶段失败 | 报错 + 手动指引 | 既有二进制保持原状（staging 清理，无半成品） |

---

## 7. Security

### 7.1 Authentication & Authorization

N/A。命令匿名访问公开 GitHub Releases，无凭据交互。代理前缀拒绝 userinfo（防凭据泄露到日志与请求头）。

### 7.2 Input Validation

- `--proxy`：§5.1.4 校验，非法快速失败（AC-009）
- checksums：严格 64 位 hex + 精确资产名匹配（防注入/错配）
- 归档条目：名称精确匹配、`TypeReg`、非符号链接、路径穿越防护（`path.Clean` 后拒绝 `..` 前缀与绝对路径）——拒绝符号链接同时满足"拒绝替换为符号链接"（PRD 5.2）
- tag 提取：仅接受本仓库 `/releases/tag/v...` 停放页形态

### 7.3 Data Protection

- 传输：仅 HTTPS（代理前缀校验 + `CheckRedirect` 拒绝非 HTTPS 重定向 + 最终 URL scheme 复核），对齐脚本 `--proto '=https' --proto-redir '=https'`
- 日志脱敏：`routeName` 只输出 authority（host[:port]），不输出路径/query/fragment/userinfo（对齐脚本 `route_name` 与 `test_proxy_logs_are_redacted`）；`--debug` 全局 flag 下也不输出完整拼接 URL 与响应正文
- 完整性：SHA-256 校验强制（PRD 5.2 Fail Fast），无弱校验降级路径
- 资源上限：checksums 下载上限 1 MiB，归档下载及解包条目上限 256 MiB（防磁盘填充）

### 7.4 Trust Model

更新以 SHA-256 校验为准，**不做发布签名验证**：checksums 清单与归档经同一路由获取，因此信任等级等于代理链中最弱一环。被完全攻陷的代理（或直连路径上的中间人）可同时提供自签 checksums 与伪造二进制，并让伪造二进制自报匹配版本，从而绕过哈希与版本复核，构成任意代码执行面。

缓解与局限：

- 路由按"镜像优先、直连兜底"顺序尝试，直连仅在镜像全部失败时介入，无法对"成功"的恶意镜像形成交叉验证
- 镜像代理为社区/第三方提供的信任边界（与青龙脚本相同）；用户可显式只保留直连（`--proxy` 留空）或只信任指定代理
- 未来方向：引入发布签名验证（如 cosign/sigstore）或将 checksums 经独立通道交叉比对（PRD [Assumption]：当前不做签名验证）

---

## 8. Performance

### 8.1 Expected Load

单用户交互式 CLI，无并发压力。典型成功路径 = 2 次元数据请求 + 1 次归档下载（经最优路由）。已是最新时仅 1 次 HEAD 请求（零下载流量，数秒内完成，PRD 5.1）。

### 8.2 Optimization Strategy

- 版本比较先于下载：本地不落后时零流量退出
- 路由尝试顺序固定"镜像优先、直连兜底"，首个可用路由即完成全部步骤
- 归档流式解压（`io` 流式读取 tar/zip 条目），不整包载入内存；`.part` 直接用于 SHA-256 计算（边写边哈希或复用文件，避免二次读盘）
- 锁仅覆盖安装阶段（短临界区），下载阶段不持锁

### 8.3 Database Considerations

N/A（无数据库）。

---

## 9. Testing Strategy

### 9.1 Unit Tests（纯函数，无网络）

| 函数 | 覆盖点 |
|------|--------|
| `compareSemver` | 脚本黄金向量 + 前导零（`01.2.3` 非法）、大数、预发布数字/字母混合、build 忽略、非法输入 error |
| `assetName` | GoReleaser 全矩阵（12 架构 × 6 OS 抽样全组合）+ 不支持平台 error；Windows 扩展名 zip |
| `parseVersionLine` | 标准输出（含 ASCII art title 多行）、无 Version 行、空白变体 |
| `checksumFromManifest` | 合法、缺条目、非法 hex、多余列、CRLF、空行 |
| `validateProxy` | 脚本非法清单（`http://`、无主机、`user@`、端口 0/65536/非数字、空 host）+ 合法路径前缀（`https://proxy.example/private-token/`） |
| `routeName` | 脱敏：路径/query/userinfo 不出现（对齐脚本测试向量 `https://user:secret@proxy.example:8443/private/token?access=hidden` → `HTTPS proxy proxy.example:8443`） |
| tag 提取 | 直连/代理前缀/代理跳回直连三形态；外部仓库、额外路径、query 拒绝 |
| `replaceExecutable` | Unix：rename 覆盖已有目标；Windows 分支：通过包级 `windowsReplace` 注入强制启用后以普通文件验证三步序列与回滚（见 9.3 说明） |

### 9.2 Integration Tests（httptest，离线）

用 `httptest.Server` 扮演代理与 GitHub（转发或直接响应），fixture 参考脚本 `install_test.sh` 的 fake curl 方案（`write_version_binary` 生成可执行 fixture 二进制、tar.gz/zip 打包、checksums 清单）：

1. **正常升级**：fake GitHub 提供 checksums + 归档，本地旧版本 → 安装成功，替换后 `--version` 显示新版本（AC-001）
2. **已是最新**：本地 == latest → 成功退出，断言 fake server 仅收到 1 次 HEAD、0 次下载（AC-002）
3. **防降级**：本地 v1.0.0 > latest → 跳过，文件哈希不变（AC-003）
4. **非 SemVer 重装**：本地 `master` → 提示后完成重装（AC-003 澄清分支）
5. **代理回退**：首代理 5xx/超时 → 第二代理成功（AC-004）
6. **哈希不匹配**：首代理返回篡改归档 → 切路由成功；全部失败 → 报错且现有二进制不变（AC-005）
7. **仅直连**：`--proxy ""` → 断言 fake 代理零请求（AC-006）
8. **版本复核失败**：归档内二进制版本不符 → 切路由（AC-007）
9. **锁占用**：预创建锁目录 → 快速失败且不删除他人锁（AC-008 单侧；并发双进程由 10 覆盖）
10. **非法代理**：`--proxy "http://user:pass@host"` → 快速失败，零网络请求（AC-009）
11. **只读目录**：目标目录 chmod 0500（非 root 下有效）→ 报错 + 手动指引（AC-010）
12. **锁内复查**：staging 校验完成后模拟目标被更新 → 拒绝覆盖（对齐脚本 `test_concurrent_upgrade_does_not_downgrade`）
13. **中断清理**：cancel ctx → 断言无 `.part`/staging/锁残留

并发测试（AC-008 双进程）：两个 goroutine 同时执行安装阶段，断言一胜一败，不使用时间 sleep 碰运气（channel 同步）。

### 9.3 Edge Case Tests

- Windows 替换序列：`replaceExecutable` 的 Windows 分支在非 Windows 平台通过临时覆盖包级 `windowsReplace` 强制启用后验证"rename→rename→remove"序列与失败回滚（与 `removeFile` 注入同模式，注入类测试不得并行）；真实 Windows 上运行中替换需人工验证（见 §11.2）
- Windows target 缺失：断言直接换入成功（自愈），`.old` 不残留
- Windows `.old` 删除失败：注入 `removeFile` 返回错误 → 告警输出、替换结果视为成功，`.old` 残留由下次升级覆盖
- 下载大小上限：Content-Length 预检与流式超限（`io.LimitReader`）均报错且不残留 `.part`；解包条目超限拒绝该路由（tar.gz 与 zip）
- 符号链接 `os.Executable`：用 t.TempDir 内 symlink 指向真实二进制模拟，断言替换后链接仍指向新版本
- 归档路径穿越条目（`../evil`）拒绝
- 多路由全失败时无任何残留文件（对齐脚本 `assert_no_temporary_files`）

### 9.4 Acceptance Criteria Mapping

| 验收 | 测试 | 类型 |
|------|------|------|
| AC-001 | 正常升级集成测试 | integration |
| AC-002 | 已是最新零流量测试 | integration |
| AC-003 | 防降级 + 非 SemVer 重装 | integration |
| AC-004 | 代理回退测试 | integration |
| AC-005 | 哈希不匹配切路由 / 全失败不安装 | integration |
| AC-006 | 仅直连测试 | integration |
| AC-007 | 版本复核失败切路由 | integration |
| AC-008 | 锁占用快速失败 + 并发双进程 | integration |
| AC-009 | 非法代理快速失败 | unit |
| AC-010 | 只读目录报错指引 | integration |
| FR-01/02 边界 | tag 提取、代理校验、路由构造 | unit |
| FR-03 | checksums 解析、SHA-256、归档结构 | unit + integration |
| FR-04 | 替换/回滚/清理/锁内复查 | unit + integration |
| FR-05 | 帮助契约测试（见下） | unit |

命令契约测试（对齐 `ncmctl_test.go` 既有风格）：`TestCommandHelpContract` 增加 `{path: []string{"update"}, use: "update", longContains: "...", exampleContains: "--proxy"}`；`TestCommandPositionalArgumentContract` 增加 `valid: {{}}, invalid: {{"extra"}}`；`TestCommandFlagDescriptionsExplainConstraints` 增加 `--proxy`（含"space-separated"、"empty"语义说明）。

---

## 10. Implementation Plan

### 10.1 Phases

实现顺序尊重依赖关系：

1. **P1 骨架**：`update.go` 命令结构、flag、帮助文本、`ncmctl.go` 注册、契约测试条目
2. **P2 纯函数层**：`assetName` / `parseVersionLine` / `compareSemver` / `validateProxy` / `routeName` / `checksumFromManifest` + 单元测试（先写测试与脚本向量对齐）
3. **P3 网络层**：专用 HTTP client（超时/HTTPS 强制/最终 URL 捕获）、`resolveLatestVersion`、下载与校验流程（`downloadAndVerify`）+ httptest 集成测试
4. **P4 安装层**：锁、锁内复查、`replaceExecutable` 双平台实现、信号清理、`install` 编排
5. **P5 收尾**：全量测试 + race、`docs/usage.md` 表格补充、`git diff --check`、`make lint`

### 10.2 Issue Mapping

| Issue | SPEC Sections | Priority | Depends On |
|-------|--------------|----------|------------|
| #1 update 命令骨架与注册 | 2.2, 2.4, 4.4, 9.4(契约) | high | — |
| #2 纯函数层（含 SemVer/资产映射/校验） | 5.1.1–5.1.5, 5.2, 9.1 | high | #1 |
| #3 网络层与下载校验 | 4.1–4.3, 5.1.6–5.1.7, 6 | high | #2 |
| #4 安装层与平台替换 | 5.1.8–5.1.9, 5.4, 7 | high | #3 |
| #5 集成测试与文档 | 9.2–9.4, 10.1-P5 | medium | #2, #3, #4 |

### 10.3 Incremental Delivery

单次发布包含完整命令（PRD 无分阶段上线要求）。`--proxy` 默认值以代码常量固化（`defaultGithubProxies`），后续镜像变更只需改常量。命令与青龙脚本并存，互不调用（PRD 风险表"已定"）。

---

## 11. Open Questions & Risks

### 11.1 Unresolved Questions

- Windows 运行中替换（`ncmctl.exe` 被进程占用时的 rename 行为）需在真实 Windows 环境验证；`.old` 回滚为 SPEC 定义方案，实现时如有偏差以实际验证为准（PRD [Assumption]）
- 公益镜像 `ghproxy.net / ghfast.top / gh-proxy.com` 的长期可用性（PRD 风险表"待确认"项已按默认值固化进常量，`--proxy` 可覆盖）

### 11.2 Technical Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Windows 替换方案与预期不符 | 更新失败 | 三步替换 + 回滚；失败路径给出手动指引，既有 exe 可恢复；target 缺失自愈、`.old` 删除失败降级为告警（§5.1.8 第 3 步） |
| 恶意/被攻陷代理提供伪造 checksums 与归档 | 任意代码执行 | 信任模型显式声明（§7.4），不做签名验证为既定边界；未来引入 cosign/sigstore 缓解 |
| 只读目录测试在 root/CI 环境失效 | 测试不可靠 | 测试内检测（若 chmod 后仍可写则 t.Skip），核心断言放在非 root 本地执行 |
| 公益镜像返回合法但过期的 checksums/资产 | 安装陈旧版本 | 候选二进制版本复核兜底（与目标 release 比对），不匹配即拒绝 |
| 锁残留导致后续更新被阻塞 | 用户需手动清理 | 错误消息明确提示"确认无安装器运行后可删除"（与脚本一致） |
| 并发 double-update 覆盖新版本 | 降级 | 锁内复查版本（§5.1.8 第 2 步） |

### 11.3 Assumptions

- 非 SemVer 本地版本 → 提示后重装 latest（2026-08-18 与用户澄清确认；PRD AC-003 "分支名"表述按笔误处理）
- 环境变量 `NCMCTL_QINGLONG_GITHUB_PROXIES` 不作用于本命令，以 `--proxy` flag 为准（PRD [Assumption]）
- 默认代理链与青龙脚本 `DEFAULT_GITHUB_PROXIES` 完全一致并固化为常量
- 不移植脚本 MAX_ATTEMPTS 重试语义（默认值即 1，PRD 未要求）
- 锁目录复用脚本命名 `.ncmctl.install.lock`：与本仓库青龙脚本指向同一安装目录时实现跨工具互斥
- 不检查"ncmctl 正在运行"（自更新场景不适用，脚本 pgrep 语义无意义）
- 归档内条目名为 `ncmctl`（非 Windows）与 `ncmctl.exe`（Windows），根目录平铺（GoReleaser 默认不包目录）