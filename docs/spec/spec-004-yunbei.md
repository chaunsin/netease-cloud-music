# SPEC: ncmctl yunbei 云贝任务命令

> Technical specification derived from: [docs/prd/prd-004-yunbei.md](../prd/prd-004-yunbei.md)
>
> Generated: 2026-09-05 | Target branch: develop | Commit: 171294d
>
> Input note: 本 SPEC 基于当前工作区中的 PRD 内容生成；PRD 相对上述提交包含尚未提交的 `task` 调度需求。
>
> Revised: 2026-09-05 — 补齐子 agent 审查的全部 P2 项，并固化 runAll、即时恢复、gate 重试和 sign 查询隔离决策。
> Revised: 2026-10-30 — 依授权实测更正 view-vip-center 响应契约为双层包裹（推翻 2026-09-05 的直接布尔结论），并确认非会员账号该任务永不生效、改为跳过。
>
> Evidence update: 2026-09-05 — 根据用户授权提供的两次真实只读响应，修正 V3/todo 关联、签到状态、任务身份和 status 冲突语义；写操作门禁仍按本节记录的剩余证据关闭。
>
> Static evidence update: 2026-09-05 — 复核网易云当前公开会员中心、小众歌曲和分享 H5 构建产物，关闭会员中心浏览与小众歌曲协议缺口，并确认分享仍依赖原生客户端交互。
>
> Runtime evidence update: 2026-09-05 — 根据用户授权的真实执行日志补齐 `NO_PREV_BOOKED`，修正会员中心上报主机，并兼容播放器返回的网易 CDN HTTP 地址。

## 1. Summary

### 1.1 What This SPEC Covers

本 SPEC 定义新的顶级命令 `ncmctl yunbei` 及其 `status` 只读模式，覆盖云贝余额、签到、连签奖励、任务列表、预约活动、任务执行、任务领奖、临时账号状态恢复和最终结果汇总；同时定义该命令接入 `ncmctl task` 的选项复制、cron 校验、同进程防重入、与 `sign` 作业互斥以及停机取消契约。

实现继续使用单个 `api.Client`、现有 WEAPI/EAPI/XEAPI wrapper、Cookie 持久化、日志、图片上传、动态发布和播放能力。不增加数据库、配置文件、后台队列或第三方依赖。服务端任务列表、进度和领取状态是唯一事实源；一次动作接口返回成功只能表示“动作已发送”，不能直接判定云贝任务完成。

本需求涉及的部分上游协议尚无足够证据。第 4.5 节列出的协议门禁必须先通过脱敏抓包、上游固定样本或用户明确授权的真实请求关闭。未关闭门禁的处理器不得以猜测字段、固定 ID、中文任务名或参考仓库常量进入发布版本。

### 1.2 PRD Reference

- Source: [docs/prd/prd-004-yunbei.md](../prd/prd-004-yunbei.md)
- User Stories covered: US-001 ～ US-006
- Functional Requirements covered: FR-01 ～ FR-15
- Acceptance Criteria covered: AC-001 ～ AC-018

### 1.3 Design Decisions Summary

| # | 决策点 | 选择 | 理由 |
| --- | --- | --- | --- |
| D1 | 执行授权 | 无参数不执行；必须显式使用 `--task` 或 `--all` | 发布、互动和长时播放都有账号副作用，不能隐式开启 |
| D2 | 任务分发 | 公开英文标识优先映射证据确认的 `missionCode`；响应未提供时回退到证据确认的 V3 `taskId`；中文名称只展示 | 真实响应中 `extInfoMap` 可为 null，不能把 `missionCode` 当成唯一身份，也不能用易变文案路由写操作 |
| D3 | 完成判定 | 动作后刷新服务端任务状态；只有稳定任务键对应的完成状态或进度增长才算确认 | 防止把 HTTP/业务成功误报为任务完成 |
| D4 | 状态恢复 | 内存补偿日志记录本次新增状态；每个互动任务确认后立即逆序恢复，`--delete` 图文也立即删除；异常/取消路径由有界清理兜底 | 缩短临时状态暴露时间，不等待后续慢任务或领奖 |
| D5 | 编排方式 | 单客户端、串行阶段、最多 3 轮；无进度变化立即收敛 | 降低风控和并发写入风险，行为可复现 |
| D6 | API 演进 | V3 任务条目使用导出的类型别名；响应字段只做证据支持的增量补齐 | 便于复用，并尽量保持外部 Go 调用方的类型兼容 |
| D7 | 协议缺口 | 先完成 Phase 0 取证；缺证据的已知任务只展示并安全跳过，不尝试写请求 | 当前仍缺连签领奖、预约领奖、三项任务身份及站外分享合同 |
| D8 | 调度互斥 | sign/yunbei 共用一个 gate；每个作业另有 `atomic.Bool` 占位，初次 `TryLock` 失败后每 30 秒重试，最多重试 3 次，仍失败则提示并跳过本次触发 | 短暂重叠可自行恢复，同名 cron 触发不会堆积成等待队列或补跑 |
| D9 | 取消与停机 | `yunbei` 使用信号上下文；调度停机先取消作业上下文，再 `cron.Stop` 并有界等待 | 当前 `task` 只停止调度、不取消或等待在途作业，无法满足清理契约 |
| D10 | 退出结果 | `unconfirmed`、动作失败、领奖失败、删除失败或恢复失败均返回非零；今日不可用/已完成/已领取正常返回 | 保持脚本和 cron 可观察性，同时把幂等跳过视为正常 |
| D11 | 共享代码 | 复用 `utils.Sleep`、`validateNoteContent`、`validateLocalImage`、`closeAPIClient` 和现有 API wrapper；媒体下载沿用既有流式/临时文件模式并增加命令内安全校验 | 不复制业务能力，同时补齐现有下载 helper 缺少的重定向与目标地址限制 |
| D12 | `task --runAll` 兼容语义 | 默认包含完整 yunbei 作业，不增加交互确认或额外 selector | 用户确认 `runAll` 是可向前扩展的“全部任务”选择；帮助、启动摘要和升级说明必须展示新增副作用 |
| D13 | sign 查询隔离 | sign 在登录后独立执行，只加载签到/连签数据；余额、V3、todo 或 reserve 故障不得阻断 sign | 独立能力不依赖无关状态源，组合执行时也先完成 sign 再加载常规任务快照 |

---

## 2. Architecture

### 2.1 System Context

```text
ncmctl root
├── yunbei [status]                         <- 本 SPEC 的一次性入口
│   ├── api/weapi                           <- 登录、余额、任务列表、签到、领奖等
│   ├── api/eapi                            <- 任务动作、预约、发布、红心等
│   └── api.Client                          <- Cookie、超时、重试、协议与关闭
├── sign                                    <- 行为保持，不迁移
└── task
    ├── sign ----┐
    │            ├── signYunBeiGate         <- 同进程共享互斥
    └── yunbei --┘
```

`yunbei status` 和执行模式共用同一套快照加载逻辑。命令层负责参数校验、任务关联、阶段编排、结果聚合和补偿；API 层只负责强类型请求/响应与协议选择，不承载任务分发或业务状态机。

### 2.2 Component Design

| 组件 | 职责 | 明确不负责 |
| --- | --- | --- |
| `YunBei` | Cobra 命令、选项、信号上下文、客户端生命周期、最终输出 | 不保存本地任务进度 |
| `yunBeiRegistry` | 英文任务标识、阶段、风险说明、`missionCode` 映射、处理器注册 | 不从中文标题猜测任务类型 |
| `yunBeiSnapshot` | 合并余额、签到、V3 完整任务和 EAPI 待办列表 | 不自行推导上游未知字段 |
| `yunBeiRuntime` | 当前 UID、执行轮次、动作尝试记录、动态 ID、补偿日志和结果 | 不跨进程或跨运行持久化 |
| 任务处理器 | 只执行一个任务族，并在动作后请求快照确认 | 不直接打印整份响应或敏感字段 |
| `Task` 集成 | 复制 `YunBeiOpts`、校验 cron、注册回调、互斥和停机等待 | 不复制云贝 API 编排 |

云贝命令层采用单文件实现，全部生产代码集中在 `internal/ncmctl/yunbei.go`。文件内部按“命令与校验、数据模型与注册表、快照加载、主编排、任务处理器、补偿与输出”的顺序组织；各方法继续保持单一职责，不因单文件约束合并成万能函数。测试仍放在独立的 `yunbei_test.go`，不计入生产实现文件。

不增加通用 service/interface 层。离线测试通过 `api.Client.SetTransport` 注入 fake `RoundTripper`；不为测试改造生产调用为一组浅包装接口。

### 2.3 Module Interactions

执行模式的数据流：

```text
参数校验
  -> signal.NotifyContext
  -> api.NewClient
  -> weapi.GetUserInfo
  -> 根据选择生成 query plan
  -> sign / reserve 使用各自状态源独立执行
  -> 仅在选择常规任务或 claim 时加载 task-list-v3 / eapi todo
  -> 依固定阶段运行已选择且待执行的处理器
       -> 动作前确认原状态
       -> 写操作成功后登记补偿项
       -> 刷新快照
       -> 仅服务端确认后标记 completed
       -> 立即恢复该任务新增的互动状态；publish-note 在 --delete 时立即删除
  -> 如有进度且仍有待执行任务，进入下一轮（最多 3 轮）
  -> 仅在选择 claim 时领取已完成待领取奖励
  -> 刷新领取状态
  -> 有界清理尚未完成的删除意图和补偿项
  -> 最终刷新、汇总、关闭 API Client
```

只读模式在登录校验和 `loadSnapshot` 后直接输出，不能实例化执行器、登记补偿项或调用任何写接口。

### 2.4 File Structure

| 文件 | 状态 | 变更内容 |
| --- | --- | --- |
| `internal/ncmctl/yunbei.go` | 新增 | 云贝命令全部生产实现：命令、选项、模型、注册表、任务感知查询、编排、全部处理器、安全媒体下载、测试时钟 hook、补偿和输出 |
| `internal/ncmctl/yunbei_test.go` | 新增 | 参数、快照、编排、取消、补偿和 task 集成离线测试 |
| `internal/ncmctl/ncmctl.go` | 修改 | 注册 `NewYunBei(c, &c.l).Command()` |
| `internal/ncmctl/task.go` | 修改 | 命名持有 `YunBeiOpts`、flags、选择、cron、共享互斥和有界停机 |
| `api/weapi/yunbei.go` | 修改 | V3 条目类型别名、状态/领奖所需字段的证据化类型调整 |
| `api/weapi/yunbei_test.go` | 修改 | 保留 live gate；增加纯 JSON/离线请求契约测试 |
| `api/eapi/yunbei.go` | 修改 | 待办项补齐已验证字段，必要时补充云贝专用动作 wrapper |
| `api/eapi/yunbei_test.go` | 修改 | 保留 live gate；增加 EAPI 固定响应和请求字段测试 |
| `api/options.go`、`api/api.go` | 修改 | 增加请求级敏感日志抑制，解码行为保持不变 |
| `api/eapi/event.go` | 修改 | 对 NOS token/上传及动态发布启用敏感日志抑制 |
| `README.md` | 修改 | 顶级命令和高风险副作用摘要 |
| `docs/usage.md` | 修改 | 完整 flags、status、执行与 task 示例 |
| `skills/ncmctl/references/commands.md` | 修改 | 用户 Skill 的命令和安全边界 |

不修改数据库、配置结构、`go.mod`、`.golangci.yaml` 或现有 `sign` 的公开行为。

### 2.5 Code Quality Contract

- 使用现有 `utils.Sleep(ctx, min, max)`；禁止新增另一份 timer/rand helper，禁止直接 `time.Sleep`。
- 每类等待区间使用集中命名的 `time.Duration` 常量；数值来自对应协议 gate，不在处理器中散落魔数。随机区间由 `utils.Sleep` 内部的 `math/rand/v2` 实现。
- 生产默认使用 `utils.Sleep`、`time.Now` 和 `context.WithTimeout`；通过 runtime 函数字段注入测试替身，离线测试不得真实等待随机区间或 60/65 秒超时。
- 所有业务错误包含操作名；传输错误使用 `%w`，业务 `Code` 与传输错误分开判断。
- 对同一用户的动作串行执行，不为提速并发点赞、关注、发布、分享或播放。
- API wrapper 不修改调用方请求中与默认值无关的字段，不记录 Cookie、token、签名或完整请求体。
- 任何响应切片索引前检查长度；任何 transport error 后不得继续解引用可能为 nil 的响应。
- `sign.go` 只有在抽取共享逻辑能由现有请求顺序与输出测试证明完全等价时才允许改动；默认方案是不改。

---

## 3. Data Model

### 3.1 Schema Changes

无数据库或持久化 schema 变化。所有快照、动作记录和补偿记录只存在于单次进程内存中。

### 3.2 Command Types

```go
type YunBeiOpts struct {
	Tasks   []string
	All     bool
	Title   string
	Message string
	Image   string
	Delete  bool
}

type YunBei struct {
	root   *Root
	cmd    *cobra.Command
	opts   YunBeiOpts
	l      *log.Logger
	status bool
}
```

`TaskOpts` 以命名字段持有 `YunBeiOpts`，不再增加第三组匿名嵌入的 `Title/Message/Image/Delete` 冲突：

```go
type TaskOpts struct {
	// 现有字段保持不变
	YunBeiOpts YunBeiOpts
	YunBei     bool
	YunBeiCron string
}
```

### 3.3 Normalized Task Model

```go
type yunBeiTaskID string

type yunBeiTaskState string

const (
	yunBeiUnavailable       yunBeiTaskState = "unavailable"
	yunBeiPending           yunBeiTaskState = "pending"
	yunBeiRunning           yunBeiTaskState = "running"
	yunBeiCompletedUnclaimed yunBeiTaskState = "completed-unclaimed"
	yunBeiClaimed           yunBeiTaskState = "claimed"
	yunBeiUnconfirmed       yunBeiTaskState = "unconfirmed"
	yunBeiFailed            yunBeiTaskState = "failed"
	yunBeiUnsupported       yunBeiTaskState = "unsupported"
)

type yunBeiTaskSnapshot struct {
	ID          yunBeiTaskID
	MissionCode string
	TaskID      int64
	UserTaskID  int64
	Name        string
	Description string
	Current     int64
	Target      int64
	Reward      int64
	State       yunBeiTaskState
	Conflicted  bool // 稳定键或状态源冲突，禁止后续 todo 覆盖为可执行
	Claim       *yunBeiClaimRef
	Action      *yunBeiActionRef
}

type yunBeiClaimRef struct {
	Period      int64
	UserTaskID  int64
	DepositCode int64
}

type yunBeiActionRef struct {
	TaskID     int64
	SubAction  string
	Type       string
	CheckToken string
}
```

`CheckToken` 只在请求边界使用，禁止进入普通日志、最终结果或格式化错误。任务进度的源字段必须由 Phase 0 为每个 `missionCode` 确认；不能把 `ProgressRate`、`Times`、`Value` 或 `CompletedPoint` 统一当成同一语义。

### 3.4 Registry and Runtime

```go
type yunBeiStage uint8

const (
	stageAccount yunBeiStage = iota // sign, reserve
	stageFast                        // view/like/follow/collect/publish/share
	stageSlow                        // indie listen, daily recommend play
	stageClaim                       // claim
)

type yunBeiTaskDef struct {
	ID           yunBeiTaskID
	Stage        yunBeiStage
	MissionCodes []string
	TaskIDs      []int64
	SideEffects  string
	Run          func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error
}
```

`MissionCodes` 和 `TaskIDs` 只能写入真实响应确认的值。存在 `missionCode` 时优先使用；缺失时仅允许使用已取证的 V3 `taskId`。两者同时存在却指向不同本地任务时必须判定冲突。`sign`、`claim`、`reserve` 是独立能力，可不依赖常规任务列表；其余任务必须有至少一种稳定身份映射后才可注册为可执行。

```go
type yunBeiRuntime struct {
	uid           int64
	round         int
	attempted     map[yunBeiActionKey]struct{}
	results       map[yunBeiTaskID]*yunBeiResult
	compensations []yunBeiCompensation
	eventIDs      []int64
	sleep         func(context.Context, time.Duration, ...time.Duration) error
	now           func() time.Time
	withTimeout   func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

type yunBeiActionKey struct {
	TaskID   yunBeiTaskID
	Kind     string
	TargetID string
}

type yunBeiCompensation struct {
	Kind     string // comment-unlike, artist-unsub, playlist-del, song-unlike
	TargetID string
	ScopeID  string // thread/playlist 等必要关联 ID
	Done     bool
}
```

`attempted` 记录“任务 + 动作 + 目标”指纹，防止同一轮或后续轮次重复操作同一资源；它不阻止任务选择新的候选继续推进。生产构造函数必须填入三个时间 hook，测试可替换为立即返回、固定时间和手动控制的 timeout。补偿项只在“原状态为 false 且本次写操作由服务端返回成功”后追加。任务确认后立即恢复并标记 `Done`；重复恢复必须是幂等的。输出目标 ID 和动作，不输出评论正文、动态正文、昵称或 token。

### 3.5 API DTO Evolution

`YunBeiTaskListV3` 的条目改为导出类型别名，保持原匿名结构的类型身份：

```go
type YunBeiTaskListV3Item = struct {
	Completed        bool                   `json:"completed"`
	CompletedPoint   int64                  `json:"completedPoint"`
	ExtInfoMap       *YunBeiTaskExtInfo     `json:"extInfoMap"`
	Period           int64                  `json:"period"`
	Status           int64                  `json:"status"`
	SubAction        string                 `json:"subAction"`
	TargetPoint      int64                  `json:"targetPoint"`
	TargetUserTaskId int64                  `json:"targetUserTaskId"`
	TaskDescription  string                 `json:"taskDescription"`
	TaskId           int64                  `json:"taskId"`
	TaskName         string                 `json:"taskName"`
	TaskPoint        int64                  `json:"taskPoint"`
	TaskPointDetail  []YunBeiTaskPointDetail `json:"taskPointDetail"`
	TaskType         string                 `json:"taskType"`
	UserTaskId       int64                  `json:"userTaskId"`
	// 其余现有展示字段原样保留
}

type YunBeiTaskExtInfo = struct {
	MissionCode string `json:"missionCode"`
}

type YunBeiTaskPointDetail = struct {
	ProgressRate     int64  `json:"progressRate"`
	RewardExtendInfo string `json:"rewardExtendInfo"`
	RewardId         int64  `json:"rewardId"`
	RewardType       int64  `json:"rewardType"`
	SortValue        int64  `json:"sortValue"`
	StageType        int64  `json:"stageType"`
	Status           int64  `json:"status"`
	SumTarget        int64  `json:"sumTarget"`
	Times            int64  `json:"times"`
	UserMissionId    int64  `json:"userMissionId"`
	Value            int64  `json:"value"`
	Worth            int64  `json:"worth"`
}
```

上例只表达命名方向，最终字段集合必须与现有结构逐项等价，不能在重构时丢弃字段。真实样本已确认 `subAction` 位于 V3 条目顶层，且当日 `newbie=null`、任务均位于 `normal.list`。`Newbie any` 仍不能作为所有日期的最终合同：G1 必须继续枚举服务端实际出现的其他非空 bucket，将共享任务字段复用为 `YunBeiTaskListV3Item`；遇到非空但未识别的 bucket 时，`status` 只显示安全摘要并正常返回，执行模式在任何对应阶段写请求前失败。

真实 todo 样本未返回 `taskId/subAction/type/checkToken`，只确认了 `userTaskId/period/depositCode` 等展示和领奖字段；这些动作字段不得因 V3 存在同名字段而擅自加入 todo 顶层合同。`period/userTaskId/depositCode` 的 Phase 0 结论必须明确其合法范围：若三者严格为正，可用零值判缺失；只要任一字段允许合法零值，就改用指针或自定义 presence-aware 解码，不能混淆“字段缺失”与“显式为 0”。

### 3.6 Correlation Rules

快照合并必须按以下顺序建立一对一关系：

1. V3 任务优先用非空 `missionCode` 关联本地注册表；缺失时才按证据确认的非零 `taskId` 关联。
2. EAPI todo 的非零 `userTaskId` 优先匹配 V3 顶层 `userTaskId` 或 `taskPointDetail[].userMissionId`。真实领奖样本确认 todo 使用后者，而非 V3 顶层值。
3. todo 实际返回非零 `taskId` 时才允许作为回退键；字段缺失或零值不能从名称补齐。
4. 两个键指向不同条目、同一键跨条目重复或所有稳定键为空时，记录“任务关联冲突”，只隔离对应条目。`status` 显示非致命摘要并正常返回；执行和领奖路径拒绝使用冲突条目。
5. 禁止用 `taskName`、`taskDescription`、数组位置或奖励数量兜底关联。

### 3.7 Migration and Compatibility

- 无数据迁移和回滚脚本。
- 新顶级命令是向后兼容增量。
- `task --runAll` 从五项扩展为六项；用户已确认其保持“全部任务默认开启”的兼容语义。无需迁移参数，但必须同步帮助、启动摘要、升级说明和用户文档。
- API 响应新增字段是兼容增量；匿名条目改为 `=` 类型别名，避免普通字段访问和复合字面量发生不必要的源兼容破坏。
- 回滚时可删除新命令注册和 task 选择器；现有 `sign`、Cookie 和配置文件无需迁移。

---

## 4. API Design

### 4.1 CLI Surface

```text
ncmctl yunbei status
ncmctl yunbei --task sign --task claim
ncmctl yunbei --task reserve,listen-indie
ncmctl yunbei --task publish-note --title "今日音乐" --message "..." --image ./cover.jpg
ncmctl yunbei --all
ncmctl yunbei --all --delete
```

| Flag/参数 | 类型 | 默认值 | 契约 |
| --- | --- | --- | --- |
| `status` | 位置参数 | 无 | 唯一允许的位置参数；只读 |
| `--task` | string slice | 空 | 可重复、可逗号分隔；去重后校验英文标识 |
| `--all` | bool | false | 选择全部本地支持任务；与 `--task` 互斥 |
| `--title` | string | 空 | 仅 publish-note/`--all` 有效；非空时 TrimSpace 后仍须非空 |
| `--message` | string | 内置 25 字默认正文 | 仅 publish-note/`--all` 有效；非空时 TrimSpace 后至少 20 个 Unicode 字符（V3 `taskDescription`：「发布一条≥20字的图文笔记」） |
| `--image` | string | 空 | 仅 publish-note/`--all` 有效；JPEG/PNG/GIF/WebP 常规文件，内容与扩展名一致，非空、非符号链接且不超过 20 MiB |
| `--delete` | bool | false | 仅 publish-note/`--all` 有效；只删除本次成功创建且已确认的图文 |

公开任务标识严格固定为 PRD 的十二项：`sign`、`claim`、`reserve`、`view-vip-center`、`like-comment`、`listen-indie`、`follow-artist`、`collect-song`、`like-song`、`publish-note`、`share-song`、`play-daily-recommend`。

任务去重保留第一次出现，但执行顺序始终由注册表阶段决定，不受参数顺序影响。

### 4.2 Task Scheduler Surface

| Flag | 默认值 | 契约 |
| --- | --- | --- |
| `--yunbei` | false | 注册云贝作业 |
| `--yunbei.cron` | `0 12 * * *` | `cron.ParseStandard` 五段式表达式 |
| `--yunbei.task` | 空 | 与直接命令 `--task` 相同 |
| `--yunbei.all` | false | 与直接命令 `--all` 相同 |
| `--yunbei.title` | 空 | 复制到 `YunBeiOpts.Title` |
| `--yunbei.message` | 空 | 复制到 `YunBeiOpts.Message` |
| `--yunbei.image` | 空 | 复制到 `YunBeiOpts.Image` |
| `--yunbei.delete` | false | 复制到 `YunBeiOpts.Delete` |

规则：

- `--yunbei` 必须选择 `--yunbei.task` 或 `--yunbei.all`。
- 任一 `--yunbei.*` 非默认参数被显式提供时，必须同时有 `--yunbei` 或 `--runAll`，防止静默忽略。
- `--runAll` 默认且无条件包含 yunbei，并隐式设置 `All=true`；这是用户确认的兼容语义，不要求额外传 `--yunbei`。
- `--runAll --yunbei.all` 允许，视为同义重复；同时显式提供 `--yunbei.task` 快速失败。显式传 `--yunbei=false` 或 `--yunbei.all=false` 不能关闭 runAll 中的云贝作业，应以范围冲突快速失败。
- 调度器创建新的 `YunBei`，完整复制 options，并调用同一 `validate`/`execute` 路径。
- `--yunbei.cron` 继续由现有 `--location` 指定的 IANA 时区解释，不新增云贝专用时区。
- `Task.validate` 在创建登录 Client 前调用纯函数 `validateYunBeiOpts`；cron 每次触发仍重新校验易变化的本地图片。复制 `Tasks` 时使用 `slices.Clone`，避免父/子命令共享 backing array。

### 4.3 Endpoint Inventory

| 能力 | Wrapper / 路径 | 协议 | 读写 | 当前结论 |
| --- | --- | --- | --- | --- |
| 登录 | `weapi.GetUserInfo` `/weapi/w/nuser/account/get` | WEAPI | 读 | 要求 `Code=200`、`Account!=nil`、`Profile!=nil`、`Profile.UserId>0` |
| 余额 | `weapi.YunBeiBalance` `/weapi/middle/mall/balance` | WEAPI | 读 | 请求必须携带 `currency="credit"`，否则返回假余额 0（见 4.5.4） |
| 完整任务 | `weapi.YunBeiTaskListV3` `/weapi/usertool/task/list/all/v3` | WEAPI | 读 | 已确认 Normal 条目、六个任务 `taskId`、顶层 `subAction` 和单明细进度；其他 bucket/任务身份仍需取证 |
| 待办/领奖 | `eapi.YunBeiTaskTodo` `/eapi/usertool/task/todo/query` | EAPI | 读 | 已确认领奖项 `userTaskId` 对应 V3 `userMissionId`；未完成项可全部为零键且不含动作字段 |
| 签到 | `weapi.YunBeiSignIn` `/weapi/pointmall/user/sign` | WEAPI | 写 | 已有 sign 离线顺序测试 |
| 签到日历/进度 | `YunBeiSignInCalendar`、`YunBeiSignInProgress` | WEAPI | 读 | 已确认由服务端时间戳的当月日序读取 `signStr`；奖励可领取枚举仍需取证 |
| 连签领奖 | `weapi.YunBeiSignLottery` `/weapi/pointmall/user/sign/lottery/get` | WEAPI | 写 | base 仅在 `status=0,id>0` 时领取并刷新确认；extra ID 仍未确认 |
| 任务领奖 | `weapi.YunBeiTaskFinish` `/weapi/usertool/task/point/receive` | WEAPI | 写 | 参数必须直接来自 todo |
| 预约 | `YunbeiReserveInfo/Booked/RewardReceive` | EAPI | 读/写 | 五个已知状态严格分支；预约后刷新确认，领奖仍要求当前响应提供 token |
| 浏览 | `weapi.YunBeiVIPCenterTaskExternal` `interface.music.163.com/weapi/vipnewcenter/app/level/task/external` | WEAPI | 写 | 官方 H5 确认等待 10 秒后提交 `type=1`；请求层把页面相对路径切到 `interface.music.163.com` 后改写为 WEAPI |
| 评论 | `RecommendSongs`、`weapi.Comments`、`CommentLike/Unlike` | WEAPI | 读/写 | 动态日推评论候选、原点赞状态、任务确认与取消点赞补偿已实现；待稳定任务身份 |
| 小众歌曲 | `YunBeiDistributionList/RecommendSong/Create` | WEAPI | 读/写 | 官方 H5 明确把 `/api/` 重写为 `/weapi/`；确认 `singleAmount`、每首 30 秒和 create 顺序，不发送无依据的 `TrialsongListen` |
| 歌手关注 | `eapi.ArtistHot` + `ArtistSub/Unsub` + `weapi.ArtistSublist` | EAPI + WEAPI | 读/写 | 只选未关注候选，任务确认后取消并查询关注集合确认恢复 |
| 歌单收藏 | `RecommendSongs`、`weapi.Playlist/PlaylistDetail/PlaylistAddOrDel` | WEAPI | 读/写 | 同时验证列表和详情所有权，只用普通歌单及原本不存在的日推歌曲；待稳定任务身份 |
| 红心 | `weapi.RecommendSongs/SongLikeList/SongLike` | WEAPI | 读/写 | 官方 H5 确认布尔 `like` 合同；只选未红心日推，任务确认后取消并查询红心 ID 集合确认恢复 |
| 图文 | `EventUploadImage/EventPublish/EventDelete` | EAPI | 写 | 使用带图片的 `noresource` 笔记合同；记录正 event ID 后以 V3 刷新确认，禁止重发 |
| 站外分享 | `DailySongShareTrigger` `/xeapi/music/song/share/trigger` | XEAPI | 写 | 不能证明属于云贝链路，编码阻塞 |
| 日推播放 | `RecommendSongs/SongDetail/SongPlayerV1/WebLog` | WEAPI | 读/写 | 复用已有固定播放载荷，严格按 V3 剩余秒数真实等待并上报实测经过时间 |
| 音乐人身份 | `eapi.MusicianRoleGet` `/eapi/nmusician/workbench/musician/role/get` | EAPI | 读 | 将当前 UID 映射为 artist ID，用于过滤本人作品 |

上表经 `api.Client.Request` 调用的业务 endpoint 默认使用 POST；`EventUploadImage` 还包含 NOS PUT 上传，封面和音频读取使用 GET。新增或修改 EAPI 请求继续嵌入 `types.EApiReqCommon`，由 `api.Client` 注入默认 `e_r/header` 并处理加密响应；命令层不得拼装密文或 XEAPI `B/S/R`。

### 4.4 Response and Success Contract

- 传输成功：`api.Client.Request` 返回 nil error。
- 接口业务成功：该 endpoint 的 `Code`/`Success`/`Data` 满足已经取证的业务规则。
- 动作成功：写接口业务成功，只代表服务端接受动作。
- 任务确认：刷新后的同一稳定任务键满足 `Completed=true`、进入已完成待领取，或证据确认的进度字段严格增长。
- 领取确认：领取接口业务成功后再次刷新，目标记录不再是“已完成待领取”；仅接口 `Data=true` 不足以独立确认最终状态。

任务在刷新后消失，不自动等于完成。只有全量列表明确完成或待办/领取状态按取证契约变化时才可确认；否则为 `unconfirmed`。

### 4.5 Protocol Evidence Gates

下列门禁全部是实现前置条件，不是可在代码中保留的 TODO：

| Gate | 必须确认的证据 | 影响任务 |
| --- | --- | --- |
| G1 | 已确认 `normal.list`、`newbie=null`、六个稳定 `taskId`，以及 todo `userTaskId` ↔ V3 `userMissionId`；仍需其余三个本地任务身份和其他日期可能出现的非空 bucket | 未取得身份的 task-list-backed 任务 |
| G2 | base 三态已由真实状态组合关闭：`0/0` 未达成、`0/id` 可领取、`1/id` 已领取，领取后必须刷新到已领取；仍需 extra 枚举和 `ExtraLotteryId` 领取路径 | sign 的 extra 奖励 |
| G3 | 会员中心任务已由官方 H5 关闭：等待 10 秒后向 `interface.music.163.com` 调用 WEAPI external `type=1`；`YunbeiClickTask` 的 `type/checkToken` 仍无依据，不用于该处理器 | 其他未来 click-task 动作 |
| G4 | 已确认 `NO_PREV_NO_BOOK -> NO_PREV_BOOKED` 的预约路径；五个已知状态之外一律失败，`PREV_BOOKED_UNCLAIMED` 仍需真实 `checkToken` 和领奖后状态转移 | reserve 领奖 |
| G5 | 官方 H5 已确认 `distribution/list` 返回 `times/amount/singleAmount`，实际播放 30 秒后以 `singleAmount` 调用 create；该页面不调用 `TrialsongListen` | 已关闭 |
| G6 | 四类互动处理器均已有动态候选、原状态查询、即时补偿及恢复确认；评论和歌单仍缺稳定任务身份，不能注册写路由 | G1 的身份映射关闭后完成 |
| G7 | 图文发布合同已关闭：`noresource` + 图片元数据，正 event ID 后只允许一次发布并刷新 V3；未确认即非零且不重发。**2026-11-02 复核确认任务达成条件是「正文≥20字」（V3 `taskDescription`），补足长度后服务端会异步创建 `userMissionId` 并计入 `progressRate=1/1`，无需任何归属载荷**（见 4.5.5）；计入延迟的**上界**仍未测定 | publish-note |
| G8 | 云贝站外分享的 URL、协议模式、channel/session 要求和确认字段 | share-song |
| G9 | ~~已关闭~~ **2026-11-03 复核推翻**：`startplay`/`play` 走 `/weapi/feedback/weblog` 返回 `code=200 data=success`，但 `听音乐30分钟` 整轮 27 次 V3 读取恒为 `0/1800`（上报后 +189 秒仍未计入）。日推秒数的真实计数来源未取证，见 Q7；取证前该任务不得声称可用 | play-daily-recommend |

可接受证据：脱敏且获准读取的真实请求/响应、上游公开固定样本、独立固定协议向量。参考项目硬编码、中文标题分支和本地自洽加解密不构成充分证据。

#### 4.5.1 2026-09-05 Read-only Evidence

用户授权提供的两次真实 `yunbei status` / `--all` 预检日志确认：

- `normal.list` 当日包含 `play-daily-recommend=3801300`、`listen-indie=6718451`、`follow-artist=3809301`、`view-vip-center=6758460`、`publish-note=6566461`、`like-song=3804300`；该样本全部 `extInfoMap=null`，因此这些映射使用 `taskId`，不使用中文名称。
- 已完成日推的 V3 顶层 `userTaskId` 与 todo `userTaskId` 不同，todo 值等于唯一 `taskPointDetail.userMissionId`；领奖成功后 todo 中该项消失。
- 未完成的浏览、关注和分享 todo 可同时缺少 `taskId` 且 `userTaskId=0`。它们在 status 中属于未关联展示项，不得升级为整批状态源失败。
- 单个 `taskPointDetail` 的 `progressRate/sumTarget` 分别给出了日推秒数和小众歌曲首数；顶层 `completedPoint/targetPoint` 在同一响应中均为零，不能作为这两类任务进度。
- 当日签到日历的 `signStr` 长度与当月天数一致，服务端时间对应日序位置为 `1`；已领取 base 奖励返回 `status=1` 且 lottery ID 仍非零。
- 同一响应中未来未达成 base 奖励为 `status=0,id=0`；结合已领取的 `status=1,id>0` 和 wrapper 的服务端 ID 合同，唯一剩余的 `status=0,id>0` 作为 base 可领取态。实现只在该组合写入，并要求刷新为 `status=1,id>0`。
- 这些日志没有包含 reserve、点击、互动、小众分配、图文、分享或播放的写请求，因此不能关闭相应写协议门禁。

#### 4.5.2 2026-09-05 Official Public H5 Evidence

对当前线上公开 H5 入口及其静态构建产物做只读检查，未携带 Cookie、未调用账号接口：

- `https://st.music.163.com/yunbei-listen` 的 `main.6be1c084.js`（SHA-256 `2662d97a9bfd161b105639ba56c86cc5f34bdadb2f8c12ad62ac0561da1eb76c`）调用 `distribution/list` 读取 `times/amount/singleAmount`，最多 10 首；单曲播放倒计时固定 30 秒，结束后调用 `distribution/create` 并原样提交 `singleAmount`。其 `encrypt-fetch` 在浏览器路径把 `/api/` 重写为 `/weapi/` 后执行 WEAPI 加密；构建产物没有调用 `TrialsongListen`。
- 同一小众歌曲 H5 的红心控件调用 `/api/song/like`，载荷使用数字 `trackId`、布尔 `like` 以及可选 `alg/rqRefer/songRqRefer`；同一网络管线将它转换为 WEAPI。YunBei 处理器不复用旧 EAPI 字符串载荷。
- `https://music.163.com/g/vip-portal` 的 `m-portal.f39e1.js`（SHA-256 `c2c80c5870b5ab29cfaa4316a7bc3caf0cdd1f39f32d1cf71dc7ee59332b4ea0`）在 `fromyunbei` 路径倒计时 10 秒，然后提交 `/api/vipnewcenter/app/level/task/external` 和 `{type:1}`；其请求层先把 `music.163.com` 页面上的相对 API 地址解析到 `interface.music.163.com`，再默认启用加密并将 `/api/` 重写为 `/weapi/`。
- `https://sg.music.163.com/st/vipsharesong` 的 `main.43f76abc.js`（SHA-256 `4b7e5a2d079b86a34789fb58ada4e1699732615e38f931b8a0002777017e1389`）只负责选择歌曲并打开原生 `orpheus://song?...action=openShare`，没有可由 CLI 等价替代的后端完成请求，因此不关闭 G8。
- 签到进度响应的 `extInfo` 当前指向公开“签到抽奖”页面；页面配置使用通用 `activityId=4262504` 和 `/api/middle/play/lottery/detail`，没有读取 `extraLotteryId` 或调用 sign-lottery endpoint，因此只能证明 extra 奖励会进入独立抽奖活动，不能关闭 G2 的领取路径。

静态产物确认请求合同，不证明当前账号的最终状态转移；实现仍必须在每个写操作后刷新 V3/todo 或专用状态接口。若刷新不确认，结果为 `unconfirmed`，同一动作不得重发。

#### 4.5.3 2026-09-05 Authorized Live Execution Evidence

用户使用当次构建产物执行 `yunbei --all --delete --debug`，并授权读取对应 `ncm.log`。脱敏后的协议结论如下：

- reserve info 初始返回 `NO_PREV_NO_BOOK`，预约接口成功后刷新为 `NO_PREV_BOOKED`，同时给出当前奖励数和倒计时。该状态证明“没有上期待领奖励且本期已预约”，属于已收敛终态，不得再次预约。
- 旧实现向 `music.163.com/weapi/vipnewcenter/app/level/task/external` 提交后得到外层包裹内层的双层成功响应，但随后的三次 V3 读取均未变化，当时据此推断 wrapper 是错误主机产物并改用 `interface.music.163.com` 要求直接布尔 `Data=true`。该推断已被 2026-10-30 授权实测推翻：`interface.music.163.com` 返回的同样是双层包裹 `{"code":200,"data":{"code":200,"data":true,"message":""},"message":""}`，直接布尔假设导致 `json: cannot unmarshal object into ... data of type bool`。响应 DTO 已改为 `RespCommon[RespCommon[bool]]` 双层结构；包裹内的 `data=true` 只代表上报被接受，任务动作是否生效仍以随后的 V3/todo 刷新为准（unconfirmed 不重发）。
- `SongPlayerV1` 返回了无显式端口的 `http://m701.music.126.net/...` 播放地址。命令只允许把初始、已通过网易域名白名单及 userinfo/端口检查的 HTTP URL 原地升级为 HTTPS；任意第三方 HTTP、显式端口以及后续重定向到 HTTP 仍失败。

#### 4.5.4 2026-10-30 Authorized Live Execution Evidence

用户使用含双层包裹修复的构建执行 `yunbei --all --delete --debug`，并授权读取对应 `ncm.log` 与一次 `view-vip-center` 单任务探针：

- `view-vip-center` 上报返回包裹成功后，三次 V3/todo 轮询中 taskId 6758460 始终 `completed=false status=0 progressRate=0/1`，命令按设计得出 unconfirmed。进一步取证发现 `GetUserInfo` 的 `account.vipType=0`（非会员），而该任务描述为「查看会员专属福利，需浏览15秒」。结论：非会员账号上报永远不会被登记，unconfirmed 是必然结果，并会连锁使本轮不安全、阻断无法确认归属的待领奖项。处理器已改为 `vipType==0` 时直接标记 unavailable 跳过，不发送上报。
- 该 unconfirmed 连锁曾两次阻断 claim 的两个无法关联 V3 的已完成 todo（3826123692、3833164222）；非会员跳过后该连锁消除，符合第 587 行「其他已确认独立任务继续领取」的约定。
- 单独执行 claim 的后续运行中，这两个 todo 以「特殊福利」身份出现且携带 `period=0 depositCode=2111`，证明 todo 的 `period` 允许合法零值（回应第 336 行 Phase 0 待定项）；claim 校验已改为仅要求 `userTaskId>0` 且 `depositCode>0`，`period` 按服务端值原样发送，不再把 0 当作字段缺失。修复后的实测运行中两条 `period="0"` 领奖均返回 `{"code":200,"message":"SUCCESS","data":true}`，奖励到账且 todo 列表中条目消失，契约确认。
- `YunBeiBalance` 空请求体返回 `balance=0` 的假余额（CLI 显示「可用 0，冻结 0」而手机 App 显示 2260）；桌面客户端抓包（39.har，eapi 同路径）携带 `"currency":"credit"`。2026-10-30 只读探针实测：空 `currency` 返回 0，`currency="credit"` 返回 2260 与 App 一致。请求结构体已增加必填 `currency` 字段，调用方固定发送 `"credit"`。

#### 4.5.5 2026-11-02 Authorized Live Execution Evidence

用户使用重构后（行为等价）的构建执行 `yunbei --all --debug` 与只读 `yunbei status`，共三次授权运行加一次只读复核，并授权读取对应 `ncm.log`：

- 前两次运行（15:04、15:40）都只调用了一次 `eapi.EventPublish` 并拿到正动态 ID，`publish-note` 得出 `unconfirmed`、命令非零退出且不重发——这正是 G7 发布合同规定的行为，失败保护工作正常。
- 15:04 那次运行内对该账号读取了 20 次 `weapi.YunBeiTaskListV3`，跨 15:04–15:09 约 5 分钟，`taskId=6566461`（发布图文笔记）始终为 `completed=false`、`taskPointDetail.progressRate=0/sumTarget=1`、`completedPoint=0`、`targetPoint=0`、`extInfoMap=null`、`actionType=672`、`subAction=""`；运行末尾的 `eapi.YunBeiTaskTodo` 仍返回该项 `completed=false`、`userTaskId=0`。当时据此得出「发布图文本身不推进该云贝任务」，该结论已被下方 16:35 复核推翻：不推进的原因是正文只有 19 字未达「≥20字」，而不是发布链路缺环节。
- 排查过程中一度怀疑缺少任务归属载荷：乐迷团发布同类图文时提交 `activityInfoList` 绑定活动（`fansgroup.go`），而 `runYunBeiPublishNote` 未提交；`yunBeiActionRef.Type` 与 `CheckToken` 全仓库从未被赋值，`eapi.YunbeiClickTask` 已实现但没有处理器调用。该假设现已被排除——补足正文长度后服务端自行创建了 `userMissionId`。附带更正：`activityInfoList` 从来不是有效对照，因为乐迷团的图文任务只按发布成功与否汇总（spec-003 5.1.6），从不回读任务状态，本身没有被验证过。
- 第二次授权运行（15:40，同一构建）独立复现：`eapi.EventPublish` 返回 `code=200` 与正动态 ID，随后 4 次 `eapi.YunBeiTaskTodo` 中「发布图文笔记」仍为 `userTaskId=0 completed=false`，V3 的 `taskId=6566461` 仍为 `progressRate=0/sumTarget=1`。
- todo 的 `userTaskId` 是判别「服务端是否登记该动作」的关键字段，同轮取证对照：关注歌手在 `0 → 414449014509`、探索小众歌曲在 `0 → 414449053620` 都紧跟各自写操作出现，且完成后 `completed=true`；「发布图文笔记」在两次运行的全部 28 次读取里始终 `userTaskId=0`，即服务端从未为该动作创建用户任务实例——不是进度延迟，而是整条链路没有识别这次发布。todo 列表顺序随机，首项变化不可作为信号。
- 根因取证来自同一份 V3 载荷：`taskId=6566461` 的 `taskDescription` 为「发布一条≥20字的图文笔记」，而命令的内置正文 `分享一首今天听到的好歌，欢迎一起听听。` 去空白后只有 19 个 Unicode 字符，`--message` 的校验下限又是 10 个。正文长度不足时服务端不登记任务，与上面观察到的 `userTaskId` 恒为 0 完全一致。已修复：云贝图文正文下限收紧为 20（`yunBeiNoteMinMessage`），内置默认正文改为 25 字（`yunBeiNoteMessage`），`--message`/`--yunbei.message` 帮助文案同步；每日推歌与乐迷团仍维持 10 字下限，因为≥20 只对该云贝任务有证据。
- 16:25 第三次授权运行（同一构建，正文按默认 25 字发布，`eventID=38038439773`）本轮 publish-note 之后仅剩 claim，确认窗口只有 3 次读取 × 1 秒，读到的仍是 `userTaskId=0 progressRate=0/1`，命令按现状得出 unconfirmed。
- 16:35 的只读 `yunbei status` 给出闭环证据：V3 `taskId=6566461` 为 `completed=true`、`status=100`、`progressRate=1/1`、`taskPointDetail.status=20`、`userTaskId=414454549164`、`taskPointDetail.userMissionId=414454549165`；todo 同步出现「发笔记 `userTaskId=414454549165 completed=true taskPoint=300`」。即服务端在为该笔记创建用户任务实例并计入进度，**不需要任何归属载荷**。
- 对照可排除「延迟计入」解释 19 字那条：15:04 发布的 19 字笔记在 15:40（+36 分钟）的读取里仍是 `progressRate=0/1`、`userTaskId=0`。因此长度是充分条件，异步窗口介于 3 秒与 9.5 分钟之间（上界未测定）。
- 附带取证：`taskPointDetail.status` 的 20 与 100 可按同日数据区分「已完成待领取」与「已领取」（本次 6566461 为 20 且有 300 云贝待领；当天已领取的 3801300/6718451/3809301/3804300 均为 100）。claim 会把本轮 `yunBeiUnconfirmed` 视为不安全任务并阻断归属不明的待领项，因此第三次运行的 `[claim] 今日无需执行` 是预期行为；单独运行 `--task claim` 时进程内 results 为空，可正常领取该 300 云贝。
- 结论：G7 的发布合同与 `publish-note` 的任务达成条件均已关闭（正文≥20 字是唯一缺失条件，Q6 就此结束）。剩余缺陷是确认窗口过短——publish-note 的服务端计入是异步的，现状会在成功计入后仍以非零退出、无法衔接本轮 claim 与 `--delete`；窗口策略另行决策。
- 窗口决策（用户选定）：publish-note 的复核改用专属窗口 `yunBeiNoteObserve`（7 次读取 × 15 秒，最长约 90 秒），其余写操作维持 `yunBeiQuickObserve`（3 次 × 1 秒）；90 秒内仍未确认时**保持非零退出与 partial 归类**，不放宽「写操作未经服务端确认即失败」的合同，仅由文案与 `yunbei status` 复核兜底。实测计入延迟可能超过 90 秒（上界未测定，见 Q6），此时该命令仍会报 unconfirmed，但任务实际已达成，单独 `--task claim` 可领取。
- 排查附带发现：`eapi.EventPublish` 携带 `SetRedactLogs`，`--debug` 下请求与响应正文均不落盘，动态 ID 一度只能靠 stdout。现已在 `runYunBeiPublishNote` 增加 `[yunbei] publish-note code=… eventID=… missionTaskID=… bodyRedacted=true` 结构化 debug 行，把发布动作与未推进的任务直接对上；正文仍保持脱敏，图片元数据不得进入日志。

#### 4.5.6 2026-11-03 Authorized Live Execution Evidence

用户使用正文长度与确认窗口修复后的构建执行 `yunbei --all --debug`（10:50:51–11:00:12），并授权读取对应 `ncm.log`。该轮同时验证了 4.5.5 的两项修复，并推翻了 G9：

- 各任务「写操作 → V3 计入」的实测时延（同一轮内可比）：关注歌手 +1.4 秒、红心歌曲 +1.1 秒、探索小众歌曲每次 `distribution/create` 后约 +0.1 秒逐首累计（`1/10 → 10/10`）、**发布图文笔记 +15.8 秒**（`10:50:59.396` 发布 → `10:51:15.186` 计入 `1/1`）。图文这条落在 `yunBeiNoteObserve` 的 7×15 秒窗口内，命令本轮确认成功并进入待领取，随后 claim 领取 300 云贝——4.5.5 的正文长度结论与窗口决策均得到实测支持。计入下界由此从「>3 秒」收紧为「约 15 秒量级」，90 秒窗口有充分余量。
- `taskPointDetail.status` 的取值由本轮补齐为四档：`0` 未开始、`10` 进行中且未完成（探索小众歌曲 `1/10`–`9/10` 全程为 10）、`20` 已完成待领取、`100` 已领取。
- **G9 被推翻**：`听音乐30分钟`（taskId 3801300）在整轮 27 次 V3 读取里恒为 `progressRate=0/1800`、`detailStatus=0`、`userTaskId=0`。处理器确实跑完整链路——`startplay` 上报（`10:53:18.430`）→ `song/detail` → `SongPlayerV1` → 取音频 → 真实等待 224 秒（当日剩余 1800 秒，单曲时长 224 秒为上限）→ `play` 上报 `time=224 end=playend`（`10:57:02.659`），两次 `/weapi/feedback/weblog` 均返回 `{"code":200,"data":"success"}`，但服务端进度纹丝不动，最后一次读取在上报 +189 秒之后。因此「复用仓库现有 startplay/play 载荷即可累计日推秒数」不成立：`code=200 data=success` 只表示遥测被接收，不表示云贝计数。同轮对照里，凡走云贝专属上报路由（`ad/power/yunbei/distribution/create`）的任务都在亚秒级计入，说明该任务的计数来源是另一条链路或另一组载荷字段（`source`/`sourceId` 是否必须是日推场景、是否需要播放心跳类端点均未取证，见 Q7）。G9 此前依据的是仓库内既有载荷样本注释，属 spec 4.5 明示的「参考项目硬编码不构成充分证据」，本轮为其反例。
- `reserve` 侧本轮触发的是实现自身的快速失败：`eapi.YunbeiReserveInfo` 返回 `type=PREV_BOOKED_UNCLAIMED` 但 `data.reqId`/`data.checkToken` 为空，处理器按 G4 约定拒绝写入。该响应解密后 817 字节，远大于当前结构体的三个字段，说明 `data` 里另有载荷未被映射；但 `YunbeiReserveInfo` 挂了 `SetRedactLogs`，`--debug` 下正文不落盘，日志无法回答「字段到底叫什么、嵌在哪一层」。Q3 因此仍无法凭日志关闭，需要该端点的脱敏响应正文（或等价的键名结构取证）。
- 汇总口径：`success=6 skipped=4 partial=1 failed=1`。`partial` 是 play-daily-recommend（动作成功、状态未变），`failed` 是 reserve（缺 `reqId`/`checkToken`）。
- 处置决策（用户选定）：`yunBeiTaskDef` 新增 `EvidenceGap`，非空时 `runYunBeiDefinition` 在调用处理器之前就把结果记为 `unavailable` 并打印「取证未闭环，已跳过」，归入 `skipped` 而非 `failed`——失败保留给真实的写入/校验错误，同时不再为已证伪的链路付出长时间播放的流量与等待。play-daily-recommend 据此挂起，Q7 取证后清除标记即可恢复；处理器与其单元测试保持原样，避免取证后重写。
- 配套取证能力：`api` 层在 `redactLogs` 的响应上新增 `[response.shape]` debug 行，只输出解密正文的**键路径**（键名与层级，取值一律不落盘，深度 4、条数 64 上限）。这是 Q3 这类「正文被刻意脱敏、但需要知道未映射字段叫什么」场景的日志内取证手段；`YunbeiReserveInfo` 的下一次 `--debug` 运行即可直接看到 `data` 的真实字段布局。

### 4.6 Breaking Changes

无 CLI 语法破坏。`task --runAll` 增加一个高副作用作业；用户已确认这是 `runAll` 的兼容扩展，默认启用且不增加交互确认。升级说明、help 和服务启动摘要仍必须醒目标注公开发布、互动、流量与可能超过 45 分钟的运行时间。

API DTO 仅允许增量字段或类型别名重构。若 Phase 0 发现必须改变既有字段类型，应单独记录兼容性决策并增加旧 JSON 样本回归测试，不能在本功能中静默完成。

---

## 5. Business Logic

### 5.1 Validation

所有校验在 `api.NewClient` 和任何账号请求之前完成：

1. 位置参数只能为空或单个字面量 `status`。
2. 执行模式必须有非空有效的 `--task` 或 `--all`。
3. `--task` 和 `--all` 互斥；空白、未知标识直接报错。
4. `status` 与 `--task/--all/--title/--message/--image/--delete` 的显式使用互斥；用 Cobra `Flag.Changed` 识别显式的零值 flag。
5. publish 参数仅在选择 `publish-note` 或 `--all` 时有效。
6. `validateNoteContent` 校验 title/message/image 的基础规则，正文下限由调用方按任务传入（`messageMin`）：云贝图文笔记为 20（服务端 `taskDescription` 要求「≥20字」），每日推歌与乐迷团维持 10；`validateYunBeiImage` 再要求本地图片为内容与扩展名一致的 JPEG/PNG/GIF/WebP 且大小 `<= 20 MiB`。message 使用 `utf8.RuneCountInString`。
7. 调度模式先完成选择、选项和所有 cron 校验，再创建 cron、客户端或注册任何 job。
8. cron 每次执行前重新检查本地图片仍是同一个非符号链接、非空、`<= 20 MiB` 的常规文件；启动后文件被替换时本轮失败且不发账号写请求。

### 5.2 Snapshot Loading

登录是唯一无条件前置：调用 `GetUserInfo` 后要求 `Code=200`、`Account!=nil`、`Profile!=nil`、`Profile.UserId>0`，保存 UID 和昵称；昵称只用于账号摘要。`YunBeiBalance.Data.UserId` 非零且与登录 UID 不一致时，余额结果无效。

随后根据已选择任务建立最小 query plan：

| 模式/任务 | 必需查询 | 无关查询故障的处理 |
| --- | --- | --- |
| `status` | 余额、签到/连签、reserve、V3 全部 bucket、todo | 任一核心查询失败则 status 非零，不输出伪完整状态 |
| `sign` | 签到状态与连签进度 | 不调用余额、reserve、V3 或 todo；这些接口故障不得阻断 sign |
| `reserve` | reserve info | 不依赖余额、签到、V3 或 todo |
| `claim` | todo；同次运行已有常规任务处于 unconfirmed/failed/unsupported 时额外读取 V3 | 单独执行 claim 不依赖余额、签到、reserve 或 V3；组合执行只用 V3 阻止不安全任务领奖 |
| 常规任务 | V3 全部 bucket + todo；再按处理器加载候选接口 | 余额和未选择能力不是前置条件 |

组合执行时先完成 sign/reserve，再加载常规任务快照；因此后续 V3/todo 故障不会撤销或阻断已经选择的 sign，命令最终仍聚合后续错误为非零。未知任务保留服务端 `missionCode`、`taskId` 和名称并标为 unsupported。无法关联的 todo 只隔离自身，不得让其他具有稳定 V3 身份的任务共同失败。

“任务确认”和“领奖确认”必须重新获取其权威状态源，不能只修改本地状态。写后权威刷新失败的处理仍按第 6 章错误合同执行。

### 5.3 Fixed Execution Order

| 阶段 | 顺序 | 条件 |
| --- | --- | --- |
| Account | sign → reserve | 只在被选择时执行；使用各自 query plan，不依赖常规任务出现 |
| Fast | view-vip-center → like-comment → follow-artist → collect-song → like-song → publish-note → share-song | 服务端存在、未完成且已选择；互动确认后立即恢复，图文按 `--delete` 立即删除 |
| Slow | listen-indie → play-daily-recommend | 快任务完成一轮后执行 |
| Claim | claim | 仅显式选择 claim 或 `--all` |
| Cleanup | pending delete intents → remaining compensations | 只处理异常、取消或即时清理失败后仍未完成的项目 |

单个动作失败继续其他已具备可信状态的选择项。登录失效立即终止；某个权威状态源或关联失败时，只终止依赖该状态源的阶段，已经完成的独立 sign/reserve 结果保留并参与最终汇总。写后权威刷新失败是否升级为停止所有后续写入，按第 6 章错误合同处理。

### 5.4 Three-Round Convergence

每轮只执行当前快照中仍为 pending 的任务。每个处理器动作后先进入独立的只读观察窗口，再决定是否允许后续动作：

```text
before = stable snapshot
run one bounded action
after = reload task state (immediate + at most 2 delayed polls)

if after completed:
    mark confirmed
else if verified progress(after) > verified progress(before):
    mark progress and allow next action/round
else:
    mark unconfirmed; do not repeat irreversible action
```

- 全部处理器最多经过 3 个外层轮次。
- 确认窗口总计最多 3 次只读观察；延迟使用对应协议 gate 固化的命名常量并响应取消。观察期间绝不重复写请求，第 3 次仍无变化才标 unconfirmed。
- 一轮没有任何已确认进度变化时立即停止。
- `publish-note` 成功得到 event ID 后，本次运行不得再次发布，即使状态暂未更新。
- `share-song` 是否可重复触发及重复上限必须由 G8 明确；缺证据时只允许一次触发并标 unconfirmed。
- 互动处理器每完成一个目标就刷新进度，不能提前对一批目标全部写入后才查询。
- “今日未出现”标 unavailable，正常跳过且不发写请求。

### 5.5 Sign and Reward Claim

签到：

1. 调用 `YunBeiSignIn`；新签到和已签到都属于可继续的幂等结果。
2. 获取连签进度；base 只对 `status=0,id>0` 调用 `YunBeiSignLottery`，`status=0,id=0` 跳过，`status=1,id>0` 视为已领取，其他组合失败。
3. extra 奖励独立分类；出现可领取 ID 时在 G2 关闭前失败，不复用 base endpoint 猜测领取。
4. 领取后刷新连签进度并输出已领取/仍待领取。

任务领奖：

1. 只遍历 todo 中明确 completed 且尚待领取的记录。
2. `period/userTaskId/depositCode` 全部直接使用服务端值；任一缺失则该项失败，不构造默认值。
3. 单项失败继续下一项，最终聚合为非零。
4. 重复 `userTaskId` 的记录全部拒绝，不选择其中任意一条猜测领取。
5. 每次成功后刷新状态；下一项写入前必须仍在最新 todo 中唯一存在且处于待领取状态，未确认移除当前待领取状态时标 unconfirmed。
6. 同次运行的常规任务若为 unconfirmed/failed/unsupported，必须通过最新 V3/todo 关联阻止对应奖励；无法确认归属的待领奖项也不在该不安全组合中写入，其他已确认独立任务继续领取。

新命令可以复用请求 DTO，但不得直接调用当前 `yunbeiClaim` 后假定其错误和刷新语义满足本 SPEC。现有 `sign` 的请求顺序、输出和“部分错误只记录”行为保持不变。

### 5.6 Reserve

1. 读取 EAPI reserve info。
2. `NO_PREV_NO_BOOK`、`PREV_CLAIMED_NO_BOOKED` 使用当前 `reqId` 预约；`NO_PREV_BOOKED`、`PREV_CLAIMED_BOOKED` 均表示本期已经预约，正常结束且不再写入。
3. `PREV_BOOKED_UNCLAIMED` 只有在当前响应同时提供非空 `reqId/checkToken` 时领奖；缺字段时不发送写请求。
4. 未识别状态按 unsupported 失败，不通过字符串包含关系猜测动作。
5. 每次写操作后重新读取 info，再决定下一步；同一 `reqId` 的同类动作不得重复。
6. 无活动、活动结束、已领取或已预约正常跳过。

### 5.7 View VIP Center

1. 只在 V3 稳定 `taskId=6758460` 当日出现且未完成时执行。
2. 按官方 H5 合同通过 `utils.Sleep` 等待 10 秒，再向 `interface.music.163.com` 调用 WEAPI `YunBeiVIPCenterTaskExternal(type=1)`。
3. 分别检查 transport error、外层 `Code=200` 和包裹内 `Data=true`；不调用旧 `YunbeiClickTask`，也不构造 `subAction/type/checkToken`。
4. 刷新同一稳定任务键。只有进度增长或完成才报告成功；否则标 unconfirmed，本次运行不重复上报。

### 5.8 Reversible Interaction Tasks

四类互动遵循同一事务规则：

1. 从任务上下文或当天服务端推荐动态选择候选，保持服务端顺序；不使用固定 ID。
2. 查询候选原状态；原状态已满足者跳过，绝不先归一化为 false。
3. 写操作业务成功后立即登记补偿项。
4. 刷新云贝任务状态；达到目标后停止新增动作。
5. 任务确认后立即逆序恢复该任务本次新增的状态；处理器因未确认或错误返回时，编排层也要在进入下一任务前立即尝试恢复。只有当前 context 已取消或恢复失败时才留给最终独立清理 context 兜底。
6. 恢复后重新查询目标原状态；若恢复未确认或任务完成状态发生回退，则标记 partial、不得领取该任务奖励并返回非零。

| 任务 | 候选过滤 | 正向动作 | 补偿动作 |
| --- | --- | --- | --- |
| like-comment | 非本人、未点赞、comment/thread ID 有效 | `CommentLike` | `CommentUnlike` |
| follow-artist | `ArtistHot.followed=false`、artist ID 有效 | `ArtistSub` | `ArtistUnsub` 后完整分页读取 `ArtistSublist` 确认不存在 |
| collect-song | 当前 UID 拥有写权限的普通歌单，歌曲原本不在其中 | `PlaylistAddOrDel(op=add)` | `PlaylistAddOrDel(op=del)` |
| like-song | 日推候选且 `SongLikeList.ids` 中不存在 | `SongLike(like=true)` | `SongLike(like=false)` 后再次查询 ID 集合 |

候选少于剩余目标时处理全部可用项，并根据刷新结果标 completed、partial/unconfirmed；“没有未处理候选”是 skipped，不能撤销用户原状态。

### 5.9 Listen Indie

1. 调用 `distribution/list` 读取当天 `times/amount/singleAmount`，并与 V3 单明细进度交叉校验；目标只接受 `1..10`。
2. 从 `distribution/recommend/song` 按当前 `times` 分批读取未完成歌曲，获取标准音质播放地址并做受限音频读取；播放器返回无端口的网易 CDN HTTP 地址时，初始请求先升级为 HTTPS。
3. 每首通过 `utils.Sleep` 真实等待 30 秒，随后调用 `distribution/create`，`yunbeiAmount` 必须原样使用当前 status 的 `singleAmount`。
4. create 后立即刷新 distribution status，只有 `times` 严格增长才继续；同时刷新 V3，达到目标立即停止。
5. 不发送官方 H5 未使用的 `TrialsongListen`，不写固定 `150`，也不从 V3 奖励展示值反推单次 amount。
6. 所有等待响应取消；同一歌曲的 create 指纹在一次运行中最多执行一次。

### 5.10 Publish Note

1. 从任务上下文或每日推荐选取歌曲；候选必须有有效 ID、名称和封面。
2. `--image` 优先，否则调用命令内的 `downloadYunBeiMediaToTemp` 下载封面；它沿用 `downloadImageToTemp` 的流式、`limit+1` 和临时文件清理模式，同时执行第 7.2 节的每跳 URL/IP 校验。大小限制使用 `dailySongCoverLimit`，所有路径 defer cleanup。
3. 标题/正文优先使用用户值，否则生成包含歌曲信息且正文不少于 20 个 Unicode 字符的自然文本（服务端按「≥20字」判定该任务是否达成）。
4. `EventUploadImage` 后构造带图片元数据的 `EventPublishReq(type=noresource)`，成功必须得到正 event ID。
5. 记录 event ID 后刷新任务；不论是否确认，本次运行都不重复发布。
6. 默认保留。`--delete` 在任务确认后立即删除该 ID，不等待慢任务或领奖；确认后先登记 delete intent，删除成功再清除，确保取消路径仍可继续清理。
7. 发布成功但未确认或删除失败均输出 event ID，返回非零，不再次发布。

### 5.11 Share Song

G8 关闭前，该已知任务只能展示为“接口未对齐”，不能调用 `DailySongShareTrigger` 或乐迷团 forward/progress 代替。

G8 关闭后：

1. 资源 ID、channel 和动作参数全部来自任务元数据或当天推荐。
2. wrapper 固定使用证据确认的 URL 和 WEAPI/EAPI/XEAPI 模式。
3. 动作业务成功后按证据要求等待，再刷新任务状态。
4. 只有服务端进度增长/完成才是成功；否则输出“分享已触发，任务未确认”，不领奖并返回非零。

CLI 不声称完成真实社交平台交互；它只报告云贝中心是否确认了对应任务。

### 5.12 Play Daily Recommend

1. 调用 `MusicianRoleGet` 建立当前 UID 与 artist ID 的映射：明确非音乐人时无需过滤；音乐人且 UID 匹配时过滤包含该 artist ID 的歌曲；查询失败、UID 不匹配或音乐人却缺少有效 artist ID 时保守失败，不开始播放。
2. 获取日推，过滤当前音乐人自己的作品、无播放权限、无有效地址项目。
3. 从任务状态读取剩余秒数；缺失或语义不明时失败，不回退固定 31/45 分钟。
4. 对每首歌执行：detail → startplay → player URL → 受限读取 → context-aware 真实等待 → play 上报。
5. 上报 `time` 使用 runtime `now` 计算单调时钟实际经过整秒数，并不超过歌曲时长；取消时不补发未经过的时长。
6. 每首结束刷新任务状态；完成立即停止，未完成继续下一首。
7. 响应体在每次迭代内关闭，不在循环中累积 defer。

命令内 `fetchYunBeiAudioSample` 沿用 `fetchAudioSample` 的 Range、限量读取和及时关闭模式，并增加第 7.2 节的每跳 URL/IP 校验。播放载荷复用仓库已有固定样本，但等待时长必须来自 V3 剩余秒数，不能复用 fansgroup 的 3～5 秒模拟值。

### 5.13 Cancellation and Compensation

`YunBei.execute` 建立 `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`，核心逻辑放在可直接传入测试 context 的 `run` 方法中。

退出时：

1. 停止新的任务动作，关闭当前图片/音频响应体。
2. 使用 `context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)` 创建独立有界清理 context。
3. 先执行已经确认且用户要求的 pending delete intents，再按写入逆序恢复未完成补偿；每项都记录结果。
4. 在同一清理 context 内关闭 API Client，保证补偿早于 Cookie/XEAPI 持久化关闭。
5. 清理超时和恢复失败进入最终非零结果；API Client 关闭错误沿用 `closeAPIClient` 的现有契约记录日志，不额外改变命令结果。

`cleanupTimeout` 固定为 60 秒。该总预算覆盖全部网络补偿，不为每项重新获得 60 秒；超时后输出仍未恢复的目标 ID。

### 5.14 State Machine

```text
unavailable -------------------------------> skipped(exit 0)
pending -> running -> completed-unclaimed
                 -> immediate interaction restore / optional note delete
                 -> claimed(claim selected and confirmed, exit 0)
                 \-> done(no claim selected, exit 0)
                 \-> no verified change -----------------> unconfirmed(exit nonzero)
                 \-> action error -----------------------> failed(exit nonzero)
unknown server task ------------------------> unsupported(display only)

any state + compensation/delete failure ----> partial(exit nonzero)
any running state + cancellation ------------> cleanup -> failed(exit nonzero)
```

### 5.15 Output Contract

状态模式至少输出：账号昵称/UID、可用和冻结云贝、今日签到状态，以及每项任务的服务端名称、本地标识（未知时 `-`）、`taskId/userTaskId/actionType/subAction`、进度、奖励和领取状态。动作元数据只用于诊断和固定样本，不得绕过注册表身份校验。

执行模式在首个写请求前输出所选任务及副作用摘要。每项输出：执行前进度、动作结果、执行后进度、确认状态、领奖结果和恢复结果。最终汇总按 `success/skipped/partial/failed` 计数；不输出 Cookie、`checkToken`、XEAPI session、加密载荷、评论正文或完整动态正文。

---

## 6. Error Handling

### 6.1 Error Taxonomy

| 类别 | 条件 | 是否继续 | 退出 |
| --- | --- | --- | --- |
| 参数错误 | flag/位置参数/文件/cron 不合法 | 否；请求前失败 | 非零 |
| 登录错误 | transport、`Code!=200`、`Account=nil`、`Profile=nil` 或 `Profile.UserId<=0` | 否 | 非零 |
| 选择项状态源错误 | 当前阶段所需查询失败或关联冲突 | 停止依赖该状态源的阶段；不阻断已先执行的独立 sign/reserve | 非零 |
| 今日不可用 | 已选择任务未出现在服务端 | 是 | 正常 skipped |
| 已完成/已领取 | 幂等状态 | 是 | 正常 skipped |
| 接口未对齐 | 已知处理器的协议门禁未关闭 | 是，执行其他任务 | 非零 failed |
| 动作失败 | transport 或业务状态失败 | 是 | 非零 failed |
| 未确认 | 动作成功但状态无预期变化 | 是，不领奖 | 非零 partial |
| 领奖失败/未确认 | 某项领取错误或刷新未确认 | 是 | 非零 partial |
| 恢复/删除失败 | 补偿或 `--delete` 失败 | 继续清理其余项 | 非零 partial |
| 取消/清理超时 | context 取消或 60 秒预算耗尽 | 停止新动作，尽力清理 | 非零 |

最终错误使用 `errors.Join` 聚合，但终端逐项输出保持简洁。不得因为一个任务失败覆盖另一个任务或清理错误。

### 6.2 Retry Strategy

- 传输重试只使用 `api.Client` 的既有 `Network.Retry`，命令层不对非幂等写操作叠加重试。
- 最多 3 轮是“经服务端确认有进度后的业务收敛”，不是盲目重试。
- 无进度变化的写操作不在本次运行重发。
- cron 单次失败只记录，下一周期重新从服务端状态开始；不做紧密重试。

### 6.3 Failure Recovery

- 补偿日志在每次成功写入后立即登记，避免后续 panic/error 漏恢复。
- 图文 ID 单独记录；默认不删除，`--delete` 也不得删除非本次创建的 ID。
- 进程被强制终止、机器断电或超过清理预算时无法保证恢复；输出和文档必须明确这是剩余风险。
- 单独启动的多个 ncmctl 进程不共享互斥锁；本 SPEC 只保证同一个 `task` 进程内互斥。

---

## 7. Security

### 7.1 Authentication and Authorization

- 所有模式先通过 `GetUserInfo` 验证登录；写操作只使用当前 Cookie 会话。
- 选择歌单时要求 `PlaylistRespList.UserId`/`Creator.UserId` 与当前 UID 一致，并满足 G6 确认的普通可写条件。
- 不接受命令行 Cookie、token、`checkToken` 或 XEAPI key 参数。

### 7.2 Input and Resource Safety

- 本地图片沿用 `validateLocalImage`：`Lstat`、拒绝符号链接、必须常规且非空。
- `validateYunBeiImage` 在基础检查后检测文件签名，只接受内容与扩展名一致的 JPEG/PNG/GIF/WebP，并拒绝大于 `dailySongCoverLimit`（20 MiB）的文件；远程下载按内容签名选择临时文件后缀，并按 `limit+1` 检测超限。
- 自动封面和音频的实际网络请求仅使用 HTTPS，host 必须是 `music.163.com`/`music.126.net` 或其子域；播放器或封面接口返回无显式端口的白名单 HTTP 地址时，仅初始地址可升级为 HTTPS，不开放任意远程 URL flag。
- 命令内媒体 Client 最多跟随 5 次重定向，每一跳重新校验 scheme/host；自定义 `DialContext` 对解析出的每个 IP 拒绝 unspecified、loopback、private、link-local 和 multicast 地址，防止重定向降级、DNS rebinding 和内网访问。
- 下载沿用 `http.Client` 默认证书校验、配置超时、Range 和字节上限；不复制 API Client 当前的 `InsecureSkipVerify` 缺陷。目标不满足策略时失败并提示用户显式提供安全的 `--image`。
- 所有 ID 在使用前要求正值；字符串 ID 不经 float 转换。

### 7.3 Sensitive Data

- `checkToken`、Cookie、CSRF、XEAPI session、签名和密文不得进入 info/error 日志；reserve info、NOS token/上传和动态发布使用请求级日志抑制，避免 `--debug` 记录解密后的 token、敏感请求头或动态正文。
- Debug 日志也只记录 endpoint 操作名、非敏感 ID、状态和耗时。
- `task` 的 debug 启动日志只记录作业选择与时区，不格式化包含 title/message/image 的完整 options。
- 评论输出不包含正文和用户资料；恢复失败只输出资源 ID 与动作。

### 7.4 Account Risk

- `--all` 开始前明确列出发布、点赞、关注、收藏、红心、站外分享和长时播放。
- 不提供跳过服务端确认、伪造时长、固定第三方对象或绕过验证码/反作弊的参数。
- 自动恢复只能减少持久状态变化，不能承诺消除风控记录。

---

## 8. Performance

### 8.1 Expected Load

命令面向单账号、低频人工或每日 cron。单次状态查询为常数级接口调用；执行请求量与当天任务目标线性相关。最坏运行时间由日推剩余时长决定，可能超过 45 分钟。

### 8.2 Resource Limits

- 全流程仅一个 `api.Client`，任务串行。
- 快照和候选只保存在内存，空间复杂度 O(任务数 + 候选数)。
- 图片沿用 20 MiB 上限；音频采样沿用现有播放链路的 512 KiB 上限，不下载整首到磁盘。
- 状态收敛最多 3 轮，无变化立即退出。
- 长任务进度按歌曲或证据确认的最小批次输出，禁止按秒刷日志。

### 8.3 Concurrency

- 一次性命令内部不并发执行任务。
- `task` 的 sign/yunbei 共用同一 gate；每个注册作业还持有独立 `atomic.Bool`，覆盖“等待 gate + 执行”的完整生命周期。新触发若发现同名作业已经运行或等待重试，立即提示并跳过，不加入等待队列。
- 共享锁的获取和释放包围最外层 `ExecuteContext`，不得让两个 cron 回调并发进入同一个 Cobra command。
- 固定常量为 `signYunBeiGateRetryCount = 3`、`signYunBeiGateRetryInterval = 30 * time.Second`。每次触发先 `TryLock` 一次；失败后通过可取消等待重试。任一次成功即执行；仍失败则输出“sign/yunbei 仍在运行，本次已重试 3 次并跳过，不会补跑”，记录 warning 后返回。
- 跳过只影响当前 cron 触发，不创建延迟补跑；下一次正常 cron 周期仍按计划触发。其他既有作业保持当前并发行为。
- 必须用 channel 控制的 race 测试验证 unlock 和下一周期恢复，不能依赖短 sleep 碰运气。

调度器建立独立 `runCtx`。关闭 hook 的固定顺序是：`cancel()` → `job.Stop()` → 等待返回 context；等待上限 `taskShutdownTimeout = 65s`，覆盖 yunbei 的 60 秒清理预算并留出收尾时间。超时写入明确错误后返回，不能永久阻塞服务退出。

---

## 9. Testing Strategy

### 9.1 Pure Unit Tests

- `--task` 重复、逗号混合、稳定去重、未知/空值、与 `--all` 冲突。
- `status` 与所有写 flags 的显式互斥，包括 `--delete=false`。
- publish 参数适用范围、Unicode 长度、图片目录/空文件/符号链接、伪图片、扩展名不匹配、恰好 20 MiB 和超过 20 MiB。
- 媒体 URL 校验覆盖白名单初始 HTTP 升级、非 NetEase HTTP 拒绝、重定向超过 5 次、HTTPS 降级、loopback/private/link-local/multicast IP 和合法 NetEase CDN。
- 注册表固定阶段顺序，不受输入参数顺序影响。
- 快照关联：userTaskId 优先、taskId 回退、冲突/重复/零值安全拒绝、未知 missionCode 展示。
- 状态机和退出聚合：全 skipped 为 0；unconfirmed/restore/delete/claim 任一失败为非零。
- 补偿日志只登记本次新增动作、逆序执行、成功项不重复、取消 context 后使用独立清理 context。
- runtime 注入固定 `now`、立即 `sleep` 和手动 timeout；断言生产构造函数使用标准实现，测试不等待真实随机时长。

### 9.2 Offline API Contract Tests

通过 `Client.SetTransport` 和固定响应覆盖：

- WEAPI/EAPI URL、crypto mode、请求 JSON 字段与字符串/数字类型。
- `EventUploadImage` 的 NOS 步骤使用 PUT，普通业务 wrapper 使用各自已确认的方法；测试不把所有请求硬编码为 POST。
- V3/todo DTO 对完整、缺字段、未知附加字段和畸形响应的解析。
- V3 的 Normal/Newbie 等所有已确认 bucket 均进入完整列表；非空未知 bucket 在执行模式 fail closed。
- 领取三元组覆盖字段缺失、显式 0、最大值和合法范围；需要时验证 presence-aware 解码。
- transport error 与 `Code != 200` 分开传播。
- `checkToken` 不出现在格式化错误和日志捕获中。
- sign 请求顺序与现有 `sign_test.go` 契约相互独立，不修改旧断言。

### 9.3 Orchestration Tests

- status 只出现白名单查询路径，零写请求。
- 未选择任务和未知任务在任何网络请求前失败。
- 登录响应覆盖 `Account=nil`、`Profile=nil`、`Profile.UserId<=0`；status 的余额 UID 非零且不匹配时失败，不继续展示混合账号数据。
- `--task sign` 的路径白名单只包含登录、签到和连签接口；即使 balance/V3/todo/reserve fake 被配置为失败也不调用、不阻断。组合任务验证 sign 先完成，随后常规快照失败时整体返回非零。
- 只执行用户选择且服务端 pending 的任务；未出现任务不写入。
- 单任务失败后继续；最终 `errors.Join` 包含全部失败。
- 动作 Code=200 但刷新不变 -> unconfirmed、零领奖、非零退出。
- 刷新有进度 -> 下一轮；无进度提前停；永不超过 3 轮。
- 原状态为 true 的互动目标完全不写；原状态为 false 时写入、确认、逆序恢复。
- 发布成功只调用一次；默认不删，`--delete` 只删本次 ID；未确认不重复发布。
- context 取消停止新请求、关闭 body、执行补偿并返回 context error。
- 对注册表中的每个任务做表驱动覆盖：今日未出现、已完成、成功完成、业务失败、状态未确认、取消；有临时状态的任务另覆盖恢复失败，其他任务明确标记为不适用而不是省略。
- `--all` 捕获输出和 fake transport 时序，断言公开发布、点赞、关注、收藏、红心、站外分享和长时播放摘要全部出现在首个写请求之前。
- 恢复失败捕获输出，断言包含目标 ID 和恢复动作，但不包含评论正文、动态正文或 token。
- 音乐人身份覆盖非音乐人、当前 UID/artist ID 正常映射、UID 不匹配、缺少 artist ID 和查询失败；后三者不得开始播放。
- 异步任务状态在第 2/3 次只读观察才更新时可确认；观察期间物理写请求始终只有一次，取消立即结束观察。

### 9.4 Scheduler Tests

- `--yunbei` 缺 task/all、非法 cron、runAll 与 yunbei.task/显式 false 冲突均在创建登录 Client 和注册 job 前失败。
- 父命令首次校验通过后替换/删除 `--yunbei.image`，cron 触发时必须重新校验并在任何账号写请求前失败。
- 单选 yunbei 只注册一个作业；`--runAll` 默认选择六项且 YunBei opts 为 All，无需额外 selector。
- `task --runAll` 的 help 和启动摘要明确包含公开图文、互动、网络流量及可能超过 45 分钟；摘要在 `job.Start` 前输出。
- 完整复制 task/title/message/image/delete options；修改子命令 Tasks 不得改变父命令 slice。
- gate 测试注入立即/手动等待：首次失败后最多重试 3 次；中途释放会执行；始终占用则只提示、跳过且不注册补跑；下一正常周期仍可执行。
- 同一作业处于重试等待时再次触发，新回调必须由 `atomic.Bool` 立即拒绝；断言最多一个 waiter，释放后不存在排队补跑。
- 锁必须包围最外层 `ExecuteContext`，跳过路径不输出 execute success，也不并发进入同一 Cobra command。
- shutdown 先 cancel，再等待 in-flight job 完成；超时路径可控且不会永久阻塞测试。
- `go test -race ./internal/ncmctl` 覆盖 gate、结果和补偿状态。

### 9.5 Acceptance Criteria Mapping

| AC | 主要测试 | 类型 |
| --- | --- | --- |
| AC-001 | 无选择时网络调用计数为 0 | unit/orchestration |
| AC-002 | status 路径白名单 + 输出字段 | orchestration |
| AC-003 | 仅 view handler，刷新确认 | orchestration |
| AC-004 | 注册表阶段顺序 + 多任务失败聚合 | orchestration |
| AC-005 | 任务缺席时写请求为 0、exit 0 | orchestration |
| AC-006 | 已签到仍查询/领取连签奖励 | offline flow |
| AC-007 | todo 原始参数逐项领奖并刷新 | offline flow |
| AC-008 | 未关注 -> 关注 -> 确认 -> 取消关注 | offline flow |
| AC-009 | 已红心候选零 SongLike 请求 | offline flow |
| AC-010 | 发布一次、确认、零删除 | offline flow |
| AC-011 | 只删除本次 event ID | offline flow |
| AC-012 | 分享动作成功但状态不变 -> 非零且零领奖 | offline flow |
| AC-013 | 取消时停止、关闭、恢复、非零 | cancellation |
| AC-014 | 一个任务失败后其他任务仍运行 | orchestration |
| AC-015 | 未知 missionCode 展示/跳过且无处理器调用 | unit/orchestration |
| AC-016 | yunbei 单选、默认 cron 12:00、options 复制 | scheduler |
| AC-017 | runAll 默认六项、yunbei All=true、风险帮助/启动文本 | scheduler/help |
| AC-018 | sign/yunbei gate 重试、最终提示和下一正常周期恢复 | scheduler/race |

### 9.6 PRD Coverage Mapping

| 需求 | SPEC 章节 |
| --- | --- |
| US-001 / FR-01 | 4.1、5.1 |
| US-002 / FR-03、FR-14 | 5.3、5.4、5.15、6.1 |
| US-003 / FR-02 | 2.3、5.2、5.15 |
| US-004 / FR-07、FR-09、FR-10、FR-13 | 3.4、5.8、5.10、5.13 |
| US-005 / FR-04、FR-11 | 4.4、5.4、5.5、5.11 |
| US-006 / FR-15 | 4.2、8.3、9.4 |
| FR-05、FR-06、FR-08、FR-12 | 5.6、5.7、5.9、5.12 |

### 9.7 Validation Commands

不启用 live 或 integration gate：

```bash
go test ./api/... ./internal/ncmctl
go test -race ./internal/ncmctl
make lint
go build -o /tmp/ncmctl-spec-004 ./cmd/ncmctl
/tmp/ncmctl-spec-004 yunbei --help
/tmp/ncmctl-spec-004 task --help
go mod tidy -diff
git diff --check
```

真实账号签到、点赞、关注、收藏、红心、发布、分享、播放和 `NCMCTL_RUN_LIVE_TESTS=1` 只在用户明确授权对应副作用后运行。不得为完成本 SPEC 验证读取 `testdata/`。

---

## 10. Implementation Plan

### 10.1 Phases

| 阶段 | 内容 | 完成门禁 |
| --- | --- | --- |
| Phase 0 | 协议取证：已关闭 G3/G5/G7/G9 及 G6 的 follow/like 分支；继续补 G1/G2/G4/G8 | 每个拟实现处理器具备 URL、模式、请求字段、响应状态和稳定键证据 |
| Phase 1 | WEAPI/EAPI DTO 命名与补字段；固定响应测试 | `go test ./api/...`，无 live 请求 |
| Phase 2 | YunBei 命令骨架、校验、任务感知 query plan、status、快照关联、时间 hook、输出与结果模型 | AC-001/002/005/015 及 sign 隔离测试通过 |
| Phase 3 | sign/claim/reserve 和三轮收敛 | AC-006/007、G2/G4 已关闭 |
| Phase 4 | 四类互动均已实现动态候选、即时补偿和恢复确认；comment/collect 待 G1 身份 | AC-008/009/013，剩余 G1 关闭，race 通过 |
| Phase 5 | 小众歌曲、安全媒体下载、图文、日推播放和音乐人过滤已实现；分享待 G8 | G8 关闭，AC-010～012 及 URL/IP 测试通过 |
| Phase 6 | task 默认六项集成、共享 gate 三次重试、停机 cancel/wait | AC-016～018 和确定性并发测试通过 |
| Phase 7 | README/usage/User Skill、构建 help 对照、全量离线回归 | 第 9.7 节全部通过 |

剩余 Phase 0 门禁未关闭前，不得发布声称所有日期、所有任务均可执行的 `yunbei --all`；无法稳定关联的任务必须只展示并安全跳过。

### 10.2 Issue Mapping

| Issue | 内容 | SPEC | 依赖 |
| --- | --- | --- | --- |
| YB-1 | 协议证据与脱敏固定样本 | 4.3～4.5、11.1 | 无 |
| YB-2 | API DTO 与请求契约测试 | 3.5、4.3、9.2 | YB-1 |
| YB-3 | 命令、校验、任务感知 query plan、status、快照和时间 hook | 2、3、4.1、5.1～5.4 | YB-2 |
| YB-4 | 签到、预约和领奖 | 5.5～5.6 | YB-3 |
| YB-5 | 浏览、互动与补偿 | 5.7～5.8、5.13 | YB-3 |
| YB-6 | 安全媒体下载与媒体类任务 | 5.9～5.12、7.2 | YB-3、YB-1 |
| YB-7 | task 默认 runAll、gate 重试与停机 | 4.2、8.3、9.4 | YB-3 |
| YB-8 | 用户文档和离线发布门禁 | 5.15、7、9.7 | YB-4～YB-7 |

### 10.3 Incremental Delivery

每个阶段独立编译、离线可测。执行器只有在自身协议 gate 关闭后才进入注册表。最终首版发布门槛仍是 PRD 要求的完整任务集；不能用静默跳过已知任务冒充完整实现。未知的未来服务端任务继续只展示和安全跳过。

---

## 11. Open Questions & Risks

### 11.1 Unresolved Protocol Questions

| # | 问题 | 关闭方式 | 对应 Gate |
| --- | --- | --- | --- |
| Q1 | 剩余三个 task-list-backed 任务的稳定身份是什么，其他日期是否出现非空 Newbie/其他 bucket？ | 任务集合不同日期的全量 V3/todo 脱敏配对样本 | G1 |
| Q2 | extraLottery 的状态枚举和领取接口是否与 base 相同？ | extra 待领取与领取前后固定响应 | G2 |
| Q3 | `PREV_BOOKED_UNCLAIMED` 的 `checkToken`/`reqId` 到底在 info 响应的哪一层：2026-11-03 实测该状态已出现，`data.reqId`/`data.checkToken` 均为空，而解密正文有 817 字节，说明 `data` 内另有未映射字段。领奖后会进入哪个状态？ | 该状态的脱敏 info 响应正文（键名与层级即可，取值可全部打码），附领奖前后各一份 | G4 |
| Q4 | like-comment 与 collect-song 的稳定任务身份是什么？ | 两类任务出现当天的 V3/todo 配对样本 | G1 |
| Q5 | 站外分享到底使用 XEAPI trigger、EAPI 还是另一条链路，channel 和确认字段是什么？ | 官方客户端完整分享链 | G8 |
| Q6 | 满足「正文≥20字」的图文笔记发布后，服务端计入 `publish-note` 任务的延迟上界是多少（2026-11-03 实测同轮约 +15.8 秒计入，下界已收紧；上界仍未测定，90 秒窗口是否足够需再观察）？ | 多次授权运行的「发布→首次读到 `progressRate=1/1`」时间分布，含最慢一次 | G7 |
| Q7 | `听音乐30分钟`(3801300) 的秒数到底由什么计入：官方客户端日推播放时提交的是哪条上报路由与哪些字段（`source`/`sourceId` 是否必须为日推场景、是否需要额外的播放心跳端点）？现有 `/weapi/feedback/weblog` 载荷返回成功但不计数 | 官方客户端播放日推歌曲那一刻的全部脱敏请求/响应，附播放前后的 V3 各一份以证明 `progressRate` 由 0 增长 | G9 |

这些问题不要求产品选择，但要求协议证据。未取得用户授权时不得通过真实账号写操作或读取 `testdata/` 自行关闭。

### 11.2 Technical Risks

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 官方任务字段和 missionCode 变化 | 误分发或漏任务 | 稳定键注册表、未知任务只展示、固定样本回归 |
| 状态异步更新 | 动作已成功但本轮未确认 | 证据化等待、最多三轮、不可逆动作不重复、非零退出 |
| 补偿失败或进程强杀 | 账号残留关注/收藏/红心/点赞 | 写后立即登记、逆序有界清理、输出目标 ID；文档说明强杀边界 |
| 多进程并行 | 同一 Cookie 状态仍可能竞争 | 明确仅同进程 gate；首版不引入跨进程锁 |
| `task --runAll` 时长显著增加 | 其他 cron 触发重叠 | sign/yunbei gate 有界重试后跳过本次、不补跑；帮助说明可能超过 45 分钟 |
| 当前 API Client 跳过 TLS 证书校验 | 会话存在中间人风险 | 本功能不复制到辅助客户端；作为现有缺陷单独修复，不在此任务扩大范围 |
| 上游风控 | 奖励停发或账号限制 | 显式选择、串行、真实等待、无固定目标、服务端确认；不承诺规避风控 |

### 11.3 Confirmed Product Decisions

- `task --runAll` 默认包含完整 yunbei 作业并设置 `All=true`，保持“全部任务”向前兼容语义；不要求额外 selector 或交互确认。
- 点赞、关注、收藏和红心在各自任务确认后立即恢复；`--delete` 图文在发布任务确认后立即删除，不等待其他慢任务或领奖。
- sign/yunbei gate 首次竞争失败后每 30 秒重试，最多重试 3 次；仍被占用时明确提示并跳过当前触发，不创建补跑。
- `--task sign` 只依赖登录、签到和连签接口；余额、reserve、V3 或 todo 故障不阻断。组合执行也先完成 sign，再加载其他任务状态。

### 11.4 Assumptions

- `--all` 表示全部十二个本地已支持标识，不包含未来未知服务端任务；未知任务展示但不使 `--all` 单独失败。
- 一次性 `yunbei --all` 和调度 `task --runAll` 都不增加交互式确认提示，以保持脚本和 cron 可用；风险通过 help、升级说明和执行/启动摘要明确展示。
- sign/yunbei gate 不规定优先级；任一方在重试期间取得锁即可执行。
- `--runAll --yunbei.all` 允许；`--runAll --yunbei.task` 及显式 false 覆盖因范围冲突失败。
- 清理总预算为 60 秒；若后续取证证明单次上游超时需要更长预算，应在实现前修订该常量和测试，不从配置隐式推导。
- SPEC 保存为与 PRD 同编号的 `docs/spec/spec-004-yunbei.md`。

---

## 12. References

### 12.1 Repository Contracts

- `AGENTS.md`：开发原则、测试副作用边界、禁止未授权读取 `testdata/`。
- `internal/ncmctl/ncmctl.go`：根命令注册和 logger 生命周期。
- `internal/ncmctl/task.go`：`scheduledCommand`、选项复制、cron 注册和当前停机边界。
- `internal/ncmctl/sign.go`、`sign_test.go`：现有云贝签到/领奖顺序和行为保持测试。
- `internal/ncmctl/share.go`：`validateNoteContent`、`downloadImageToTemp`、图文发布模式。
- `internal/ncmctl/fansgroup.go`：现有播放/发布能力及 `utils.Sleep` 用法；其“先取消再红心”策略不适用于本需求的原状态保留契约。
- `api/weapi/yunbei.go`、`api/eapi/yunbei.go`：当前云贝 DTO、路径和协议模式。
- `api/eapi/daily_song_share.go`：现有 XEAPI 分享 trigger，仅作能力参考，不能代替 G8 证据。
- `api/eapi/musician.go`：`MusicianRoleGet` 的 UID/artist ID 映射。
- `pkg/utils/utils.go`：context-aware 随机等待。
- `api/api.go`：客户端超时、重试、Cookie 和协议边界。

### 12.2 Product References

- [docs/prd/prd-004-yunbei.md](../prd/prd-004-yunbei.md)：本 SPEC 的需求事实来源。
- [云贝规则](https://y.music.163.com/g/yida/886846b41d264b51b4b835acfb0eb830?fromRN=1)：奖励领取时限和违规风险。
- `3899/ncmm` 云贝任务实现：只用于发现可能流程，不作为协议、字段或固定 ID 的事实来源。
