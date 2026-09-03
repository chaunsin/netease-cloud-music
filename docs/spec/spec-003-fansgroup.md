# SPEC: ncmctl fansgroup 乐迷团任务命令

> Technical specification derived from: [docs/prd/prd-003-fansgroup.md](../prd/prd-003-fansgroup.md)  
> Generated: 2026-09-01 | Target branch: develop | Commit: 6d42fd7  
> Revised: 2026-09-01 — 双子 Agent 审查（PRD 覆盖度 + 源码契约）后修复 4 P1 / 13 P2

## 1. Summary

### 1.1 What This SPEC Covers

本 SPEC 定义 `ncmctl fansgroup` 一次性命令（含 `status` 只读位置参数）与接入现有 `ncmctl task` 调度器的实现契约。执行流程覆盖：登录检查、乐迷团详情与加入状态查询、任务列表获取与进度判定、五类任务（播放歌曲、分享歌曲、点赞乐迷笔记、发布图文笔记、今日加速任务）的分发与执行、最终进度回显，以及可选的动态删除。

实现以现有 `api.Client`、`weapi`/`eapi` wrapper、`--home`/`--config` 生命周期和 `task` 调度框架为边界。服务端 `FansGroupMissionAll` 返回的任务状态与进度是唯一事实源；客户端不落任何本地进度标记，不新建数据库表，不新增第三方依赖。任务参数（歌曲 ID、分享资源、boardId）只做结构化 JSON 解析，失败即报告该任务失败，不做正则兜底。

### 1.2 PRD Reference

- Source: [docs/prd/prd-003-fansgroup.md](../prd/prd-003-fansgroup.md)
- User Stories covered: US-001 ～ US-006
- Functional Requirements covered: FR-01 ～ FR-10
- Acceptance Criteria covered: AC-001 ～ AC-031

### 1.3 Design Decisions Summary

| # | 决策点 | 选择 | 理由 |
|---|--------|------|------|
| D1 | API 层改动范围 | 仅一处：`FansGroupFeedRecommendResp.Data` 由 `any` 类型化为结构（3.1） | 点赞任务需要读取帖子字段；已确认全仓库无该接口业务调用方，改造无兼容性风险 |
| D2 | 删除动态时序 | 发布循环全部完成后，统一等待 5~30s 再逐条删除本次 event ID | 「后置于任务确认」；备选方案（每篇发布后立即删）记入 11.3 |
| D3 | 命令退出码 | 仅当「某团至少执行过一个任务且最终结果全部为 failed」时该团失败；任一团失败 → 命令退出非零 | 空集（任务列表为空/全部 skipped/未加入）不判失败（PRD FR-09 字面边界的歧义消解）；部分成功靠服务端进度幂等在下轮 cron 自然补偿 |
| D4 | task 集成 | `--runAll` 四项 → 五项；复用 `registerScheduledCommand` 复制 `FansGroupOpts` 执行 | 不在 `task.go` 复制第二套乐迷团编排（US-005） |
| D5 | 防风控等待 | `sleepRange` 有名变量集中定义 + 单一 `sleep(ctx, r)` 辅助函数，`math/rand/v2` 均匀取值，timer+select 响应 context 取消 | 与现有 `share`/`partner` 裸 `time.Sleep` 的关键差异，也是 AC-031 的实现基础 |
| D6 | 短参数 | （已按 PRD FR-01 修订）`StringSliceVarP`/`StringVarP`/`BoolVarP` 注册 `-g`/`-t`/`-m`/`-i`/`-d`，取 flag 名称首字母 | 原决策为不设短参数并对齐 `share.go` 骨架，偏差已关闭：用户确认恢复 PRD 字面要求，详见 4.1 与 11.1 Q1 结论 |
| D7 | 未知任务类型 | 输出原始标题并 `skipped`，不算失败 | FR-03：官方新增任务类型不应导致命令不可用 |
| D8 | 解析策略 | 只走结构化 JSON 解析（`missionButtonParams`），失败即报告该任务失败 | PRD 产品方案明确禁止正则兜底 |

## 2. Architecture

### 2.1 System Context

命令注册于 `internal/ncmctl/ncmctl.go` 的根命令列表（`Root.Add`），与 `share`、`scrobble`、`partner`、`sign` 平级。运行依赖由根命令统一初始化：`--home`/`--config` 决定 `root.Cfg`（网络超时、Cookie 传输策略），`api.NewClient` 创建会话客户端。

```
ncmctl（root）
├── fansgroup [status] [--group-id --title --message --image --delete]   ← 本 SPEC
├── share（命令骨架与图片/发布/删除模式的参照）
├── scrobble / partner / sign
└── task（--fansgroup 调度接入，见第 7 章）
```

### 2.2 Component Design

命令结构体对齐 `share.go` 骨架（`NewDailySongShare`/`Command`/`addFlags`/`validate`/`execute` 五件套）：

```go
type FansGroup struct {
	root   *Root
	cmd    *cobra.Command
	opts   FansGroupOpts
	l      *log.Logger
	uid    int64 // GetUserInfo 校验后的当前用户 ID
	status bool  // 位置参数解析结果
}
```

方法职责（SRP，一层只做一件事）：

| 方法 | 职责 |
|------|------|
| `NewFansGroup(root, l)` | 构造命令、注册 flags、绑定 RunE；`Args` 校验仅允许零个或一个 `status` 字面量 |
| `Command()` | 返回 `*cobra.Command`（供 `task` 的 `scheduledCommand` 接口复用） |
| `addFlags()` | flag 注册（默认值直接引用常量，无运行时回退分支） |
| `validate()` | 参数校验（与 `status` 互斥的 flag 组合在此快速失败） |
| `execute(ctx)` | 默认任务流程：登录检查 → 逐团编排 |
| `executeStatus(ctx)` | 只读流程：登录检查 → 逐团读取输出；不等待、不执行、不上传、不发布、不点赞、不红心、不删除 |
| `runGroup(ctx, ...)` | 单团编排：详情 → 加入状态 → 任务列表 → 分发循环 → 加速任务 → 聚合 |
| `dispatchMission(...)` | 按标题关键词分发到五类任务执行器，返回单任务结果 |
| `runPlayMission` / `runShareMission` / `runLikeMission` / `runNoteMission` / `runSpeedUpMission` | 五类任务执行器，各自独立可测 |
| `aggregate(...)` | 团级结果聚合（D3 规则）与最终进度回显 |

### 2.3 Module Interactions

- 登录校验复用 `share.go load()` 模式：`api.NewClient` → `weapi.GetUserInfo`，`Code != 200` 或 `Profile == nil` 即 `need login`；`uid` 供点赞过滤（剔除本人帖子）。
- 图片准备复用 `share.go prepareImage` 模式：本地路径校验后直用；否则 `http.NewRequestWithContext` 下载到 `os.CreateTemp` 临时文件，`io.LimitReader` 限流，返回 cleanup 闭包。
- 本地图片校验（`os.Lstat` + 非符号链接 + 常规文件 + 非空）从 `share.go validateFlags` 提取为包级共用函数 `validateLocalImage(path string) error`，`share` 与 `fansgroup` 共用，消除复制粘贴。
- API client 关闭统一 `closeAPIClient(ctx, cli, c.l)`（`utils.go`）。
- task 侧复用 `registerScheduledCommand` 与 `scheduledCommand` 接口（`Command()` + `validate()`），注册时由 `describeSchedule` 自动输出下次执行时间。

### 2.4 File Structure

| 文件 | 变更 |
|------|------|
| `internal/ncmctl/fansgroup.go` | 新增：命令、五类任务执行器、sleep 体系、解析结构 |
| `internal/ncmctl/share.go` | 修改：图片校验提取为 `validateLocalImage`，`validateFlags` 改为调用它 |
| `internal/ncmctl/task.go` | 修改：`TaskOpts` 内嵌 `FansGroupOpts`、`--fansgroup*` flags、`taskSelection`/`validate`/`execute` 增量 |
| `internal/ncmctl/ncmctl.go` | 修改：根命令注册 `NewFansGroup(root, l).Command()` |
| `api/eapi/fansgroup.go` | 修改：`FansGroupFeedRecommendResp.Data` 类型化（3.1） |
| `docs/usage.md` | 修改：新增 fansgroup 命令说明与示例 |
| `internal/ncmctl/fansgroup_test.go` | 新增：测试（第 9 章） |

### 2.5 Code Quality & Style Contract

本章为强制契约，映射 `AGENTS.md` 核心原则，实施与 review 逐条对照：

| AGENTS.md 原则 | 本 SPEC 落实方式 |
|---------------|------------------|
| YAGNI / 简单至上 | 无本地进度存储、无新依赖、无 `--dry-run`（`status` 即只读预览）；flag 默认值直接引用常量，删除「空时回退」分支 |
| 高内聚低耦合 | 乐迷团编排集中在 `fansgroup.go` 单文件；`task.go` 只复制 opts 注册，不复制编排；API 层不加业务逻辑 |
| 单一职责 | `dispatchMission` 只分发、`runXxxMission` 只执行、`aggregate` 只聚合；不写万能大函数 |
| 复用优先 | `closeAPIClient`、`validateLocalImage`、`prepareImage` 模式、`registerScheduledCommand`、`describeSchedule`、既有 API wrapper 一律复用，禁止重复实现 |
| 标准库优先 | `math/rand/v2`、`time.Timer`+`select`、`os.CreateTemp`、`io.LimitReader`；不新增任何第三方依赖 |
| 快速失败 | 参数校验前置（`status` 与写 flag 互斥在发起任何请求前失败）；传输错误与业务 `Code != 200` 分开判定；禁止吞错，业务失败不得伪装成成功 |
| 删除胜于添加 | 不引入正则兜底、不引入文案库/图片池配置模型、不引入多账号队列 |
| 为维护者编程 | sleep 区间集中命名常量化（调区间改一处）；未知任务类型输出服务端原文便于排查；每个执行器独立可测 |

错误包装规范（全命令统一）：

- 传输/执行错误：`fmt.Errorf("operation: %w", err)`，如 `fmt.Errorf("fans group detail: %w", err)`。
- 业务失败：`fmt.Errorf("operation: code=%d message=%s", resp.Code, resp.Message)`，操作名可定位到具体接口。
- 特例：`eapi.SongLikeResp` 仅有 `Code` 字段无 `Message`（`api/eapi/song.go:297-299`），降级为 `fmt.Errorf("song like: code=%d", resp.Code)`。
- 临时文件清理走 `defer cleanup()`，成功与失败路径一致。

## 3. Data Model

### 3.1 Schema Changes（API 层）

唯一一处 API 层改动：`FansGroupFeedRecommendResp.Data` 由 `any` 类型化。已确认该接口在仓库内无业务调用方（仅定义与测试），改造零兼容性风险。

```go
// FansGroupFeedRecommendResp 获取乐迷团推荐Feed响应.
// Data 由 any 类型化：点赞任务需要读取帖子 threadId / 点赞状态 / 发布者。
// 注意：posts 数组在 data 下的挂载层级来自 PRD 对参考实现的转述，Phase 1 验证（11.1 Q2）。
type FansGroupFeedRecommendResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Posts []FansGroupFeedPost `json:"posts"`
	} `json:"data"`
}

type FansGroupFeedPost struct {
	ThreadID string `json:"threadId"`
	Info     struct {
		Liked bool `json:"liked"`
	} `json:"info"`
	User struct {
		UserID int64 `json:"userId"`
	} `json:"user"`
}
```

### 3.2 Entity Definitions（命令层）

命令选项：

```go
type FansGroupOpts struct {
	GroupID []string // 乐迷团 ID 列表；默认 defaultFansGroupID
	Title   string   // 笔记标题覆盖
	Message string   // 笔记正文覆盖
	Image   string   // 本地图片路径；空时下载乐迷团头像
	Delete  bool     // 任务循环完成后延时删除本次动态
}
```

常量集中定义（避免魔数散落）：

```go
const (
	defaultFansGroupID       = "1872529203038486609" // 内置默认乐迷团（PRD 待确认项，帮助文本明示）
	audioFetchLimit    int64 = 512 << 10             // 单次音频拉取上限 512 KiB（PRD FR-04）
	fansAvatarLimit    int64 = 20 << 20              // 头像下载上限 20 MiB，对齐 share 命令封面约定
)
```

防风控等待区间（闭区间 `[Min, Max]`，`math/rand/v2` 均匀取值）：

```go
// sleepRange 表示一处防风控等待的随机区间（闭区间 [Min, Max]）。
type sleepRange struct {
	Min, Max time.Duration
}

// sleep 在区间内均匀取值等待，响应 ctx 取消：取消时立即返回 ctx.Err()。
// timer+select 实现，替代现有命令的裸 time.Sleep（AC-031）。
func sleep(ctx context.Context, r sleepRange) error {
	d := r.Min + time.Duration(rand.Int64N(int64(r.Max-r.Min)+1))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
```

```go
var (
	sleepGroupGap     = sleepRange{3 * time.Second, 10 * time.Second} // 乐迷团之间
	sleepMissionGap   = sleepRange{2 * time.Second, 5 * time.Second}  // 同团任务之间
	sleepPlayIter     = sleepRange{2 * time.Second, 5 * time.Second}  // 播放迭代之间
	sleepShareIter    = sleepRange{2 * time.Second, 5 * time.Second}  // 分享迭代之间
	sleepLikeIter     = sleepRange{1 * time.Second, 3 * time.Second}  // 点赞迭代之间
	sleepNoteIter     = sleepRange{2 * time.Second, 5 * time.Second}  // 发布迭代之间
	sleepBeforeDelete = sleepRange{5 * time.Second, 30 * time.Second} // 发布完成→删除前
	sleepBeforeLike   = sleepRange{1 * time.Second, 3 * time.Second}  // 归一化→红心
	sleepBeforeUnlike = sleepRange{3 * time.Second, 10 * time.Second} // 红心→取消恢复
	sleepSpeedUpIter  = sleepRange{2 * time.Second, 5 * time.Second}  // 加速任务迭代之间
)
```

任务参数结构化解析（禁止正则兜底）：

```go
// flexString 接收既可能是字符串也可能是数字的 JSON 值: 服务端在同一批参数里混用两种类型
// (实测分享任务 progressParams 的 resourceId 为字符串而 resourceType 为数字, songId 亦为数字)。
// null 视为空值; 对象与数组等非标量直接报错, 避免静默吞掉类型错误。
type flexString string

// missionButtonParams 是任务 button.url / iconUi.targetUrl 携带的参数 JSON 的结构化表示。
// 字段均为可选：不同任务类型只填充其中的子集，解析后按非空原则取值。
// ID 与类型字段统一用 flexString，兼容服务端字符串/数字混发。
type missionButtonParams struct {
	SongID   flexString   `json:"songId"`
	SongIDs  []flexString `json:"songIds"`
	TrackID  flexString   `json:"trackId"`
	TrackIDs []flexString `json:"trackIds"`

	ActionCustomParams struct {
		ProgressParams struct {
			ResourceID   flexString `json:"resourceId"`
			ResourceType flexString `json:"resourceType"`
		} `json:"progressParams"`
	} `json:"actionCustomParams"`
}
```

> **实测修正（2026-09-02）**：初版把这些字段声明为 `string`，线上分享任务返回
> `{"progressParams":{"resourceId":"3357361025","resourceType":4,...},"songId":3357361025}`
> 时 `json.Unmarshal` 直接报 `cannot unmarshal number into Go struct field ... of type string`，
> 整个分享任务判 failed；播放任务的 `songId` 同样可能是数字，会连带失败。
> 因此统一改为 `flexString`：**不要把这里的字段类型改回 `string`**。

单团执行期状态（不持久化，随命令结束丢弃）：

```go
// fansGroupRuntime 保存单个乐迷团执行周期内的中间状态。
type fansGroupRuntime struct {
	groupID   string   // 当前乐迷团 ID
	boardID   string   // 详情返回的 boardId，发布笔记 activityInfoList 用
	groupName string   // 乐迷团名，activityInfoList.name 用
	avatarURL string   // 详情返回的头像 URL，未指定 --image 时用
	songIDs   []string // normal 任务解析到的歌曲 ID，加速任务回退用
	eventIDs  []int64  // 本次执行链内发布成功的动态 ID，--delete 用
	results   []taskResult
}

// taskResult 单个任务的最终结果，团级聚合输入。
type taskResult struct {
	Title  string
	Status taskStatus // done / partial / skipped / failed
}
```

### 3.3 Relationships

任务列表与执行结果的关系：`FansGroupMissionAll` 返回的 `Status`/`CurrentProgress`/`AllProgress` 是唯一事实源；`taskResult` 只是本轮客户端执行结果，用于输出与退出码聚合，不反写任何进度。

### 3.4 Migration Plan

无数据库迁移、无配置文件变更。`FansGroupFeedRecommendResp.Data` 类型化属源码级重构，调用方为零，一次提交完成。

## 4. API Design

### 4.1 Command Surface

一次性命令 flags（短参数按 PRD FR-01 取 flag 名称首字母，长短参数语义完全等价）：

| Flag | 短参数 | 默认值 | 约束/行为 |
|------|--------|--------|-----------|
| `--group-id` | `-g` | `defaultFansGroupID`（内置 `1872529203038486609`） | `StringSliceVarP`，天然支持逗号分隔与重复传参；每个值必须为非空纯数字字符串 |
| `--title` | `-t` | 空（内置默认标题） | `TrimSpace` 后非空 |
| `--message` | `-m` | 空（内置默认正文，含随机元素） | `TrimSpace` 后至少 10 个 Unicode 字符 |
| `--image` | `-i` | 空 | 存在的非符号链接、非空常规文件（`validateLocalImage`）；空时下载乐迷团头像 |
| `--delete` | `-d` | `false` | 发布成功且任务循环完成后延时删除本次动态 |

```go
func (c *FansGroup) addFlags() {
	f := c.cmd.Flags()
	// 短参数取 flag 名称首字母 (PRD-003 短参数约定), 与 share.go 的 -i/-t/-m 语义一致;
	// -d 未被本命令占用, 无需像 share.go 那样让位给 --draw 而改用大写 -D。
	f.StringSliceVarP(&c.opts.GroupID, "group-id", "g", []string{defaultFansGroupID},
		"fans group IDs (digits only); comma-separated or repeated")
	f.StringVarP(&c.opts.Title, "title", "t", "", "note title override")
	f.StringVarP(&c.opts.Message, "message", "m", "", "note message override; at least 10 Unicode characters")
	f.StringVarP(&c.opts.Image, "image", "i", "", "local image file; empty downloads the fans group avatar")
	f.BoolVarP(&c.opts.Delete, "delete", "d", false, "delete notes published by this run after the mission loop")
}
```

**PRD FR-01 短参数偏差说明（D6，已关闭）**：初版按命令层主流惯例（参照 `share.go` 无短参数的 flags）设计为无短参数。现已确认遵循 PRD 字面要求，注册改为 `VarP` 系列并补齐 `-g`/`-t`/`-m`/`-i`/`-d`：`VarP` 用法在命令层另有 `task --location/-l`、`scrobble --num/-n` 等先例，短参数与根命令持久参数 `-c`（`--config`）及本命令其他短参数均无冲突。`share.go` 的 `--delete` 用大写 `-D` 是因其 `-d` 已分配给 `--draw`，本命令无 `--draw`，故取小写 `-d`。

位置参数：仅允许零个或一个 `status` 字面量（对齐 `share.go` 的 `Args` 闭包模式）；`status` 与 `--delete`/`--title`/`--message`/`--image` 组合时在 `validate` 快速失败（AC-004）；`--group-id` 与 `status` 兼容（status 支持多团查询，AC-005）。

### 4.2 Invoked NetEase Endpoints

| 接口 | 层 | 用途 | 写操作 | status 模式调用 |
|------|----|------|--------|----------------|
| `GetUserInfo` | weapi | 登录校验、取 uid/昵称 | 否 | 是 |
| `FansGroupDetailGet` | eapi | 团详情（boardId/名称/头像） | 否 | 是 |
| `FansGroupUserGroupDetailGet` | eapi | 加入状态/等级/头衔 | 否 | 是 |
| `FansGroupMissionAll` | eapi | 任务列表与进度（事实源） | 否 | 是 |
| `FansGroupFeedRecommend` | eapi | 点赞候选帖子 | 否 | 否 |
| `FansGroupMissionForwardProgress` | eapi | 分享进度上报 | 是 | 否 |
| `ResourceLike` | eapi | 点赞帖子 | 是 | 否 |
| `EventUploadImage` | eapi | 上传笔记图片 | 是 | 否 |
| `EventPublish` | eapi | 发布笔记 | 是 | 否 |
| `EventDelete` | eapi | 删除本次动态 | 是 | 否 |
| `SongLike` | eapi | 红心/取消红心 | 是 | 否 |
| `SongDetail` | weapi | 歌曲名/时长/专辑（可选步骤） | 否 | 否 |
| `SongPlayerV1` | weapi | 播放地址 | 否 | 否 |
| `WebLog` | weapi | startplay / play 上报 | 是（行为日志） | 否 |

### 4.3 Request/Response Schemas（关键载荷）

播放链路 `WebLog` 载荷（对齐 `api/weapi/feedback.go:67-74` 样本注释；`Logs` 为 `[]map[string]any`，wrapper 内部序列化）：

```json
[{"action":"startplay","json":{"id":<songId>,"type":"song","content":"id=<songId>","mainsite":"1"}}]
```

```json
[{"action":"play","json":{"type":"song","wifi":0,"download":0,"id":<songId>,"time":<3~5随机秒>,"end":"interrupt","source":<source>,"sourceId":"<sourceId>","mainsite":"1","content":"id=<songId>"}}]
```

注意值类型差异：样本中 `id` 为数字、`sourceId`/`content` 为字符串、`source` 两种取值都出现过（`list`/`toplist`），Phase 1 验证（11.1 Q4）。

发布笔记 `EventPublishReq` 关键字段：`Type: "noresource"`、`Title`/`Msg`、`Pics`（`EventUploadImage` 返回值）、`ActivityInfoList`（JSON 字符串）。`activityInfoList` 构造（`id` 来自详情接口 `boardId`，不硬编码；格式对齐 `api/eapi/event.go:48` 注释）：

```json
[{"id":"<boardId>","type":3,"subType":11,"name":"<groupName>","selected":true,"canChange":true}]
```

点赞 `ResourceLikeReq.AppLogExt` 携带乐迷团归属标记（`addRefer`/`multiRefer` 指向当前乐迷团 ID，具体 JSON 结构待 Phase 1 验证，11.1 Q3）。

加速任务 `SongLikeReq`：`TrackId` 歌曲 ID、`Like` 为 `"true"`/`"false"` 字符串、`Time` 参考默认 `"3"`、`CheckToken` 留空（两个参数语义待验证，11.1 Q5）。`SongLikeResp` 仅 `Code` 无 `Message`。

播放地址 `SongPlayerV1Req`：`Ids types.IntsString`（单曲传 `[]int64{songID}`，不传 `_uid` 后缀）、`Level: types.LevelStandard`（标准品质 128000，`api/types/quality.go:12`）。

### 4.4 Error Responses

所有业务错误按 2.5 错误包装规范输出「操作名 + code + message」；`SongLikeResp` 无 `Message` 时降级输出 code。`SongPlayerV1Resp` 存在双层 Code 语义：外层 `Code` 为接口业务码，`Data[i].Code` 为单曲状态（`200` 正常 / `404` 下架变灰），两者分别判定（5.1.3）。

### 4.5 Breaking Changes

`FansGroupFeedRecommendResp.Data` 类型化（3.1）。仓库内无业务调用方，仅可能影响直接断言 `any` 的测试（当前不存在），属安全重构。

## 5. Business Logic

### 5.1 Core Algorithms

#### 5.1.1 登录与团级编排

```
1. api.NewClient → weapi.GetUserInfo：
   Code != 200 或 Profile == nil 或 UserId <= 0 → 立即终止（"need login"，AC-002）
   记录 uid、输出昵称与 UID（不输出 Cookie）
2. 若为 status 模式：逐团执行读取输出后结束（5.1.2 读取部分），不进入任务流程
3. 解析 --group-id（flag 默认值即 defaultFansGroupID，无需运行时回退），逐团串行执行：
   for i, gid := range groupIDs:
       if i > 0: sleep(ctx, sleepGroupGap)   # 团间 3~10s
       err = runGroup(ctx, gid)
       err != nil → 记录该团失败，继续下一团（AC-008 单团失败隔离）
   每团输出分组边界（组头：团名 + groupID）
4. 汇总退出码：任一团「至少执行过一个任务且最终结果全部 failed」→ 返回 error；否则成功（6.1）
```

`runGroup`（单团编排）：

```
detail = FansGroupDetailGet(gid)          # Code != 200 → 团失败（详情是后续一切的前置）
member = FansGroupUserGroupDetailGet(gid) # Code != 200 → 团失败
member.Joined == false → 输出「未加入，跳过」，不调用任务接口（AC-009），团成功
missions = FansGroupMissionAll(gid)       # Code != 200 → 团失败
打印全部任务标题/状态/进度（含原始状态值 INIT/PROCESSING/COMPLETED，5.4 可用性）
runtime = {groupID, boardID, groupName, avatarURL 从 detail 填充}
分发循环（5.1.2）→ 加速任务（5.1.7）→ 可选删除（5.1.8）→ 团级聚合（6.1）
```

#### 5.1.2 任务列表获取与分发

```
for i, mission := range missions.Data.Normal.Data:
    if 已完成(mission): 输出「已完成，跳过」，skipped（AC-011）
    if i > 0（且上一任务非首个待执行）: sleep(ctx, sleepMissionGap)  # 同团任务间 2~5s
    result = dispatchMission(ctx, mission)
    runtime.results = append(runtime.results, result)

已完成判定（FR-02/FR-03，服务端为准，客户端不推算自然日）：
  Status == "COMPLETED" || (AllProgress > 0 && CurrentProgress >= AllProgress)
剩余次数 = AllProgress - CurrentProgress；结果 <= 0 时按 1 次处理
```

任务分发规则（只依据服务端标题与进度，不硬编码任务数量或顺序）：

| 标题关键词 | 任务 | 实现 | 结果状态 |
|-----------|------|------|---------|
| 含「播放」 | 播放歌曲 | 5.1.3 | done/partial/skipped/failed |
| 含「分享」 | 分享歌曲 | 5.1.4 | done/partial/skipped/failed |
| 含「点赞」 | 点赞乐迷笔记 | 5.1.5（前置：Feed） | done/partial/skipped/failed |
| 含「笔记」或「发布」 | 发布图文笔记 | 5.1.6 | done/partial/failed |
| 均不命中 | 未知类型 | 输出原始标题，skipped（非失败，FR-03/US-006 安全跳过） | skipped |

同团任务间等待时机：每个任务完成或跳过后、执行下一任务前（即分发循环内 `i > 0` 时先等待）。

#### 5.1.3 播放歌曲任务

```
params = 解析 mission.Button.Url（必要时 IconUi.TargetUrl）→ missionButtonParams
songIDs = 非空合并(SongIDs, []{SongID}, TrackIDs, []{TrackID})
songIDs 为空 → 该任务 failed（AC-013），输出原因，继续下一任务
解析得到的 songIDs 累积进 runtime.songIDs（供加速任务回退）

for i := 0; i < remaining; i++:
    song = 随机选择一首（songIDs）
    if i > 0: sleep(ctx, sleepPlayIter)   # 迭代间 2~5s
    detail = SongDetail(song)             # 可选步骤：歌名/时长/专辑 ID；失败仅记录，不中断
    WebLog(startplay)                      # 载荷见 4.3
    player = SongPlayerV1(song, LevelStandard)
    if player.Code != 200:                          本轮迭代失败，continue
    if len(player.Data) == 0:                       本轮迭代失败，continue  # 空切片防护
    if player.Data[0].Code != 200 || Url == "":     本轮迭代失败，continue  # 404=下架/无地址
    Range 请求拉取 ≤ audioFetchLimit（512 KiB）音频后关闭连接，遵循 ctx 取消
    WebLog(play)                           # time 为 3~5 秒随机且不超过歌曲时长；end=interrupt
                                           # source/sourceId 取专辑信息，无专辑时 source=toplist
聚合：全部迭代成功 done；部分成功 partial；全部迭代失败 failed
播放链路内部步骤（startplay→拉取→play）之间不插入 sleep；上报 time 与实际拉取解耦（FR-04）
```

#### 5.1.4 分享歌曲任务

```
params = 解析 mission.Button.Url → missionButtonParams
resourceID = params.ActionCustomParams.ProgressParams.ResourceID
resourceID 为空 → 该任务 failed，不猜测资源 ID
resourceType = ProgressParams.ResourceType，缺省按 "4"（歌曲）

for i := 0; i < remaining; i++:
    if i > 0: sleep(ctx, sleepShareIter)   # 迭代间 2~5s
    resp = FansGroupMissionForwardProgress(ctx, &FansGroupMissionForwardProgressReq{
        ResourceId:   resourceID,
        ResourceType: resourceType,
    })   # Action/FansGroupId 由 wrapper 默认（share / null）
    传输错误或 Code != 200 → 该次上报失败，带 code/message 记录后继续
聚合：全部成功 done；部分成功 partial；全部失败 failed
```

#### 5.1.5 点赞乐迷笔记任务（前置：Feed）

```
feed = FansGroupFeedRecommend(ctx, &FansGroupFeedRecommendReq{
    FansGroupId: runtime.groupID,   # 必须显式传入：wrapper 对该字段无默认值（fansgroup.go:152），
                                    # 空串会拼进 URL，帖子不按团过滤、点赞不被任务计数
    Size:        strconv.Itoa(remaining + 5),   # 覆盖剩余次数并留余量
    Cursor:      "0",
    ArtistSelf:  "0",
})
feed.Code != 200 → 该任务 failed
posts = 过滤: ThreadID 非空 && Info.Liked == false && User.UserID != 当前 uid
posts 为空 → 输出「无可点赞帖子」，skipped（非失败，AC-016）
n = min(remaining, len(posts))
for i := 0; i < n; i++:
    if i > 0: sleep(ctx, sleepLikeIter)    # 迭代间 1~3s
    resp = ResourceLike(ctx, &ResourceLikeReq{
        ThreadId:  posts[i].ThreadID,
        AppLogExt: appLogExt(runtime.groupID),   # 结构待 Phase 1 验证（11.1 Q3）
    })
    传输错误或 Code != 200 → 记录错误后继续下一篇
聚合（FR-06「不超过可用帖子数」是合法完成场景）：
  - 全部点赞成功 → done；可用帖子少于剩余次数时附「帖子不足」说明（仍为 done，非 partial）
  - 存在迭代失败但有成功 → partial；全部迭代失败 → failed
```

#### 5.1.6 发布图文笔记任务

```
activityInfoList = [{"id": runtime.boardID, "type": 3, "subType": 11,
                     "name": runtime.groupName, "selected": true, "canChange": true}]
image, cleanup = prepareImage:
    --image 非空 → validateLocalImage 校验后直用（无 cleanup）
    否则下载 runtime.avatarURL 到临时文件（≤ fansAvatarLimit 20 MiB，遵循 ctx），defer cleanup
pics = EventUploadImage(ctx, image)   # 失败 → 该任务 failed，不发布无图笔记（cleanup 仍执行）
title, message = 文案生成：
    --title/--message 提供时 TrimSpace 后原样使用（校验在 validate 完成）
    默认标题内置模板；默认正文内置模板 + 随机元素（如编号），避免连续多日内容相同
    正文长度 ≥ 10 Unicode 字符（utf8.RuneCountInString 校验）

for i := 0; i < remaining; i++:
    if i > 0: sleep(ctx, sleepNoteIter)    # 迭代间 2~5s
    resp = EventPublish(ctx, &EventPublishReq{
        Type:             "noresource",
        Title:            title,
        Msg:              message,
        Pics:             pics,
        ActivityInfoList: activityInfoList,
    })
    resp.Code != 200 → 该次发布失败，记录后继续
    resp.Id > 0 → 输出 event ID；runtime.eventIDs 追加（仅供 --delete 使用）
聚合：全部成功 done；部分成功 partial；全部失败 failed
发布失败时不删除、不重发（AC-019）
```

#### 5.1.7 今日加速任务

```
mission = missions.Data.Originality.Data
mission.Title 为空或已完成 → 跳过（无输出失败）
songIDs = 解析 Button.Url + LogInfo + MissionDetail 中 JSON 的歌曲 ID
为空时回退 runtime.songIDs（normal 播放任务累积）
仍为空 → 输出跳过，不使用硬编码歌曲（AC-022）
副标题含「收藏」或「红心」→ 红心流程；无法识别 → 同样按红心流程处理并输出原始副标题（FR-08）

红心流程（每轮迭代）：
    song = 随机选择一首（songIDs）
    if iter > 0: sleep(ctx, sleepSpeedUpIter)   # 迭代间 2~5s
    SongLike(like=false)   # 归一化为未红心（已红心时取消，确保后续动作被计数）
    sleep(ctx, sleepBeforeLike)     # 1~3s
    SongLike(like=true)    # 完成任务计数
    sleep(ctx, sleepBeforeUnlike)   # 3~10s
    SongLike(like=false)   # 恢复原状，不在账号遗留红心（AC-021）
    任一步传输错误或 Code != 200 → 该轮失败，记录后继续下一轮
聚合：全部轮次成功 done；部分成功 partial；全部失败 failed
```

#### 5.1.8 最终进度回显与可选删除

```
final = FansGroupMissionAll(gid)   # 尽力而为：失败仅记录，不作为成功判定（进度可能异步更新）
输出最终各任务进度与剩余积分（final.Code == 200 时）

if opts.Delete && len(runtime.eventIDs) > 0:
    for _, eventID := range runtime.eventIDs:
        sleep(ctx, sleepBeforeDelete)   # 每条删除前 5~30s（发布循环已全部完成，D2 时序）
        resp = EventDelete(ctx, &EventDeleteReq{Id: eventID})
        传输错误或 Code != 200 → 输出 event ID 与原因，按部分成功处理，不重发
删除只针对本次执行链内发布成功的动态；未发布（任务已完成跳过）时不删除
```

### 5.2 Validation Rules

| 校验 | 时机 | 失败行为 |
|------|------|---------|
| 位置参数仅零个或一个 `status` 字面量 | `Args` 闭包 | 快速失败（对齐 share.go 模式） |
| `status` 与 `--delete/--title/--message/--image` 互斥 | `validate` | 快速失败，不发起任何请求（AC-004） |
| `--group-id` 每个值非空纯数字 | `validate` | 快速失败，提示必须为数字（AC-006） |
| `--title` TrimSpace 后非空 | `validate` | 快速失败 |
| `--message` TrimSpace 后 ≥10 Unicode 字符 | `validate` | 快速失败 |
| `--image` 非符号链接、非空常规文件 | `validateLocalImage` | 快速失败 |
| 登录有效（GetUserInfo） | `execute` 前置 | 终止命令，不调用任务接口（AC-002） |

### 5.3 State Machine

任务结果状态（`taskStatus`）转移：

```
待执行 ──(服务端已完成)──────────────→ skipped
      ──(全部迭代成功)──────────────→ done
      ──(部分迭代成功)──────────────→ partial
      ──(所有迭代失败/解析失败)─────→ failed
      ──(未知类型/无可用资源)───────→ skipped（输出服务端原文）

点赞特例：可用帖子 < 剩余次数但全部成功 → done（附说明），非 partial
发布特例：done/partial + 删除失败 → 保持原状态，逐行输出删除失败详情与 event ID
```

团级聚合（D3）：`len(results) == 0` → 团成功；全部结果均为 `failed` → 团失败；其余（done/partial/skipped 混合）→ 团成功。命令级：任一团失败 → 退出非零。

### 5.4 Edge Cases

| 场景 | 行为 |
|------|------|
| 任务列表为空 / 全部已完成 / 全部 skipped / 团未加入 | 团成功（不判失败），输出相应提示 |
| `SongPlayerV1Resp.Data` 空切片 | 迭代失败防护，不 panic（`weapi/song.go:297-307` 切片语义） |
| `Data[0].Code == 404`（歌曲下架变灰） | 该轮迭代失败并输出歌曲状态，与外层业务 Code 分别判定 |
| `button.url` JSON 解析失败 | 该任务 failed，输出解析错误原文，不用正则兜底 |
| Feed 帖子全部已点赞/本人帖子/空 threadId | 全部剔除；剔除后为空 → skipped（AC-016） |
| 头像下载失败 / 超过 20 MiB | 该发布任务 failed，临时文件清理仍执行 |
| Cookie 失效（任务中途） | 后续接口传输/业务错误按任务失败记录，命令按聚合规则退出 |
| context 取消（Ctrl+C） | `sleep` 立即返回；音频/图片下载遵循 ctx；命令尽快退出（AC-031） |
| 进度异步未更新 | 最终进度输出尽力而为，不报错、不作为成功判定 |
| 服务端返回未知任务类型 | 输出原始标题，skipped，不中断其他任务（US-006） |
| 多条动态删除（remaining > 1） | 每条删除前独立等待 5~30s，逐条输出结果 |

## 6. Error Handling

### 6.1 Error Taxonomy

| 类别 | 定义 | 处理 | 输出 |
|------|------|------|------|
| 参数校验失败 | 5.2 任一规则不满足 | 快速失败，命令退出非零，不发起请求 | 校验错误信息 |
| 登录失效 | `GetUserInfo` 失败/无有效会话 | 立即终止命令，退出非零 | `need login`（AC-002） |
| 团前置失败 | 详情/加入状态/任务列表接口失败 | 该团记失败，继续下一团（AC-008） | 团标识 + 操作名 + code/message |
| 任务失败 | 该任务所有迭代均失败（或解析失败） | 记录后继续下一任务 | 任务标题 + 原因 |
| 部分成功 | 部分迭代成功部分失败 | 逐行输出，不改退出码 | 成功/失败计数 |
| 团失败 | 该团至少执行过一个任务且最终结果全部为 failed（skipped 不计） | 任一团失败 → 命令退出非零 | 团标识、各任务失败原因 |
| 删除失败 | `EventDelete` 失败 | 输出 event ID，按部分成功，不重发 | event ID + 原因（AC-019） |
| `SongLike` 业务失败 | `Code != 200`（响应无 Message 字段） | 该轮迭代失败，继续 | `song like: code=%d` |

退出码语义：成功（含全部跳过、部分成功）退出 0；仅当至少一团满足「至少执行过一个任务且全部 failed」时返回错误（PRD FR-09 字面边界的空集歧义消解，D3）。部分成功的迭代失败与删除失败逐行输出但不改退出码，下一轮 cron 依赖服务端进度幂等自然补偿。

### 6.2 Retry Strategy

同一执行周期内不自动重试：单次迭代失败记录后继续下一次迭代；任务失败记录后继续下一任务；团失败记录后继续下一团。task 场景下单次调度失败只记录错误并等待下一次 cron（AC-029），不进入紧密重试。补偿依赖服务端进度幂等：已完成部分在下一轮自动跳过。

### 6.3 Failure Modes

- 不吞错：所有错误经 2.5 规范包装后输出，含操作名与 code/message。
- 不伪装成功：业务 `Code != 200` 一律计为该操作失败，即使 HTTP 200。
- 副作用可恢复：红心任务结束恢复原状；删除只针对本次链内 event ID。
- 快速失败边界：仅清理路径（`closeAPIClient`、临时文件 cleanup）允许记录后继续。

## 7. Task Integration（task 命令接入）

`TaskOpts` 增量（内嵌 + 选择器，对齐现有四项任务模式）：

```go
type TaskOpts struct {
	// ... 现有 PartnerOpts / ScrobbleOpts / SignInOpts / ShareOpts 保持不变 ...
	FansGroupOpts

	FansGroup            bool
	FansGroupOptsCrontab string
}
```

task 侧 flags（`PersistentFlags`，对齐现有惯例不设短参数）：

| Flag | 默认值 | 说明 |
|------|--------|------|
| `--fansgroup` | `false` | 注册乐迷团任务 |
| `--fansgroup.cron` | `30 10 * * *` | 五段式 cron（`[Assumption]`，用 `--location` 时区；PRD FR-10） |
| `--fansgroup.group-id` | `defaultFansGroupID` | 同一次性命令格式约束 |
| `--fansgroup.title` | 空 | 空时使用内置默认标题 |
| `--fansgroup.message` | 空 | 空时使用内置默认正文（含随机元素） |
| `--fansgroup.image` | 空 | 空时下载乐迷团头像 |
| `--fansgroup.delete` | `false` | 发布成功后删除本次动态 |

集成点（全部为增量修改，不重构现有四项）：

1. `taskSelection` 增加 `FansGroup` 字段；`--runAll` 返回五项全选（sign、partner、scrobble、share、fansgroup）；空选择错误提示追加 `--fansgroup`（AC-027）。
2. `Task.validate` 增加 fansgroup 闭包：`FansGroupOptsCrontab` 非空 + `cron.ParseStandard` 合法 + `group-id` 每值数字格式（AC-028，启动 cron 前失败）。
3. `Task.execute` 增加 fansGroup 闭包：`command := NewFansGroup(c.root, c.l); command.opts = c.opts.FansGroupOpts; c.registerScheduledCommand(ctx, job, "fansgroup", c.opts.FansGroupOptsCrontab, "[fansgroup] crontab error", command)`。
4. `Task.execute` 通过现有 `registerScheduledCommand` 创建 `NewFansGroup` 实例、复制 `c.opts.FansGroupOpts` 后注册；不得把乐迷团 API 编排复制进 `task.go`（US-005：复用一次性命令完整流程）。
5. 注册输出下次执行时间：沿用 `registerScheduledCommand` 内置 `describeSchedule` 输出（`[fansgroup] 下次执行: ...`），无需额外实现。
6. `task` 帮助文本 `Long`/`Example` 同步四项 → 五项，明示乐迷团任务会修改账号状态（播放、红心、点赞、动态），发布笔记默认保留（AC-026）。

## 8. Security

- 帮助文本明示：本命令修改账号状态（播放记录、红心列表、点赞、公开动态），发布笔记默认保留公开；`--delete` 仅删除本次执行链内动态；自动化行为存在账号风控风险，使用频率由用户自行决定。
- 不接受通过命令参数传入 Cookie 或 token；复用现有 Cookie 传输策略与 `--home`/`--config` 会话选择。
- 正常输出与错误不包含 Cookie、Token、设备 ID、完整加密请求体（PRD 5.2）。
- 不提供绕过已完成判定的参数；删除只接受本次执行链内的 event ID。
- 上传图片来自本地受控文件或团头像 URL，不引入外部图片 URL 池。

## 9. Testing Strategy

### 9.1 Unit Tests（`internal/ncmctl/fansgroup_test.go`，fake transport / 纯函数）

- `missionButtonParams` 黄金向量：播放任务（`songIds` 数组）、分享任务（`actionCustomParams.progressParams`，含 `resourceType`/`songId` 为数字的线上真实载荷）、字段全空、畸形 JSON、非标量类型（对象/数组/bool）报错；解析失败错误信息包含原文片段。
- `flexString` 单测：`"123"`/`123`/`0`/`-1`/转义字符串/`null`/字符串 `"null"` 的取值，对象、数组、`true` 报错。
- `sleepRange`/`sleep`：区间内取值（注入可控 rand 或统计边界样本）；`ctx` 已取消/取消中立即返回 `ctx.Err()`，不阻塞（AC-031）。
- 默认文案：长度 ≥10 Unicode 字符；随机后缀两次生成不相等。
- `taskStatus` 聚合：done/partial/skipped/failed 组合下的团级判定，重点覆盖空集不判失败（D3）、点赞「帖子不足但全部成功」为 done（5.1.5）。
- 位置参数与 flag 互斥校验、`--group-id` 非数字、`--message` 长度不足等 `validate` 分支。

### 9.2 Offline API / Orchestration Tests

- 点赞过滤逻辑：已点赞/本人/空 threadId 帖子剔除；剔除后为空 → skipped（AC-015/016）。
- Feed 请求断言：`FansGroupFeedRecommendReq.FansGroupId` 等于当前团 ID（P1 修复项，防回归）。
- 播放链路编排：`SongPlayerV1` 返回空 `Data`、`Data[0].Code=404`、外层 `Code != 200` 三种失败的迭代降级（5.4）。
- `WebLog` 载荷黄金向量：startplay/play JSON 结构与值类型断言（`id` 数字、`sourceId`/`content` 字符串）。
- 发布编排：`--image` 本地路径校验失败快速返回；头像下载失败/超限时任务 failed 且临时文件已清理；`activityInfoList` 的 `id` 取自详情返回值而非硬编码。
- 删除编排：仅删除本次链内 event ID；删除失败不改任务状态、不改退出码。
- 退出码聚合：多团混合结果（团 A 全 failed、团 B 成功）命令返回错误；全 skipped/空列表返回成功。

### 9.3 Task Tests

- `taskSelection`：`--fansgroup` 单选、`--runAll` 五项全选、空选择报错信息含 `--fansgroup`（AC-024/026/027）。
- `Task.validate`：非法 `--fansgroup.cron` 在启动 cron 前失败（AC-028）。
- 注册路径：`NewFansGroup` 满足 `scheduledCommand` 接口；opts 复制后 flag 值完整传递。

### 9.4 Acceptance Criteria Mapping

| AC | 章节 | AC | 章节 |
|----|------|----|------|
| AC-001 帮助 | 4.1/8 | AC-017/018/019 发布/删除 | 5.1.6/5.1.8 |
| AC-002 未登录 | 5.1.1 | AC-020 图片回退 | 5.1.6 |
| AC-003/004/005 status | 5.1.1/5.2 | AC-021/022 加速任务 | 5.1.7 |
| AC-006 非法 group-id | 5.2 | AC-023 最终进度 | 5.1.8 |
| AC-007/008 多团/隔离 | 5.1.1 | AC-024~029 task | 7/9.3 |
| AC-009/010/011 查询/展示/跳过 | 5.1.1/5.1.2 | AC-030 离线验证 | 9 |
| AC-012/013 播放 | 5.1.3 | AC-031 随机等待 | 3.2/5.1.1 |
| AC-014/015/016 分享/点赞 | 5.1.4/5.1.5 | | |

## 10. Implementation Plan

### 10.1 Phases

| 阶段 | 内容 | 完成门禁 |
|------|------|---------|
| Phase 1 | **协议证据确认**：11.1 全部未验证点（Q1~Q7），用固定样本或授权 live 请求确认；产出：确认结论或 spec 修订 | 11.1 清零或转化为已确认契约 |
| Phase 2 | API 层类型化：`FansGroupFeedRecommendResp.Data` + `FansGroupFeedPost` + 黄金向量单测 | `go test ./api/eapi` |
| Phase 3 | 命令骨架 + `status` 只读模式 + `validate` 全分支 + `validateLocalImage` 提取（share.go 回归） | `go test ./internal/ncmctl` + lint |
| Phase 4 | 五类任务执行器 + sleep 体系 + 聚合退出码 + 边界防护（空切片/404/空帖子） | 9.1/9.2 全绿 + race |
| Phase 5 | task 集成（五项 runAll）+ 帮助文本 + `docs/usage.md` | 9.3 全绿 + `make lint` + `git diff --check` |

### 10.2 Incremental Delivery

每阶段独立可编译、可测试、可 review；Phase 3 结束即可交付只读查询能力；Phase 4/5 结束交付完整任务能力。不依赖真实账号的验证边界（live 测试）需用户明确授权后单独执行（`NCMCTL_RUN_LIVE_TESTS=1`）。

## 11. Open Questions & Risks

### 11.1 Unresolved Questions（Phase 1 门禁）

| # | 问题 | 影响面 | 来源 |
|---|------|--------|------|
| Q1 | ~~**短参数偏差决策**~~ **已关闭（采纳 PRD FR-01）**：已按 `-g`/`-t`/`-m`/`-i`/`-d` 注册短参数，D6 随之修订为带短参数方案。`share.go` 的 `--delete` 用大写 `-D` 属冲突规避（`-d` 已给 `--draw`），本命令无 `--draw` 故取小写 `-d` | 4.1 flag 注册 | 用户确认 |
| Q2 | `FansGroupFeedRecommendResp.Data` 下帖子数组的挂载层级与字段路径（`threadId`/`info.liked`/`user.userId`） | 3.1 类型化、5.1.5 过滤 | PRD 风险表 |
| Q3 | `ResourceLikeReq.AppLogExt` 的 JSON 结构（`addRefer`/`multiRefer` 如何指向乐迷团 ID） | 5.1.5 点赞计数 | PRD 风险表 |
| Q4 | `WebLog` play 载荷细节：`source`/`sourceId` 在无专辑时的取值；play 事件是否必须携带 `content`；`id`（数字）与 `sourceId`（字符串）的值类型差异 | 5.1.3 播放有效性 | 源码样本注释 |
| Q5 | `SongLikeReq.Time`（默认 `"3"` 的语义）与 `CheckToken` 是否必需 | 5.1.7 红心链路 | 源码字段 |
| Q6 | 加速任务预归一化时序（先取消红心再收藏）是否必要 | 5.1.7 | PRD 风险表 |
| Q7 | 播放链路有效性：512 KiB 拉取 + 3~5 秒上报是否被服务端计为有效播放；无效时命令不得误报成功 | 5.1.3 聚合语义 | PRD 风险表 |
| Q8 | 默认乐迷团 ID `1872529203038486609` 是否沿用（来自参考实现作者加入的团），或改为文档示例引导显式传入 | `defaultFansGroupID` 常量 | PRD 待确认项 |

### 11.2 Technical Risks

| 风险 | 缓解 |
|------|------|
| 标题关键词分发依赖官方文案（播放/分享/点赞/笔记/发布），文案变化需跟随调整 | 未知类型安全跳过并输出原文（D7）；分发规则集中在一张表 |
| 等待区间未经风控实测标定 | 区间集中常量定义，调整只改一处（D5）；机制不变 |
| 任务进度异步更新导致最终回显滞后 | 最终进度尽力而为，不作为成功判定（PRD 已定边界） |
| 自动化行为的账号风控 | 帮助文本明示；区间与现有命令一致，不做更激进模拟 |

### 11.3 Assumptions

- 删除时序采用「发布循环全部完成后统一删除」（D2）；备选方案「每篇发布成功后立即删除」被否决，理由：删除后置于任务确认，避免删除时序影响服务端任务计数（PRD 产品方案关键决策 4）。
- task 默认调度 `30 10 * * *`（Asia/Shanghai 每天 10:30，与其他任务错峰）为 PRD `[Assumption]`。
- 团级聚合「至少执行过一个任务且全部 failed 才判团失败」是对 PRD FR-09「至少一个乐迷团全部任务失败」字面边界的消解（任务列表为空/全 skipped 不判失败），已固化为 D3 与 6.1。
- 剩余次数 `<= 0` 时按 1 次处理（PRD FR-03 规则 5）。

## 12. References

### 12.1 Repository Contracts

- `api/eapi/fansgroup.go`：`FansGroupDetailGet`/`FansGroupUserGroupDetailGet`/`FansGroupMissionAll`/`FansGroupFeedRecommend`/`FansGroupMissionForwardProgress`/`ResourceLike` 签名与响应结构（`FansGroupMissionItem` 含 `Title/Status/CurrentProgress/AllProgress/Integral/Button.Url/IconUi.TargetUrl`）。
- `api/eapi/event.go`：`EventUploadImage(ctx, filePath) (string, error)`、`EventPublish`（`ActivityInfoList` 格式注释 :48）、`EventDelete`。
- `api/eapi/song.go`：`SongLike`（`SongLikeResp` 仅 `Code`，:297-299）。
- `api/weapi/feedback.go`：`WebLog`（startplay/play 样本注释 :67-74）。
- `api/weapi/song.go`：`SongPlayerV1`（`Resp.Data` 切片，`Data[].Code` 404=下架，:297-330）、`SongDetail`。
- `api/types/quality.go`：`types.Level`、`LevelStandard`。
- `internal/ncmctl/share.go`：命令骨架、`prepareImage` 模式、`validateLocalImage` 提取点。
- `internal/ncmctl/task.go`：`scheduledCommand` 接口、`registerScheduledCommand`、`taskSelection`、`describeSchedule`。
- `internal/ncmctl/utils.go`：`closeAPIClient`。
- `AGENTS.md`：核心开发原则（2.5 契约的映射来源）。

### 12.2 Product References

- `docs/prd/prd-003-fansgroup.md`：FR-01~10、AC-001~031、防风控等待表、风险表。
- `docs/spec/spec-002-daily-song-challenge.md`：SPEC 结构与章节惯例参照。
