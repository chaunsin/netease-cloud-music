# SPEC: ncmctl share 每日推歌挑战赛

> Technical specification derived from: [docs/prd/prd-002-daily-song-challenge.md](../prd/prd-002-daily-song-challenge.md)
> Generated: 2026-08-20 | Target branch: develop | Commit: 51fc1a5

## 1. Summary

### 1.1 What This SPEC Covers

本 SPEC 定义 `ncmctl share` 一次性命令、`status`/`draw` 子命令，以及将每日推歌接入现有 `ncmctl task` 的实现契约。一次性流程负责会话检查、活动报名、周期报名、状态幂等判断、选歌、图片准备、公开笔记发布、分享触发、服务端进度确认、抽奖和可选的抽奖后删除。

实现以现有 `api.Client`、Cookie、XEAPI/WEAPI 封装和 `task` 调度生命周期为边界，不新增账号配置、多账号队列、数据库或独立调度器。活动进度、活动 ID、抽奖次数和 event 状态均以服务端响应为准；客户端不写“今日已发布”之类的本地标记，也不因后续失败自动重发公开笔记。

### 1.2 PRD Reference

- Source: [docs/prd/prd-002-daily-song-challenge.md](../prd/prd-002-daily-song-challenge.md)
- User Stories covered: US-001 ～ US-007
- Functional Requirements covered: FR-01 ～ FR-11
- Acceptance Criteria covered: AC-001 ～ AC-026

### 1.3 Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| 命令形态 | 顶层 `share` + `status`、`draw` 子命令 | 读操作与账号写操作分离；父命令复用现有单次命令生命周期 |
| 活动事实源 | `DailySongShareRegistrationGuide` | 周期、报名、打卡、奖励和抽奖信息不在本地重算，避免重复发布 |
| 会话与客户端 | 使用根命令初始化的 `Cfg.Network`，每次命令创建标准 `api.NewClient` | 复用 `--home`、`--config`、Cookie、XEAPI 状态和关闭流程，不建立第二套会话 |
| 发布载荷职责 | CLI 只组装歌曲、用户、标题、正文和上传后的 `Pics`；API wrapper 保留既有 XEAPI 选项、默认字段和请求头 | 遵循当前 `api/eapi/daily_song_share.go` 的边界，避免在 CLI 复制协议默认值 |
| 抽奖默认行为 | 发布命令默认 `--draw=true`；次数由 guide 和每次 lottery 的 `RestChance` 共同约束 | 完成推歌后及时消费周期机会，同时防止客户端超发 |
| 删除时序 | 只删除本次发布返回的正数 event ID，并且严格位于最后一次成功抽奖之后 | 删除过早可能使打卡失效；默认不删除，异常时保留动态以便恢复 |
| task 兼容性 | 无选择器且不带 `--runAll` 直接报错；`--runAll` 注册全部四项（含每日推歌） | 避免未经用户明确选择就发布公开动态 |
| 状态码 | 成功/合法跳过退出 0；前置失败、部分成功和删除失败返回非零 | 便于 shell 和长期任务区分“无需操作”与“需要检查” |
| 协议证据 | 当前 XEAPI wrapper 和离线 wire 测试是基础；抽奖参数、发布协议字段和运行时反作弊字段未验证前不得猜测 | 当前 PRD 标记为实现前确认协议兼容，内部加密往返不能证明线上兼容 |

---

## 2. Architecture

### 2.1 System Context

```text
用户
 │
 ├─ ncmctl share
 │       ├─ status: GetUserInfo → guide
 │       ├─ draw:   GetUserInfo → guide → lottery × N
 │       └─ publish: guide → 报名/周期报名 → 选歌 → 图片上传
 │                         → publish → trigger → guide → lottery → delete(可选)
 │
 └─ ncmctl task
         ├─ 既有 sign / partner / scrobble
         └─ share: 注册 cron，触发同一个 publish 执行路径

Root.Cfg / Root.l / api.NewClient
       ├─ api/weapi: GetUserInfo / RecommendSongs / SongDetail
       ├─ api/eapi XEAPI: guide / register / attendance / publish / trigger / lottery
       ├─ api/eapi EAPI: EventUploadImage / EventDelete
       └─ net/http: 歌曲封面下载（无 Cookie，受 context 和大小上限约束）
```

命令不绕过根命令的配置初始化、Cookie transport、XEAPI session 和 `closeAPIClient`。task 只负责选择任务、校验 cron、注册 job 和记录调度错误，不直接调用活动 API。

### 2.2 Component Design

#### `internal/ncmctl/daily_song_share.go`

新增 `DailySongShareOpts` 与 `DailySongShare`，遵循现有 Cobra 命令的 `NewXxx`、`Command`、`Add`、`addFlags`、`validate`、`execute` 结构。

`DailySongShareOpts` 至少包含：

- `SongID`：未指定时为 0，表示使用每日推荐。
- `Image`、`Title`、`Message`：一次性发布输入；空标题/正文在解析后补默认值或快速失败。
- `Draw`：默认 `true`。
- `DeleteAfterLottery`：默认 `false`。
- `DryRun`：默认 `false`；仅一次性父命令可用。
- `Count`：仅 `draw` 使用；未指定时以服务端全部剩余次数为准。

命令内部只保留单次运行状态：当前 guide、登录用户 ID、歌曲信息、临时封面路径、event ID、抽奖成功标记和状态确认标记。任务参数复制到一次性命令实例后执行，不另建第二套业务流程。

#### `internal/ncmctl/task.go`

扩展 `TaskOpts` 和 flag 绑定：

- 添加 `DailySongShare` 任务选择器。
- 添加 `DailySongShareOptsCrontab`，默认 `0 9 * * *`。
- 添加 `--share.song-id/image/title/message/draw/delete`。
- `task` 不暴露 `dry-run` 和 `count`，长期任务始终执行真实状态流程并使用服务端全部剩余抽奖机会。

`Task.validate` 在创建 cron 和 API client 前校验每日推歌 cron、`draw=false` 与 `delete` 的组合。`Task.execute` 通过现有 `registerScheduledCommand` 机制创建 `NewDailySongShare`，复制选项后执行父命令；不得把活动 API 编排复制进 `task.go`。

每日推歌 job 使用 `cron.SkipIfStillRunning` 或等效的单任务 guard，避免上一轮仍在执行时并发发布第二条动态。其他既有任务的并发和生命周期行为保持不变。

#### API 与文件下载边界

- 选歌复用 `api/weapi.RecommendSongs`、`api/weapi.SongDetail`。
- 活动流程复用 `api/eapi/daily_song_share.go` 的 wrapper。
- 图片上传和删除分别复用 `api/eapi.EventUploadImage`、`api/eapi.EventDelete`。
- 歌曲封面下载使用独立 HTTP client，不能把 CDN 下载请求混入 API Cookie transport；下载须遵循命令 context、响应大小上限和临时文件清理。

### 2.3 Module Interactions

一次性发布流程顺序固定为：

1. 先完成参数交叉校验；`--draw=false` 与 `--delete` 冲突时在任何网络请求前失败。
2. 创建 API client，调用 `GetUserInfo` 验证当前会话并获取 `Profile.UserId`。
3. 调用 `DailySongShareRegistrationGuide`，按 5.3 的状态机处理。
4. 仅非 `dry-run` 路径执行必要的首次报名/周期报名，并在每次报名后重新读取 guide。
5. 确定可发布且本周期未打卡后，按 5.1 选择歌曲、生成标题/正文并准备图片。
6. 上传一张图片，构造公开单曲资源笔记，调用 `DailySongSharePublish`；返回成功后立即保存 event ID。
7. 用同一歌曲调用 `DailySongShareTrigger`，再读取 guide 确认打卡状态。
8. `Draw=false` 时结束；否则按 5.1.2 抽奖。
9. 只有删除条件全部满足时，最后调用 `EventDelete`；任何前置条件不满足都保留动态。

特殊路径：

- `status` 只执行会话检查和 guide 查询，不报名、不选歌、不上传、不发布、不触发、不抽奖、不删除。
- `draw` 只读取 guide 并抽奖，不发布，也没有可绑定的本次 event ID，因此不支持删除。
- `dry-run` 可以读取会话/guide 并准备歌曲与标题/正文，但不报名、不上传封面、不发布、不触发、不抽奖、不删除。
- 发布前 guide 已显示本周期完成时直接合法跳过，不执行歌曲、图片、发布、抽奖或删除；需要独立消费已有机会时使用 `draw`。

### 2.4 File Structure

```text
internal/ncmctl/
├── daily_song_share.go       [new]     Cobra 命令、校验、编排、输出和辅助函数
├── daily_song_share_test.go  [new]     参数、状态机、请求顺序和输出的离线测试
├── task.go                   [modified] 任务选项、选择规则、cron 注册与每日任务 guard
├── ncmctl.go                 [modified] 注册 share 顶层命令
└── ncmctl_test.go            [modified] 命令帮助、参数和 task 选择行为契约
api/
├── api.go / options.go       [conditional] 为 lottery 提供请求级 no-retry 能力（若现有全局 retry 无法满足 FR-08）
└── eapi/
    ├── daily_song_share.go  [conditional] 仅按固定协议证据修正抽奖字段或补充活动字段
    └── event.go              [existing] 复用 EventUploadImage / EventDelete，不复制实现
docs/
├── usage.md                  [modified] 用户命令、task 选择和账号副作用
└── spec/spec-002-*.md        [modified] 本 SPEC
skills/ncmctl/
├── SKILL.md                  [modified] 快速参考和安全边界
└── references/commands.md    [modified] 命令参数、默认值、输出和 task 行为
README.md                     [modified] 每日任务能力清单（若该功能纳入项目总览）
```

不新增数据库表、配置文件、Cookie 文件参数、账号队列或独立调度服务。

---

## 3. Data Model

### 3.1 Schema Changes

无。本功能不保存活动进度、event ID、歌曲去重信息或抽奖次数。服务端 guide 是唯一状态源。

### 3.2 Entity Definitions

复用现有 API 类型，不为一次调用复制一份活动 DTO：

| 类型 | 关键字段 | 用途 |
|------|----------|------|
| `eapi.DailySongShareRegistrationGuideResp` | `RegisterStatus`、`ActivityId`、`ActivityCycleId`、`ActivityInterestId`、`Duration`、`RewardJumpUrl`、`RegisteredGuide` | 状态、周期、活动身份和奖励展示 |
| `RegisteredGuide` | `RewardCount`、`HaveRewardCount`、`AlreadyPubEvent`、`PubEventCount` | 服务端进度与剩余抽奖候选次数 |
| `eapi.DailySongSharePublishReq` | `Type`、`Id`、`Uid`、`Title`、`Msg`、`Pics`、公开设置及协议字段 | 发布单曲资源笔记 |
| `eapi.EventPublishResp` | `Code`、`Message`、`Id` | 记录本次 event ID |
| `eapi.DailySongShareLotteryResp` | `Code`、`Message`、`RestChance`、`PrizeDetailInfoMap`、`NoLotteryContent` | 抽奖结果和下一轮次数 |
| `weapi.GetUserInfoResp` | `Profile.UserId`、`Profile.Nickname` | 会话确认和发布 UID |
| `weapi.RecommendSongsResp` / `SongDetailResp` | 歌曲 ID、名称、艺术家、专辑名称、`PicUrl` | 选歌和封面来源 |

命令可以增加进程内 `activityState` 枚举（未报名、需周期报名、可发布、已打卡、已禁止、未知），但不应把该枚举持久化，也不应改变 API 层响应类型。

### 3.3 Relationships

- `ActivityCycleId`、`ActivityId`、`ActivityInterestId` 只来自当前 guide；不跨周期缓存。
- `event ID` 只在当前执行链内关联 publish、错误输出和最后的 EventDelete；不接受用户传入任意 event ID。
- 歌曲选择结果只在当前进程内使用；不做跨天去重，不写本地数据库。
- task 保存的是命令参数，不保存服务端状态；每次 cron 触发重新创建客户端并读取 guide。

### 3.4 Migration Plan

不适用。新增命令和 task flags 对既有命令语义保持兼容；但 `task` 不再保留无选择器默认三项的行为，无选择器且不带 `--runAll` 必须快速失败。

---

## 4. API Design

### 4.1 Command Surface

| 命令 | 作用 | 读取 | 写入/副作用 |
|------|------|------|-------------|
| `ncmctl share [flags]` | 报名、发布一次推歌、确认、默认抽奖，可选删除本次动态 | 用户信息、guide、歌曲 | 可能报名、上传图片、发布公开动态、触发、抽奖、删除本次动态 |
| `ncmctl share status` | 展示当前活动状态 | 用户信息、guide | 无账号写操作 |
| `ncmctl share draw [--count N]` | 使用已有活动抽奖机会 | 用户信息、guide | 消费抽奖次数 |

一次性发布 flags：

| Flag | 默认值 | 约束/行为 |
|------|--------|-----------|
| `--song-id` | 空 | 正整数；指定后只调用 `SongDetail`，不调用每日推荐 |
| `--image` | 空 | 目标必须是存在的非目录、非符号链接、非空常规文件；空时下载歌曲封面 |
| `--title` | `今日推荐：<歌曲名>` | 解析后非空；用户值优先 |
| `--message` | `分享一首今天听到的好歌，欢迎一起听听。` | `TrimSpace` 后至少 10 个 Unicode 字符 |
| `--draw` | `true` | 发布和状态确认后抽奖；`false` 时不调用 lottery |
| `--delete` | `false` | 仅父命令可用；要求 `--draw=true`，且只删除本次 event |
| `--dry-run` | `false` | 只读状态并准备歌曲/正文；禁止所有状态变更和图片上传 |

抽奖 flags：

| Flag | 默认值 | 约束/行为 |
|------|--------|-----------|
| `--count` | 未指定 | 未指定时使用服务端全部剩余候选次数；指定值必须为 1～8；发布与 `draw` 流程均适用 |

task flags：

| Flag | 默认值 | 约束/行为 |
|------|--------|-----------|
| `--share` | `false` | 选择每日推歌任务 |
| `--share.cron` | `0 9 * * *` | 五段式 cron，使用 task 的 `--location` 时区；时间为产品假设 |
| `--share.song-id` | 空 | 每轮固定歌曲 ID；空时每轮读取每日推荐 |
| `--share.image` | 空 | 每轮固定本地图片；空时使用歌曲封面 |
| `--share.title` | 空 | 覆盖一次性命令标题默认值 |
| `--share.message` | 空 | 覆盖一次性命令正文默认值 |
| `--share.draw` | `true` | 每轮默认抽奖；显式 `false` 关闭 |
| `--share.delete` | `false` | 每轮抽奖成功后删除本轮新 event；要求 draw 开启 |

所有上述命令拒绝位置参数。`--delete` 不属于 `status`/`draw`；`--count` 适用于发布与 `draw` 流程；task 不提供 `dry-run`。

帮助文本必须直接说明：发布会修改账号动态、默认公开、默认抽奖、默认不删除；删除可能影响全勤奖励资格；`--draw=false` 与删除参数不能组合。

### 4.2 Invoked NetEase Endpoints

| Wrapper | Logical endpoint | Crypto | 作用 |
|---------|------------------|--------|------|
| `weapi.GetUserInfo` | `/weapi/w/nuser/account/get` | WEAPI | 验证登录并取得 UID/昵称 |
| `weapi.RecommendSongs` | `/weapi/v3/discovery/recommend/songs` | WEAPI | 未指定歌曲时获取候选 |
| `weapi.SongDetail` | `/weapi/v3/song/detail` | WEAPI | 校验并读取指定歌曲 |
| `eapi.DailySongShareRegistrationGuide` | `/xeapi/note/attendance/activity/registration/v2/guide` | XEAPI | 读取唯一活动状态源 |
| `eapi.DailySongShareRegister` | `/xeapi/note/common/activity/in/registration` | XEAPI | 首次活动报名 |
| `eapi.DailySongShareAttendanceRegister` | `/xeapi/note/attendance/activity/register` | XEAPI | 周期报名；wrapper 强制 `AutoRegister=true` |
| `eapi.DailySongSharePublish` | `/xeapi/note/share/friends/resource` | XEAPI | 发布公开单曲资源笔记 |
| `eapi.DailySongShareTrigger` | `/xeapi/music/song/share/trigger` | XEAPI | 触发活动分享；默认 channel 为 `cloudmusic` |
| `eapi.DailySongShareLottery` | `/xeapi/middle/play/do/lottery` | XEAPI | 消费抽奖机会 |
| `eapi.EventUploadImage` | NOS token → PUT → `/eapi/upload/event/img/v1` | EAPI + upload | 上传一张动态图片并生成 `Pics` |
| `eapi.EventDelete` | `/eapi/event/delete` | EAPI | 删除本次正数 event ID；仅在最终清理阶段调用 |

XEAPI wrapper 的外层传输、Cookie、请求头和 session 状态由 `api.Client` 处理。不要在 CLI 拼接 `B/S/R`、伪造 `checkToken`、复制移动端 anti-cheat 参数或替换 `Client.GetClient().Transport`。

### 4.3 Request/Response Schemas

#### Guide 与进度

CLI 按原始字段输出以下信息：

```text
周期: Duration / ActivityCycleId
报名状态: RegisterStatus
活动 ID: ActivityId
活动兴趣 ID: 仅在需要诊断时显示数值，不写入本地
已发布笔记数: RegisteredGuide.PubEventCount
抽奖总数: RegisteredGuide.RewardCount
已使用/已获得字段: RegisteredGuide.HaveRewardCount（按服务端字段名展示）
候选剩余次数: max(RewardCount - HaveRewardCount, 0)
全勤/奖励跳转: RewardJumpUrl（非空时）
```

客户端不根据本地日期推算“今天”，也不把 `PubEventCount` 伪装成自然日列表。

#### Publish 请求

CLI 组装的最小业务字段必须满足：

- `Type="song"`。
- `Id` 为选中歌曲 ID 的十进制字符串。
- `Uid` 为 `GetUserInfoResp.Profile.UserId` 的十进制字符串。
- `Title`、`Msg` 为最终文本；`Msg` 去首尾空白后至少 10 个 Unicode 字符。
- `Pics` 为 `EventUploadImage` 返回的单图片 JSON 字符串，不为空。
- `PrivacySetting="0"`、`SocialSpaceVisible=1`，明确表示公开可见。
- `ThreadId`、`ResourceId` 与歌曲 ID 对应；当前 wrapper 在 `Type="song"` 且 `Id` 非空时负责派生 `R_SO_4_<id>` 和资源 ID。

当前 wrapper 已负责的 `AutoSaveDraft`、`UseNewUpload`、`FromRn`、`NeedsGuardianToken`、`OS`、发布时间和活动发布请求头由 API 层统一设置，CLI 不重复散落默认值。`Uuid`、`ServerUuid`、`SessionId`、`PubTraceId` 等协议字段若被确认必填，则按固定协议向量在 CLI/API 边界统一生成和测试；不得使用无证据的活动话题或 anti-cheat 值填充 `ActivityInfoList`。

#### Lottery 请求与响应

guide 返回的 `ActivityInterestId` 是业务要求的抽奖身份。当前 API 类型的请求字段名为 `activityId`，实现前必须使用固定协议证据确认其 wire 语义；在确认前不得把任意硬编码活动 ID 发送到 lottery。每轮请求只使用当前 guide 的身份，响应 `Code=200` 且能解码为已知结构才算一次已知结果。

每轮输出 `PrizeDetailInfoMap` 中有返回的奖品名称/中奖描述/领取链接及 `RestChance`。没有奖品或服务端返回 `NoLotteryContent` 时按服务端文本展示，不本地补全奖品。

#### EventDelete 请求

```json
{"id": <本次 DailySongSharePublish 返回的正数 event ID>}
```

不接受用户传入 event ID，不删除历史动态，不在抽奖前调用。

### 4.4 Error Responses

API wrapper 的传输/解码错误和业务 `Code != 200` 必须分开处理。命令层将业务失败包装为包含操作名、code、message 的错误；不得把非 200 当作成功，也不得把未知 lottery 网络结果自动重试后继续删除。

### 4.5 Breaking Changes

新增命令和 task flags 不改变既有命令语义。`--runAll` 注册四项任务；不带选择器且不带 `--runAll` 的 `task` 必须快速失败，不再默认三项。若为 lottery 增加请求级 no-retry 能力，应设计为 API 内部/通用选项的向后兼容扩展，并配套现有 API 测试。

---

## 5. Business Logic

### 5.1 Core Algorithms

#### 5.1.1 发布与 `dry-run`

```text
validate flags
if draw == false && delete:
    fail before creating client or sending request

client = api.NewClient(root.Cfg.Network, logger)
defer closeAPIClient(ctx, client, logger)
weapi = weapi.New(client)
eapi = eapi.New(client)

user = GetUserInfo()
if transport/decode failure or user.Code != 200 or user.Profile == nil:
    fail need-login / operation error

guide = RegistrationGuide()
state = classify(guide)
if state == completed:
    print progress and skip with exit 0
if state == forbidden or state == unknown:
    print raw RegisterStatus/reason and skip without writes

if dry-run:
    song = selectSong()                  # still no write
    title, message = resolveText(song)
    print state, song, title, message
    return success/skip

if state == not-registered:
    Register()
    require Code == 200
    guide = RegistrationGuide()          # failure stops before publish
    state = classify(guide)

if state == needs-cycle-registration:
    require ActivityId > 0 && ActivityCycleId > 0
    AttendanceRegister(ActivityId, ActivityCycleId, AutoRegister=true)
    require Code == 200
    guide = RegistrationGuide()
    state = classify(guide)

require state == ready-to-publish
song = selectSong()
title, message = resolveText(song)
image = validateLocalImage() or downloadCover(song.PicUrl, 20 MiB limit)
pics = EventUploadImage(image)
publish = DailySongSharePublish(song, user, title, message, pics)
require publish.Code == 200
eventID = publish.Id                  # retain even if later steps fail

trigger = DailySongShareTrigger(song.Id, channel="cloudmusic")
if trigger transport/business failure or trigger.Data != true:
    return partial success with eventID; no redraw/no delete

confirmed = RegistrationGuide()
if confirmed fails or does not show completed:
    return partial success with eventID; no lottery/no delete

if draw == false:
    print success and eventID; return

lotteryResult = drawAvailable(confirmed, Count)
if lotteryResult has unknown failure:
    return partial/failure according to whether a publish exists; no delete
if lotteryResult has no usable chance:
    print no available opportunity; return success/skip; no delete

if delete && eventID > 0 && at least one lottery request succeeded:
    delete = EventDelete(eventID)       # last network operation
    if delete fails or delete.Code != 200:
        return partial success with eventID and "dynamic remains"
    print lottery complete and dynamic deleted
return success
```

状态确认失败时即使 publish 已返回成功，也不能通过猜测继续抽奖或删除。再次运行时重新读取 guide；不能因为上次没有保存 event ID 就重新发布。

#### 5.1.2 抽奖循环

```text
guide = RegistrationGuide()
if guide is unregistered / ActivityInterestId <= 0 / server says not drawable:
    print no available opportunity; return skip 0

candidate = max(guide.RewardCount - guide.HaveRewardCount, 0)
if count was explicitly set:
    candidate = min(candidate, count)
candidate = min(candidate, 8)
if candidate == 0:
    return skip 0

for i in [1..candidate]:
    result = DailySongShareLottery(current ActivityInterestId)
    if transport/decode failure:
        stop immediately; do not retry or delete
    if result.Code != 200:
        stop immediately with code/message; do not delete
    print prize/result and result.RestChance
    successfulLottery = true
    if result.RestChance <= 0:
        break
```

`RestChance` 是下一轮是否继续的服务端事实；它不能为负数时继续循环。`--count` 只限制本次消费上限，不扩大服务端次数。

#### 5.1.3 选歌、图片和文本

- 有 `SongID` 时调用 `SongDetail`，要求返回匹配的正数 ID；不回退到推荐列表。
- 无 `SongID` 时调用 `RecommendSongs`，过滤正数 ID、非空歌曲名的候选后随机选择一首；若使用默认封面，还要求 `Al.PicUrl` 非空。
- 指定 `--image` 时不要求歌曲有封面 URL；未指定时必须下载选中歌曲封面。
- 封面下载使用 context、20 MiB 上限和受控临时文件；下载/上传失败时不调用 publish。
- 标题默认使用歌曲名；正文默认使用 PRD 规定文案；正文长度用 `utf8.RuneCountInString`，不是字节长度。

### 5.2 Validation Rules

| 输入/条件 | 规则 | 失败时机 |
|-----------|------|----------|
| 位置参数 | 所有相关命令使用 `cobra.NoArgs` | Cobra 解析阶段 |
| `--song-id` | 解析为正整数 | 任意网络请求前 |
| `--count` | 仅显式设置时允许 1～8 | 任意网络请求前 |
| `--image` | `Lstat` 成功、目标为非符号链接的常规文件、大小大于 0；不接受目录 | 任意网络请求前 |
| `--title` | 解析后的标题非空 | 任意网络请求前 |
| `--message` | `TrimSpace` 后 Unicode 字符数至少 10 | 任意网络请求前 |
| delete/draw | `delete` 只能用于父命令且要求 `draw=true` | 任意网络请求前 |
| task cron | 每日推歌启用时使用 `cron.ParseStandard` 校验 | cron 注册前 |
| task 选择 | 无 selector 且无 runAll = 报错；`--share` = 仅该项；`--runAll` = 4 项 | cron 注册前 |

参数校验不得通过 API 请求验证歌曲或图片；`--dry-run` 也必须先通过本地参数校验。

### 5.3 State Machine

状态判定只依据 guide 原始字段和服务端明确的状态文案；未知 `RegisterStatus` 不能当作可发布。

| 状态 | 判定 | 发布命令行为 | status/draw 行为 |
|------|------|-------------|-----------------|
| `completed` | `AlreadyPubEvent=true` 或服务端明确表示本周期已有合格发布 | 输出进度，合法跳过；不选歌、不上传、不抽奖、不删除 | 展示状态；draw 仍可按已有次数独立抽奖 |
| `not-registered` | 服务端明确表示尚未完成活动报名 | 调用 `DailySongShareRegister`，成功后重读 guide | 只展示，不能报名 |
| `needs-cycle-registration` | 服务端要求周期报名，且 `ActivityId`、`ActivityCycleId` 都是正数 | 调用 wrapper 强制 `AutoRegister=true`，成功后重读 guide | 只展示，不能报名 |
| `ready-to-publish` | 已报名、可参加、未完成本周期打卡 | 进入选歌和发布 | status 展示；draw 只抽奖 |
| `forbidden` | 服务端明确表示已放弃、不可再次报名或不可参与 | 输出原始原因，合法跳过，不绕过限制 | 展示原因，不写入 |
| `unknown` | 状态枚举或关键字段无法解释 | 输出原始 `RegisterStatus`，安全跳过，不发送写请求 | 展示原始值；draw 不抽奖 |

报名或周期报名成功但 guide 刷新失败时，不继续下游操作。`ActivityId`/`ActivityCycleId` 缺失时不能构造周期报名请求；`ActivityInterestId` 缺失时不能抽奖。

### 5.4 Edge Cases

- 登录检查失败：不上传、不发布、不抽奖、不删除。
- guide 已完成：不因 `--delete` 删除历史动态；本次没有新 event ID。
- 报名成功但 guide 刷新失败：停止，不依赖本地推断继续发布。
- 推荐列表为空、指定歌曲无详情、歌曲 ID/名称无效：发布前失败，不替换另一首歌曲。
- 自定义图片有效但歌曲无封面：允许使用自定义图片；默认封面不可用时失败。
- 图片是符号链接、目录、空文件、超过 20 MiB 或下载中断：不发布；封面临时文件无论成功失败都清理。
- publish 返回业务成功但 event ID 非正数：不删除；输出服务端响应缺少有效追踪 ID，并按部分成功/失败处理。
- trigger 失败、`Data=false`、guide 未确认完成或 guide 刷新失败：保留 event ID，不抽奖、不删除、不重发。
- `--draw=false`：发布和状态确认成功后结束，不调用 lottery；不能与删除参数组合。
- 无抽奖次数：不调用 lottery；即使启用删除参数也不能删除。
- lottery 业务失败、传输错误、解码错误或结果未知：立即停止，不重试，不删除；已知成功轮次仍输出。
- lottery 成功但没有奖品名称：按 `NoLotteryContent`/原始可安全字段输出，不硬编码奖品。
- delete 失败：返回部分成功，输出 event ID 和“动态仍保留”，不重发、不尝试删除其他 event。
- task 任务失败：只记录当前 job 错误并等待下一次 cron；不在同一轮自动重发。
- task job 重入：使用 skip guard；跳过重入不产生第二个 publish。

---

## 6. Error Handling

### 6.1 Error Taxonomy

| 类别 | 条件 | 退出/调度行为 | 必须输出 |
|------|------|---------------|----------|
| 成功 | status 查询完成、发布链完成、抽奖链完成或删除完成 | CLI 退出 0；task 记录 success | 进度、抽奖结果、event ID（有） |
| 合法跳过 | 已打卡、无抽奖次数、活动禁止/未知状态、dry-run | CLI 退出 0；task 记录 skip | 原始状态/原因；不得伪造完成 |
| 前置失败 | 未登录、guide/报名/选歌/图片/发布业务失败 | CLI 非零；task 记录当前错误 | 操作名、code/message（有）、下一步 |
| 发布后部分成功 | publish 成功后 trigger/guide/lottery 失败 | CLI 非零；task 等待下一轮 | event ID、失败阶段、`status` 建议；不重发 |
| 清理部分成功 | 发布、确认和至少一次抽奖成功，但 EventDelete 失败 | CLI 非零；task 等待下一轮 | event ID、动态仍保留；不重发 |

错误使用 `fmt.Errorf("operation: %w", err)` 保留根因。业务错误不得只输出 `%+v` 的完整请求对象；不得把 Cookie、Token、设备标识、完整加密 envelope 写入普通输出或错误。

### 6.2 Retry Strategy

- CLI 不增加业务重试；publish、trigger、register、attendance、upload、lottery、delete 均不因业务错误重放。
- lottery 的传输/解码错误属于未知消费结果，必须立即停止；实现必须确保 `api.Client` 的全局 retry 配置不会把 lottery 自动重放。若当前 Resty 全局 retry 无法按请求关闭，应先在 `api` 层增加请求级 no-retry 选项并用 fake transport 验证，再接入命令。
- 普通只读 guide/选歌请求可以沿用仓库现有客户端传输策略，但不得用重试掩盖业务 `Code != 200`。
- task 不在 cron job 内 sleep、紧密重试或创建第二次 publish；下一次 cron 是唯一恢复机会。

### 6.3 Failure Modes

- 服务端不可达：在当前阶段返回错误；已发布 event 必须带 ID，后续不推断状态。
- XEAPI session/公钥问题：保留 API 原始错误上下文，不伪造 `checkToken`/`S`/`B`。
- CDN 封面不可达：不回退到无图发布；用户可使用 `--image` 重新执行，但仍以服务端 guide 判断是否已完成。
- 活动状态字段漂移：未知状态安全停止并输出原始值，避免把新状态当作可发布。
- task 启动校验失败：在注册任何 job 前返回错误；不启动长期服务。

---

## 7. Security

### 7.1 Authentication & Authorization

- 只复用当前 `api.Client` 的登录 Cookie 和设备/session 状态；不提供 Cookie、anti-cheat token 或独立账号文件 flags。
- publish 前必须执行 `GetUserInfo`；task 启动沿用现有登录检查。
- 不提供 `--force` 绕过已打卡、抽奖前删除或任意 event ID 删除。
- `EventDelete` 的授权对象严格限定为当前执行链保存的 publish 返回值。

### 7.2 Input Validation

- 按 5.2 校验数值、路径、正文长度和 flag 组合。
- 图片文件不接受符号链接，避免命令在用户未明确指定的目标上读取。
- 封面 URL 只作为服务端歌曲信息提供的下载地址；下载请求不携带网易 Cookie，遵循 context 和大小限制。
- 不把用户正文扩展为外部话题、歌单 URL 或参考实现的固定 JSON。

### 7.3 Data Protection

- 普通输出只显示昵称/UID、周期、歌曲、CDN 主机名、event ID 和服务端奖励字段；不显示 Cookie、Token、设备 ID、请求头或加密正文。
- 不打印 `Uuid`、`ServerUuid`、`SessionId`、`PubTraceId` 等请求内部字段。
- 临时封面使用受控权限创建，上传完成或失败后清理；本地图片只读取用户指定文件。
- debug 日志仍遵循根命令现有敏感数据警告；本功能不新增完整请求体日志。

---

## 8. Performance

### 8.1 Expected Load

一次发布顺序执行有限请求：会话/guide、必要的报名、选歌、图片下载与上传、publish、trigger、guide、默认 lottery 若干次以及可选 EventDelete；不并发发布，不保存大响应到无界内存。抽奖最多 8 次，实际次数受 guide 和 `RestChance` 限制。

task 只在 cron 到点执行一次；每日 job 不能与自身并发重入。其他既有 task 任务不因本功能改变调度频率。

### 8.2 Optimization Strategy

- 使用根配置 timeout 和命令 context；不使用固定 sleep。
- 封面下载以 `Content-Length` 预检（若存在）并以 `LimitReader(limit+1)` 保护 20 MiB 上限，避免无界内存或磁盘写入。
- 图片只上传一张；复用 `EventUploadImage` 的一次图片流程，不在 CLI 复制 NOS token/PUT 逻辑。
- 抽奖循环不提前查询额外接口；使用 guide 候选次数和响应 `RestChance` 控制请求数。

### 8.3 Database Considerations

不适用。不得把活动状态或 event ID 写入现有 Badger 数据库；删除数据库不能影响服务端活动状态。

---

## 9. Testing Strategy

### 9.1 Unit Tests

在 `internal/ncmctl/daily_song_share_test.go` 覆盖：

- `--song-id`、`--count`、`--image`、`--title`、`--message` 和 draw/delete 组合校验。
- Unicode 9/10/11 字符边界，含多字节字符和首尾空白。
- 本地图片的目录、符号链接、空文件和普通文件判定。
- 指定歌曲不调用推荐列表；推荐列表过滤空 ID/名称/封面。
- 默认标题/正文解析、cover 下载大小限制和临时文件清理。
- guide 状态机：未报名、周期报名、可发布、已完成、禁止、未知。
- 抽奖次数计算、`--count` 钳制、8 次上限和 `RestChance=0` 提前停止。
- 输出脱敏与错误分类；event ID 在后续失败中不丢失。

### 9.2 Offline API / Orchestration Tests

使用 `api.Client.SetTransport`、静态固定响应和 `httptest`，不启用 `NCMCTL_RUN_LIVE_TESTS`，不读取 `testdata/`：

- 验证 WEAPI/XEAPI/EAPI endpoint 路径、方法和当前 wrapper 头部契约。
- 捕获并解密可恢复的请求载荷，断言 `type=song`、歌曲 ID、UID、标题/正文、`pics`、公开字段和 wrapper 默认字段。
- 断言 `status` 只访问用户信息/guide；`draw` 不访问发布、上传、trigger 或 delete。
- 断言完整调用序列：`publish → trigger → guide → lottery × N → delete`。
- 断言已完成、未报名、报名刷新失败、trigger 失败、guide 未确认、无次数和 lottery 未知结果的停止边界。
- 断言删除只收到本次 publish 的 event ID；删除失败保留 event ID。
- 用 fake transport 证明 lottery 没有自动重试；若新增请求级 no-retry 选项，覆盖其默认行为不受影响。
- 使用 `httptest.Server` 验证封面下载取消、超限和临时文件清理。

### 9.3 Task Tests

- `Task.validate`：每日 cron 解析、delete/draw 冲突、`--runAll` 选择；无 selector 且不带 `--runAll` 必须快速失败。
- task flags/help：每日推歌公开副作用、默认抽奖/不删除、默认 09:00 和 `--runAll` 四项说明。
- scheduler 注册：`--share` 只注册 daily；`--runAll` 注册四项；无 selector 且不带 `--runAll` 不注册任何任务并快速失败；下一次执行时间沿用现有日志格式。
- job guard：模拟长时间运行，证明同一 daily job 不会并发进入发布流程。
- task 单次错误：只记录错误，不停止 scheduler，不在同一 cron 轮次重发。

### 9.4 Acceptance Criteria Mapping

| US/FR | AC | 测试/验证 | 类型 |
|-------|----|-----------|------|
| US-001 / FR-01, FR-02, FR-10 | AC-001, AC-003, AC-009 | 命令帮助、status 只读、周期/进度/奖励链接输出与脱敏 | 单元 + 离线 |
| US-002 / FR-02, FR-03, FR-05, FR-06, FR-07 | AC-002, AC-004, AC-005, AC-007, AC-008, AC-013 | 登录门禁、报名刷新、公开单曲载荷、幂等、部分成功、临时文件 | 离线 |
| US-003 / FR-04, FR-05 | AC-006 | 指定歌曲绕过推荐、图片来源和校验失败不发布 | 单元 + 离线 |
| US-004 / FR-08 | AC-010, AC-011, AC-012, AC-026 | 无次数跳过、次数循环、RestChance 停止、未知结果不重试、关闭默认抽奖 | 离线 |
| US-005 / FR-06, FR-07, FR-10 | AC-008, AC-012, AC-014 | event ID 保留、后续错误传播、请求顺序和敏感输出 | 离线 + 本地质量检查 |
| US-006 / FR-09, FR-10 | AC-015, AC-016, AC-017, AC-018 | 默认不删、严格后置删除、抽奖失败不删、删除失败部分成功 | 离线 |
| US-007 / FR-11 | AC-019, AC-020, AC-021, AC-022, AC-023, AC-024, AC-025 | task 选择、cron/时区、复用父命令、兼容性、错误和重入 guard | 单元 + scheduler 测试 |
| 全局 | AC-014 | `go test`、race、lint、`git diff --check`、构建后核对 `--help` | 本地 |

安全边界：普通 `go test ./...` 和目标包测试不得访问真实账号；未获明确授权不运行 `make test-live`、integration 示例或任何会发布/抽奖/删除的真实 API 测试。

---

## 10. Implementation Plan

### 10.1 Phases

1. **协议与 API 边界（P0）**：用现有离线 wire 测试确认 publish wrapper 的字段默认、lottery 的 `ActivityInterestId`/`activityId` 语义和 lottery no-retry 方案；未确认项不得硬编码。
2. **命令骨架（P0）**：新增 `daily_song_share.go`，注册父命令和 `status`/`draw`，完成 flags、参数校验、帮助和输出模型。
3. **状态与选歌（P0）**：实现登录检查、guide 状态机、报名/周期报名刷新、幂等跳过、推荐/指定歌曲选择和文本校验。
4. **发布链（P0）**：实现封面/本地图片准备、上传、公开单曲 publish、trigger、guide 确认、event ID 保留和错误分类。
5. **抽奖与删除（P1）**：实现默认抽奖、`draw` 子命令、次数/RestChance 控制、严格删除门禁和 EventDelete；补齐 AC-015～AC-018、AC-026。
6. **task 集成（P0）**：扩展 `TaskOpts`、flags、选择规则、cron 校验、daily job 注册、skip guard 和错误日志；无 selector 且不带 `--runAll` 快速失败、`--runAll` 四项。
7. **测试与用户文档（P0/P1）**：完成离线编排、scheduler、race、lint 和构建帮助检查，更新 `docs/usage.md`、`skills/ncmctl/`、README 相关清单。

### 10.2 Issue Mapping

| Issue | SPEC Sections | Priority | Depends On |
|-------|---------------|----------|------------|
| #1 协议证据与 no-retry 边界 | 4.2～4.4, 6.2, 11.1 | high | — |
| #2 命令骨架、flags、状态与校验 | 2.2, 4.1, 5.2, 5.3 | high | #1 |
| #3 选歌、图片和发布编排 | 2.3, 4.3, 5.1.1, 5.1.3, 5.4 | high | #2 |
| #4 抽奖与删除 | 4.3, 5.1.2, 5.4, 6.1 | high | #1, #3 |
| #5 task 调度集成 | 2.2, 4.1, 5.2, 9.3 | high | #2, #3, #4 |
| #6 离线测试与质量门禁 | 9.1～9.4 | high | #2～#5 |
| #7 用户文档与帮助同步 | 2.4, 4.1, 9.4 | medium | #5, #6 |

### 10.3 Incremental Delivery

- 不新增 feature flag；命令是独立入口，既有命令不改变账号副作用。
- `status` 和 `draw` 可先完成离线契约，但未完成 publish/trigger/guide 确认前不应开放默认发布流程。
- task 的每日推歌只在一次性父命令和删除门禁测试完成后注册；`--runAll` 的行为变更必须与帮助和用户文档同批交付。
- 没有真实账号授权时，以固定协议向量、fake transport、httptest、race、lint、构建和 `--help` 为发布前验证边界。

---

## 11. Open Questions & Risks

### 11.1 Unresolved Questions

- **抽奖字段语义**：当前 `DailySongShareLotteryReq` 的 JSON key 是 `activityId`，PRD 要求传 guide 的 `ActivityInterestId`。必须由固定输入/输出或经授权的移动端请求证据确认是“key 保持 activityId、值传 interest ID”，还是 API 类型需要新增/重命名字段；确认前不得实现盲发。
- **发布协议字段**：`PubSource`、`ActivityInfoList`、`Uuid`/`ServerUuid`/`SessionId`/`PubTraceId` 的必填性和取值规则需由当前协议证据确定。不得复制无关乐迷团话题、固定活动 ID 或参考实现的 JSON。
- **运行时反作弊字段**：当前 XEAPI 文档明确 `checkToken`/`t1`/`t2` 属于运行时 token，本地客户端不应伪造。若活动接口要求这些字段，需要先决定由 API client 的现有会话能力承载，还是安全地将功能标记为不兼容；不新增用户输入 token。
- **`RegisterStatus` 枚举**：现有类型为字符串但没有仓库内完整枚举。需要固定状态样本覆盖未报名、已报名、周期报名、已放弃和已完成；未知值必须继续安全跳过。
- **Lottery no-retry 实现**：当前 `api.NewClient` 从配置设置 Resty 全局 retry count。必须确认请求级关闭 retry 的最小 API 设计，避免 lottery 传输错误被底层自动重放。

### 11.2 Technical Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| 活动接口或风控拒绝自动化发布 | 发布失败、账号受到限制 | 明确公开动态副作用；不绕过验证码/风控；仅在授权后做真实验证 |
| 抽奖字段错误或底层重试 | 消费错误活动次数，甚至误删动态 | 固定 wire 证据、request payload 测试、no-retry、删除前严格状态门禁 |
| publish 成功但后续请求失败 | 重复发布或用户误以为未完成 | 保留 event ID，返回部分成功，要求再次以 guide/status 恢复判断 |
| 删除与全勤规则冲突 | 动态删除后可能失去全勤资格 | 默认不删除；帮助、task 注册日志和删除成功输出明确警告 |
| task 长期运行中的 job 重入 | 同一日多次发布 | 每日 job 使用 skip guard，失败等待下一次 cron |
| 活动规则和奖品池变化 | 旧字段/奖品解释失效 | 输出服务端原始字段和链接，不硬编码奖品名称或领取接口 |
| XEAPI 会话状态未线上验证 | 离线测试通过但线上失败 | 以源码+固定向量为离线边界，真实账号测试必须单独授权 |

### 11.3 Assumptions

- UID 使用 `GetUserInfoResp.Profile.UserId`，以十进制字符串写入 publish 请求。
- 成功/合法跳过退出 0；前置失败、发布后部分成功和删除失败退出非零；task job 错误只写 scheduler 日志并继续服务。
- 封面下载上限为 20 MiB；使用独立下载 client，不携带网易 Cookie，临时文件使用受控权限并清理。
- `--dry-run` 可读取 guide 并准备歌曲/标题/正文，但绝不报名、上传、发布、触发、抽奖或删除。
- task 默认每日 09:00，实际时区由 `--location` 决定；该时间只是 PRD 假设，用户可覆盖 cron。
- 未知 `RegisterStatus` 按安全跳过处理并输出原始值；绝不把未知状态当作可发布。
- 活动周期的自然周和“今天”由服务端 guide 判定，CLI 不根据本地时间自行计算。
- 全勤奖励没有明确的独立领取 wrapper 时，只展示 `RewardJumpUrl` 和服务端奖励信息，不自动领取。
- 本 SPEC 不读取、不复制 `testdata/` 中的参考实现；参考代码不能替代当前仓库的 API、Cookie 和 XEAPI 契约。

---

## 12. References

### 12.1 Repository Contracts

- `api/eapi/daily_song_share.go`：活动报名、周期报名、guide、publish、trigger、lottery wrapper。
- `api/eapi/event.go`：`EventUploadImage` 和 `EventDelete`。
- `api/weapi/login.go`：`GetUserInfo` 与 `Profile.UserId`。
- `api/weapi/recommend.go`、`api/weapi/song.go`：每日推荐和歌曲详情。
- `internal/ncmctl/task.go`：现有 cron、时区、任务选择、错误日志和长期生命周期。
- `internal/ncmctl/ncmctl.go`、`docs/usage.md`、`skills/ncmctl/references/commands.md`：命令注册、帮助和用户文档边界。
- `.agents/skills/ncmctl-dev/references/protocols.md`、`docs/xeapi.md`：XEAPI 固定证据、运行时 token 和不应伪造的协议字段。

### 12.2 Product References

- [每日推歌挑战赛规则说明](https://y.music.163.com/g/yida/b43648015e0c44e1936ac64eefb14625?fromRN=1)
- [每日推歌挑战赛活动页](https://y.music.163.com/g/yida/67552d649bf9453abff1668bb2ba44c7?fromRN=1)
