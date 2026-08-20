# PRD: ncmctl update 自更新命令

- 最后更新: 2026-08-18
- 模块: ncmctl CLI（internal/ncmctl）

## 1. 背景与目标

ncmctl 目前通过 GitHub Releases 发布预编译二进制，用户升级需要手动访问 Releases 页面、匹配平台资产、下载解压并替换本地文件。国内网络环境下 GitHub 直连缓慢或不可达，进一步抬高了升级门槛；手动下载也无法保证文件在传输过程中的完整性与真实性。

仓库内青龙安装脚本（`script/qinglong/qinglong_ncmctl_install.sh`）已经沉淀了一套经过生产验证的升级流程：latest release 解析、多代理路由（镜像优先 + GitHub 直连兜底）、SHA-256 checksums 校验、防降级、原子替换。但该能力仅存在于青龙场景，Go CLI 本身缺失自更新能力。

本需求将这套已验证逻辑移植为 CLI 一等命令 `ncmctl update`，让所有安装方式的用户都能一条命令完成安全升级。

### 1.1 目标

- 提供 `ncmctl update` 命令，自动检测最新 release 并完成下载、校验、替换
- 更新资源下载支持代理，默认使用仓库既定的 GitHub 代理链，适配国内网络
- 下载后进行 SHA-256 校验，保证资源完整性与真实性
- 与既有青龙脚本保持同一套发布约定（资产命名、checksums 文件、代理默认值），不制造两套事实源

### 1.2 非目标

- 不涉及现有 `ncmctl download` 音乐下载命令的代理能力（已澄清，代理仅作用于 update 的资源下载）
- 不实现 Docker 镜像内自更新（容器重建由镜像拉取负责，青龙环境继续使用既有脚本）
- 不做后台自动更新检查、定时更新等主动推送行为
- 不做 GPG/代码签名验证，校验以 SHA-256 为准
- 不追踪 pre-release / 指定版本安装（仅跟随 latest release）

## 2. 用户故事(User story)

### US-001: 一条命令完成升级

**描述:** 用户当前安装 ncmctl v0.4.0，看到 README 或 release 通知有新版本，希望不离开终端完成升级。手动升级需要识别平台、下载、解压、替换、赋权多个步骤，易出错。
**验收标准:**

- [ ] 执行 `ncmctl update` 能自动解析最新版本并与本地版本比较
- [ ] 本地已是最新时输出提示并以成功状态退出，不产生任何下载流量
- [ ] 存在新版本时自动完成下载、校验、替换，最终输出新版本号

### US-002: 国内网络环境下可用的升级

**描述:** 用户所在网络访问 GitHub 缓慢或失败，直接下载 release 资产不可行，需要经过 GitHub 代理镜像下载，且代理不可用时能自动回退。
**验收标准:**

- [ ] 默认启用仓库既定代理链（镜像优先、GitHub 直连兜底）
- [ ] 单个代理失败时自动切换下一个代理，全部失败后给出明确错误
- [ ] 支持通过 `--proxy` 覆盖代理配置，显式空值表示仅直连

### US-003: 下载资源可靠可信

**描述:** 用户担心代理镜像返回被篡改或损坏的文件，希望安装前有强校验，校验失败不能进入安装步骤。
**验收标准:**

- [ ] 下载完成后按 release 附带 checksums 文件做 SHA-256 比对
- [ ] 哈希不匹配时丢弃该文件并切换代理重试，绝不安装未校验文件
- [ ] 解压后复核二进制报告的版本与目标 release 一致

### US-004: 不会意外降级

**描述:** 用户本地构建了比最新 release 更新的版本（如主分支构建），执行 update 不应把本地版本覆盖成旧版本。
**验收标准:**

- [ ] 本地版本高于或等于 latest release 时跳过更新并提示原因
- [ ] 本地版本无法解析为 SemVer 时给出可理解的提示并按既有逻辑处理

## 3. 功能需求

整体流程：

```mermaid
flowchart TD
    A[执行 ncmctl update] --> B[读取当前二进制版本]
    B --> C[解析 latest release 版本]
    C --> D{本地 >= latest?}
    D -->|是| E[提示已是最新, 退出]
    D -->|否| F[遍历代理路由]
    F --> G[同路由下载 checksums + 归档]
    G --> H{SHA-256 校验通过?}
    H -->|否| F
    H -->|是| I[解压并复核二进制版本]
    I -->|失败| F
    I -->|成功| J[原子替换当前可执行文件]
    J --> K[输出新版本, 完成]
```

### FR-01: [P0]-版本检测与比较

描述: 获取本地版本与 GitHub latest release 版本，比较后决定是否进入下载流程，避免无谓流量与意外降级。

**前置条件**

- 无（命令可离线报告本地版本相关诊断）

**业务流程**

1. 读取当前可执行文件（`os.Executable()`）的版本：执行 `--version` 解析 `Version:` 行，与青龙脚本 `binary_version` 行为一致
2. 通过轻量 HEAD 请求 `https://github.com/chaunsin/netease-cloud-music/releases/latest`，从重定向最终 URL 提取 tag（`/releases/tag/vX.Y.Z`）
3. 对版本做 SemVer 校验（`v` 前缀 + `MAJOR.MINOR.PATCH`），非法则视为失败
4. 比较：本地 >= latest 时提示跳过并退出；本地 < latest 时进入下载流程

**交互说明**

- 每个阶段输出人类可读进度（"Fetching the latest release tag from GitHub..."、解析出的版本号）
- 非 SemVer 的本地版本（如本地 `make build` 注入的分支名）按既有脚本语义处理：提示后允许重装 latest

**规则边界限制**

- latest 解析失败（HEAD 失败、重定向 URL 非本仓库 tag 停放页、版本非 SemVer）→ 报错退出，不继续
- 网络请求统一受元数据超时限制（沿用青龙脚本 30s 约定）

**其他**

- 版本解析与比较逻辑、资产命名映射与青龙脚本保持同一套规则，避免两套事实源

### FR-02: [P0]-代理路由与默认代理

描述: 更新资源下载支持代理，默认采用仓库既定默认 GitHub 代理链，镜像失败后 GitHub 直连兜底。

**前置条件**

- 无

**业务流程**

1. 构造路由列表：用户配置的代理前缀（按顺序）+ 空前缀（GitHub 直连）固定在最后兜底
2. 默认路由与青龙脚本一致：`https://ghproxy.net/ https://ghfast.top/ https://gh-proxy.com/` + 直连兜底
3. 路由只负责转发完整 GitHub URL，不参与版本定义与校验逻辑
4. 校验清单（checksums）与归档下载必须经过同一路由，任一环节失败切换下一路由

**交互说明**

- `--proxy` flag 接受空格分隔的多个代理前缀；显式传空值（`--proxy ""`）表示仅直连
- 日志只展示代理域名（authority），不输出完整拼接 URL，避免泄露路径或用户信息（沿用脚本 `route_name` 思路）
- 默认路由可在 PRD 审核后固化进代码常量

**规则边界限制**

- 代理 URL 需为合法 HTTPS 前缀（拒绝用户信息、空主机、越界端口），非法配置快速失败
- 全部路由失败 → 明确报错退出，列出已尝试的路由

**其他**

- 默认值与青龙脚本 `DEFAULT_GITHUB_PROXIES` 保持一致；环境变量 `NCMCTL_QINGLONG_GITHUB_PROXIES` 仅作用于青龙脚本，本命令以 flag 为准 [Assumption]

### FR-03: [P0]-release 资产下载与 SHA-256 校验

描述: 下载平台对应资产归档与 checksums 清单，校验通过前不得进入安装步骤。

**前置条件**

- 已解析出最新版本号（FR-01）
- 已构造路由列表（FR-02）

**业务流程**

1. 按平台映射资产名（与 GoReleaser `archives` 配置一致）：
   - 归档：`ncmctl_<OS>_<ARCH>.tar.gz`（Windows 为 zip），OS 首字母大写（Linux/Darwin/Windows/Freebsd/Openbsd/Netbsd），ARCH 映射（amd64→x86_64、386→i386、arm→armv6 等）
   - checksums：`ncmctl_<版本去v>_checksums.txt`，GoReleaser 标准格式（`<64位hex>  <资产名>`）
2. 逐路由：先下载 checksums 清单并提取当前资产对应 SHA-256；再下载归档写入 `.part` 临时文件
3. 计算归档 SHA-256 与期望值比对，一致后校验归档内含 `ncmctl` 二进制（tar 列表 / zip 条目）
4. 全部校验通过才把 `.part` 改为正式临时文件，进入安装步骤

**交互说明**

- 校验失败输出"SHA-256 verification failed"并切换到下一路由，不中断命令
- checksums 中缺少当前资产条目或哈希格式非法 → 按该路由失败处理

**规则边界限制**

- 任何校验失败不得进入安装步骤（Fail Fast，禁止降级为弱校验）
- 下载受连接超时（10s）与下载超时（300s）约束，与青龙脚本约定一致
- 归档始终先写 `.part`，校验完成前不产生正式文件

### FR-04: [P0]-解压复核与原子替换

描述: 解压并复核候选二进制版本与 release 一致，随后原子替换当前可执行文件。

**前置条件**

- 归档已通过 SHA-256 校验（FR-03）

**业务流程**

1. 解压归档中的 `ncmctl` 到临时目录，确认是常规文件且非符号链接
2. 执行候选二进制 `--version`，解析版本并与目标 release 比对，不一致则判该路由失败
3. 替换当前可执行文件：
   - 目标目录内先写 staging 文件（同文件系统，避免跨文件系统 mv 半成品）
   - 并发防护：目标目录内以原子 `mkdir` 建立锁目录，冲突时快速失败并提示；完成后释放
   - Unix：直接 `rename` 覆盖当前二进制（运行中可覆盖）
   - Windows：先重命名旧文件（`.old`）再放入新文件，绕过运行中 exe 不可覆盖限制，随后删除旧文件 [Assumption]

**交互说明**

- 替换成功后输出新版本号与安装路径
- 失败时保留现场（staging 文件清理策略见 6.4），不留下半成品二进制

**规则边界限制**

- 候选文件不可执行 / 无法报告版本 / 版本不匹配 → 拒绝安装
- 目标目录不可写或替换失败 → 报错退出并给出手动升级指引（Releases 页面链接）
- 中断（INT/TERM）时清理临时文件

### FR-05: [P1]-命令注册与帮助

描述: `ncmctl update` 作为根命令子命令注册，帮助信息说明输入、副作用与限制（与仓库现有命令风格一致）。

**前置条件**

- 无

**业务流程**

1. 在 `internal/ncmctl/ncmctl.go` 的 `New()` 中注册 `NewUpdate` 子命令
2. 帮助文本描述：升级来源（GitHub Releases）、默认代理行为、`--proxy` 用法、对本地文件的副作用

**交互说明**

- 遵循现有命令帮助风格（`Run 'ncmctl <command> --help' for command-specific inputs, side effects, and limits`）
- `--debug` 全局 flag 生效时输出请求级调试日志

## 4. 产品方案

| 方案 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| Go 内置实现（移植青龙脚本逻辑） | 零外部依赖、跨平台、逻辑已被青龙生产验证、与既有发布约定天然一致 | 实现工作量中等；需处理 Windows 替换运行中 exe 的差异 | 采用 |
| 命令内调用 curl + 外部 sha256sum | 实现快 | 依赖外部命令、跨平台差异大、弱化 Fail Fast 与原子性 | 放弃 |
| 仅提示用户访问 Releases 手动下载 | 零工作量 | 未解决任何痛点，升级门槛不变 | 放弃 |

关键决策点：

1. **同一套事实源**：资产命名、checksums 文件、默认代理链全部沿用 GoReleaser 配置与青龙脚本既有约定，避免出现两套规则互相漂移
2. **同路由强校验**：checksums 与归档必须经同一代理路由获取，防止"清单走干净通道、文件走脏通道"的拼接攻击
3. **防降级**：版本比较沿用脚本 SemVer 比较规则，本地高于 latest 时跳过
4. **原子替换**：staging 与目标同目录 + rename 替换，任何时刻目标路径要么是旧版要么是新版

## 5. 非功能性需求

### 5.1 性能要求

- 已是最新版本时零下载流量，命令应在数秒内完成
- 下载受超时约束：连接 10s、元数据 30s、归档 300s（与青龙脚本一致）

### 5.2 安全要求

- SHA-256 校验失败禁止进入安装步骤，不得降级为弱校验
- 仅允许 HTTPS（含重定向），防止代理降级明文传输
- 日志脱敏：不输出完整代理 URL 与响应正文
- 拒绝替换为符号链接、非常规文件

### 5.3 兼容性要求

- 平台：linux / darwin / windows / freebsd / openbsd / netbsd，架构映射覆盖 GoReleaser 全量矩阵
- Windows：运行中 exe 替换需先重命名旧文件
- 本地非 SemVer 版本（分支名构建）可被识别并给出合理提示

### 5.4 可用性要求

- 代理链逐路由回退，单个镜像故障不影响整体升级
- 中断信号触发临时文件清理，不残留 `.part`/staging 文件
- 替换失败给出 Releases 手动下载指引，不把用户留在死胡同

## 6. 验收指标

| 场景ID | 验收点 | 前置条件(Given) | 触发动作(When) | 预期结果(Then) |
| --- | --- | --- | --- | --- |
| AC-001 | 正常升级 | 本地 v0.4.0，release 最新 v0.5.0，网络可达 | 执行 `ncmctl update` | 输出解析到的 v0.5.0，下载校验通过，替换后 `ncmctl --version` 显示 v0.5.0，命令成功退出 |
| AC-002 | 已是最新 | 本地 v0.5.0 与 latest 相同 | 执行 `ncmctl update` | 提示已是最新并成功退出，无任何下载流量 |
| AC-003 | 防降级 | 本地构建版本高于 latest（如 v1.0.0 或分支名） | 执行 `ncmctl update` | 提示本地不落后/跳过降级，成功退出 |
| AC-004 | 代理链回退 | 首个默认代理不可达，直连可用 | 执行 `ncmctl update` | 自动切换至下一路由并完成升级，日志展示各路由尝试 |
| AC-005 | 哈希不匹配 | 模拟代理返回篡改归档 | 执行 `ncmctl update` | SHA-256 校验失败，丢弃文件切换路由；全部失败时报错退出且不安装 |
| AC-006 | 仅直连 | 用户显式 `--proxy ""` | 执行 `ncmctl update` | 仅访问 github.com，不经过任何镜像 |
| AC-007 | 版本复核失败 | 归档内含版本不匹配的二进制 | 执行 `ncmctl update` | 该路由判失败并切换，最终不安装不匹配二进制 |
| AC-008 | 并发防护 | 两个 update 同时执行 | 并发执行 `ncmctl update` | 一个取得锁完成，另一个快速失败并提示锁占用 |
| AC-009 | 非法代理配置 | `--proxy "http://user:pass@host"` | 执行 `ncmctl update` | 快速失败，明确报出配置非法 |
| AC-010 | 权限不足 | 当前二进制目录只读 | 执行 `ncmctl update` | 明确报错并给出手动下载指引，不产生半成品 |

## 7. 风险与待决事项

| 事项 | 说明 | 状态 |
| --- | --- | --- |
| 默认代理链的具体镜像 | 本 PRD 默认 `ghproxy.net / ghfast.top / gh-proxy.com` + 直连兜底（与青龙脚本一致），公益镜像可能失效，需在实现时固化并允许 flag 覆盖 | 待确认 |
| Windows 运行中替换 | 需先重命名旧 exe 再放入新文件，方案为 [Assumption]，实现时验证 | 待确认 |
| 校验清单文件名 | GoReleaser checksums 默认模板需发版实测确认 `ncmctl_<去v>_checksums.txt` 命名（青龙脚本已按此约定生产使用） | 已验证 |
| 本地分支名版本 | `make build` 注入分支名导致本地版本非 SemVer，语义按"允许重装 latest"处理 | 已定 |
| 与青龙脚本的关系 | update 命令与青龙脚本并存，两者共享默认代理与发布约定，但不互相调用 | 已定 |

## 8. 参考

### 8.1 术语与定义

| 名词 | 说明 |
| ------ | ------ |
| release | GitHub Releases，含预编译资产与 checksums 清单 |
| checksums | GoReleaser 生成的 SHA-256 校验清单文件 |
| 路由(route) | 一个可用的下载前缀：某代理前缀或空前缀(GitHub 直连) |
| SemVer | 语义化版本号 MAJOR.MINOR.PATCH，release tag 带 v 前缀 |
| 防降级 | 本地版本不低于 latest 时拒绝覆盖安装 |

### 8.2 参考文档

- `script/qinglong/qinglong_ncmctl_install.sh`：青龙安装脚本，本需求移植的既有逻辑（代理链、版本解析、校验、原子替换、锁）
- `.goreleaser.yaml`：资产命名、checksums、ldflags 版本注入约定
- `docs/qinglong.md`：青龙脚本代理环境变量说明
- `internal/ncmctl/ncmctl.go`：命令注册与全局 flag（`--debug`、`--home`）
- `cmd/ncmctl/main.go`：ldflags 注入的 Version/Commit/BuildTime