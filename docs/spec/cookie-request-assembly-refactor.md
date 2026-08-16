# Cookie 请求组装职责重构 SPEC

> 状态：已实施
>
> 更新日期：2026-08-16
>
> 适用范围：`api.Options`、`api.Client.Request`、`requestCookiePolicy`、`cookieTransport`

本文定义 Cookie 请求组装职责重构的当前行为和验收标准。后续修改必须以本文和对应回归测试为准。

## 1. 背景

重构前，Cookie 处理由 [`api/api.go`](../../api/api.go)、[`api/options.go`](../../api/options.go) 和 [`api/cookie_transport.go`](../../api/cookie_transport.go) 共同完成：

- `Options` 接收请求级 Cookie，但校验、复制和同名覆盖在 `requestCookiePolicy` 构造阶段才完成。
- `Client.Request` 需要从 Cookie 中读取 CSRF、设备身份、协议 Header 和 XEAPI 加密字段，但最终 Cookie 主要在 transport 边界合并。
- `requestCookiePolicy` 同时承担 Options 输入清洗、首跳身份快照、默认值管理和逐跳合并，职责过多。
- 若把 Jar/default 自动值写入首跳 Cookie Header，transport 只能按 Name/Value 猜测重定向 Header 中的来源，无法区分原样复制与 `CheckRedirect` 删除后等值重加。

这使“谁决定 Cookie”“谁组装首跳请求”“谁处理后续物理请求”不够直观，也增加了重复 Cookie、错误覆盖和重定向行为回归的风险。

## 2. 目标

重构后按以下边界分工：

```text
Options
  校验、复制、同名覆盖
        |
        v
Client.Request + requestCookiePolicy
  按最终首跳 URL 解析来源、设置模式默认值、冻结协议身份
        |
        v
cookieTransport
  以当前 Header 为显式层，每次 RoundTrip 重查 Jar、补默认值、即时回写 Set-Cookie
        |
        v
lower http.RoundTripper
```

具体目标：

1. `Options` 在 `SetCookies` 调用时完成请求级 Cookie 的校验、复制和同名覆盖。
2. `Client.Request` 在加密模式 `switch` 中设置模式专属 Cookie，最终只把 Options 显式 Cookie 写入请求 Header。
3. `requestCookiePolicy` 只管理来源优先级、默认值和协议身份快照，不再清洗 Options 输入或猜测 Header 来源。
4. `cookieTransport` 在包括首跳在内的每次实际发送前查询当前 URL 的 Jar，正确处理重试和多次重定向。
5. 保持此前确定的线上请求、重定向、Cookie 回写和持久化行为，除修复重复 Cookie 及本文明确列出的 API 变化外，不引入协议变化。

## 3. 非目标

本次不处理以下事项：

- 不把持久化 Jar 重新安装到 `Client.GetClient().Jar`。
- 不改变 `pkg/cookie` 的文件格式、Public Suffix List、同步周期、单写者或关闭行为。
- 不改变 Go/Resty 的重定向次数、方法转换、敏感 Header 传播或重试策略。
- 不新增跨域身份熔断、禁止 `307/308`、关闭重定向等限制。
- 不改变 API、WEAPI、EAPI、LinuxAPI、XEAPI 的加密算法、请求路径或响应解码协议。
- 不新增单数 `SetCookie`、Options 批量 Cookie getter、Cookie 配置项或持久化字段。
- 不顺带调整 TLS、日志、命令、端点或其他无关代码。

## 4. 行为兼容契约

### 4.1 来源优先级

一次逻辑请求的 Cookie 优先级固定为：

```text
Options.SetCookies > 当前 URL 的 Cookie Jar > 当前加密模式的协议默认值
```

必须满足：

- Cookie Name 区分大小写。
- Options 中同名 Cookie 只发送最后一次设置的值。
- Options 的显式空值表示有效覆盖，必须阻止 Jar 和默认层补入同名非空值。
- Options Cookie 按 Name 覆盖所有低优先级同名值，不按 Domain 或 Path 区分。
- Jar 中不同 Path 的合法同名 Cookie 在没有 Options 覆盖时必须全部保留，并维持 Jar 返回的顺序。
- 协议默认值只在 Options 和 Jar 均没有同名 Cookie 时补入。

Options 是单次请求覆盖层。其 `Domain`、`Path` 等属性仍需通过 `http.Cookie.Valid()` 校验和复制，但写入请求 Cookie Header 时只使用标准请求 Cookie 表示；不同 Path 的同名保留仅属于 Jar 层。

### 4.2 默认值信任域

协议默认 Cookie 只允许发送到：

- `music.163.com`
- `music.163.com` 的子域

比较时忽略主机名大小写和末尾的点。IP、`example.com`、对象存储、CDN、代理捕获域名及其他网易域名均不属于默认值信任域。

目标不在信任域时：

- Options Cookie 仍可发送。
- Jar 仍按实际请求 URL 的作用域发送。
- 协议默认 Cookie、生成的设备身份和匿名 Token 不得注入。

### 4.3 URL 与 Host

- Cookie 查询、合并和响应回写始终使用 `req.URL`，不能使用自定义 `req.Host` 替换作用域。
- 自定义 `Host` 只改变线上 HTTP authority。
- XEAPI 必须先完成 `/api` 或 `/eapi` 到 `/xeapi` 的传输 URL 改写，再以改写后的 URL 查询 Jar 和建立首跳快照。
- Query 不参与首跳身份匹配；Scheme、Host 或 Path 改变时视为不同 Cookie 目标。

### 4.4 逻辑请求与物理请求

一次 `Client.Request` 是逻辑请求；首跳、重试和每次重定向是物理请求。

- Options Cookie 和协议身份在逻辑请求构造阶段确定。
- 普通 Jar Cookie 在每次物理请求发送前重新查询。
- 中间响应写入 Jar 的普通 Cookie 必须能被下一次重试或重定向立即看到。
- 已参与 CSRF、协议 Header、设备身份或加密载荷计算的 Cookie 在原始 URL 及其重试中保持首跳快照，避免线上 Cookie 与已生成的 Header/密文身份不一致。
- 响应更新的协议身份仍写入 Jar，但只供后续逻辑请求使用。

需要冻结的名称沿用现有 `protocolCookieNames`：

```text
__csrf, __csrf_token, MUSIC_U, MUSIC_R_U, MUSIC_A,
appver, buildver, channel, deviceId, sDeviceId, sdeviceId,
mobilename, ntes_kaola_ad, os, osver, resolution, versioncode,
WEVNSM, WNMCID, x-antiCheatToken
```

## 5. Options 契约

### 5.1 接口形态

`Options` 的 `Method`、`CryptoMode` 和 `Headers` 保持公开。现有公开字段 `Cookies` 改为私有存储，外部只能通过 `SetCookies` 写入请求级 Cookie。

`SetCookies` 保持现有无返回值签名：

```go
func (o *Options) SetCookies(cookies ...*http.Cookie)
```

不新增功能相同的单数 setter。

### 5.2 写入规则

每次 `SetCookies` 调用按以下顺序处理：

1. 忽略 `nil` 项。
2. 复制剩余 Cookie，调用 `http.Cookie.Valid()` 校验副本。
3. 任一项非法时，整批调用不写入任何 Cookie。
4. 全部合法后，按区分大小写的 Name 合并到 Options 私有层。
5. 同名旧值被移除，新值放在最后一次设置的位置；因此最后一次设置生效，同时保持可预测顺序。

空 Value 是合法值，不得因 `strings.TrimSpace` 或非空判断被丢弃。

建议使用私有有序切片配合名称索引实现，不使用无序 map 直接生成 Header。`cloneOptions` 必须深拷贝 Cookie、名称索引和延迟错误。

### 5.3 延迟错误

由于 `SetCookies` 不返回 error，Options 保存首个 Cookie 校验错误：

- 错误在 Options 生命周期内保持，不被后续合法调用清除。
- 记录错误后，后续 `SetCookies` 不再修改 Cookie 层。
- `Client.Request` 在基本参数检查和 Options 克隆之后、URL 改写、加密及网络调用之前返回该错误。
- 错误只能包含 Cookie Name、该次 `SetCookies` 调用内的输入索引和 `http.Cookie.Valid()` 的错误原因。
- 错误、日志和测试失败信息都不得包含 Cookie Value。

调用方若要从设置错误中恢复，应创建新的 `Options`，而不是复用已记录错误的对象。

### 5.4 读取和隔离

- `GetCookie(name)` 按区分大小写的名称读取最终 Options 值。
- 返回值必须是副本；调用方修改返回指针不能改变 Options。
- `SetCookies` 必须复制输入；调用方在 setter 返回后修改原始指针不能改变请求。
- `SetHeader` 和 `SetHeaders` 继续忽略任何大小写形式的 `Cookie` Header，Cookie 只能经 `SetCookies` 输入。

## 6. requestCookiePolicy 契约

### 6.1 构造输入

policy 在最终首跳 URL 已确定后构造，输入为：

- 克隆后的最终首跳 `*url.URL`。
- 已由 Options 规范化的请求级 Cookie。
- `cookieJar.Cookies(finalURL)` 返回的首跳 Jar 快照。

policy 构造函数不再接收原始 Options Cookie，不调用 `http.Cookie.Valid()`，也不执行 Options 同名去重。

policy 保存以下信息：

- 原始 Scheme、Host 和 Path。
- Options 显式层及其顺序。
- 首跳 Jar 层及其顺序。
- 当前模式的默认层。
- `protocolCookieNames` 对应的冻结值。
- 首跳是否允许协议默认值。

完成构造后交给 transport 的 policy 必须只读，能够被重试、重定向和并发安全地读取；不得依赖“第几次 RoundTrip”的可变计数判断首跳。

### 6.2 默认值 setter

policy 提供内部 setter，用于：

- 批量写入当前 `HeaderItem.Cookie`。
- 写入或覆盖一个模式默认 Cookie。
- 仅在默认层不存在时设置生成值。

默认层内部按 Name 唯一，最后一次显式设置生效。空默认值不写入；空 Options 值仍按高优先级覆盖处理。

### 6.3 单值读取

供 CSRF、Header 和加密身份使用的单值读取必须按“来源优先、别名次序”解析：

1. 按给定别名顺序查找 Options 层。
2. Options 均不存在时，按别名顺序查找首跳 Jar 层。
3. Jar 均不存在且目标允许默认值时，按别名顺序查找默认层。

Jar 同名多 Path 时使用 Jar 返回的第一项，因为标准 Jar 已按 Path 从长到短排序。存在的空 Options 值必须返回“已找到、值为空”，不能继续回退。

典型别名包括：

- `__csrf`、`__csrf_token`
- `MUSIC_U`、`MUSIC_R_U`
- `sDeviceId`、`sdeviceId`

### 6.4 最终化

policy finalize 只完成两件事：

1. 从首跳 Jar/default 快照中冻结 `protocolCookieNames`，Options 同名值保持显式最高优先级。
2. 将 policy 标记为只读；`Client.Request` 随后仅把 policy 已拥有的 Options 副本写入 Cookie Header。

Jar、default 和 frozen identity 始终留在 policy/transport 自动层，不写入原始 Header。因此 `net/http` 只复制真正的显式 Cookie，transport 不需要从 Name/Value 反推来源。

## 7. Client.Request 契约

`Client.Request` 按以下顺序处理 Cookie：

1. 校验 URL、请求值和响应值等基本参数。
2. 克隆 Options；若存在延迟 Cookie 错误，立即返回。
3. 校验加密模式并取得对应 `HeaderItem`。
4. 对 XEAPI 先完成 envelope URL 和传输 URL 改写。
5. 解析最终传输 URL，查询首跳 Jar，创建 policy。
6. 写入当前模式的配置默认 Cookie 和通用生成值。
7. 在加密模式 `switch` 中写入模式专属 Cookie，并从 policy 读取 Header、查询参数和加密身份。
8. `switch` 完成后 finalize policy，删除请求中已有的 `Cookie` Header，只写入 Options 显式 Cookie。
9. 将 finalized policy 放入请求 context，再执行 Resty 请求。

不得在 `switch` 前后通过另一条独立逻辑再次拼装 Options、Jar 或默认 Cookie。

### 7.1 通用默认值

在信任域内，所有模式先加载对应 `HeaderItem.Cookie`，并处理：

- `WNMCID`：配置值优先，否则使用进程生成值。
- `deviceId`：配置值优先，否则使用进程生成值。
- `MUSIC_A`：只有 `MUSIC_U` 和 `MUSIC_R_U` 均不存在时，才使用匿名管理器值；匿名管理器为空时回退到模式配置值。
- `x-antiCheatToken`：Cookie 优先于用户 Header，用户 Header 优先于模式默认 Header。

用户 Options、Jar 或默认层中存在空登录 Cookie 时，按正常优先级处理，不通过非空判断擅自改写其来源语义。

### 7.2 加密模式矩阵

| 模式 | URL 与 Cookie 作用域 | 模式专属 Cookie | Cookie 派生值 | 不变量 |
| --- | --- | --- | --- | --- |
| `CryptoModeAPI` | 使用调用方 URL | 无额外动态 Cookie | 通用 Header 和 Token | 不新增请求参数序列化行为 |
| `CryptoModeWEAPI` | 使用调用方 URL | `__remember_me=true`；配置缺失时生成 `_ntes_nnid`、`_ntes_nuid` | 从 `__csrf`/`__csrf_token` 生成 `csrf_token` 查询参数 | 没有 CSRF 时不注入伪值；不修改业务请求对象 |
| `CryptoModeEAPI` | 使用调用方 URL，签名路径仍取原始 URL Path | `sDeviceId` 缺失时回退到已解析的 `deviceId` | Cookie 参与相关 Header；不向业务 JSON 擅自补设备字段 | 保持现有 EAPI JSON 规范化、加密和响应模式 |
| `CryptoModeLinux` | 使用调用方 URL | 无额外动态 Cookie | 通用 Header 和 Token | 保持 LinuxAPI 加解密路径 |
| `CryptoModeXEAPI` | 先改写到最终 `/xeapi/` URL，再查询 Jar | `sDeviceId` 缺失时回退到已解析的 `deviceId` | `appver`、`buildver`、`channel`、`deviceId`、`sDeviceId`、`mobilename`、`os`、`osver`、登录 Token 同时供 `x-*` Header 和 XEAPI envelope 使用 | Header、Cookie、envelope 和首跳 URL 必须来自同一逻辑快照 |

所有模式都必须在 `switch` 后通过同一条 finalize 路径冻结身份并写入显式 Cookie Header。

## 8. cookieTransport 契约

### 8.1 所有权

- `Client.GetClient().Jar` 保持 `nil`。
- 持久化 Jar 只由 `cookieTransport` 查询和回写。
- `Client.SetTransport` 只替换 Cookie wrapper 下层的 `http.RoundTripper`。
- 直接替换 `GetClient().Transport` 仍属于绕过受支持所有权模型的用法。

### 8.2 每次 RoundTrip 的合并

每次 `RoundTrip`，包括首跳、Resty 重试和每次 `net/http` 重定向，都执行：

1. 进入现有 in-flight 生命周期保护。
2. 克隆 `http.Request`、URL 和 Header，不修改调用方对象。
3. 以 `req.URL` 查询当前 Jar，不使用 `req.Host`。
4. 读取 context 中的 finalized policy。
5. policy 存在时，把当前 Header Cookie 全部视为经 `net/http`/`CheckRedirect` 处理后的显式层，再与当前 Jar、可用 default 和原 URL frozen identity 合并。
6. policy 不存在时，将请求 Header Cookie 作为显式层，按“Jar 基础、显式覆盖”合并，保持 `Client.NewRequest` 和直接 `http.Client.Do` 的现有行为。
7. 删除克隆请求的旧 `Cookie` Header，按最终顺序逐项 `AddCookie`。
8. 调用可替换的下层 transport。

首跳也必须重新查询 Jar。若 Request 组装结束后、下层 transport 调用前 Jar 被并发更新：

- 普通 Jar Cookie 使用发送时的新值。
- 原 URL 上的协议身份仍使用 policy 冻结值。
- Options 和默认值优先级不变。

### 8.3 重试

Resty 重试原 URL 时：

- 重新查询 Jar，使上一次 HTTP 响应写入的普通 Cookie 立即生效。
- 保持已参与本次协议计算的冻结身份。
- Options 继续作为最高优先级覆盖层。
- 不重复追加同名 Options Cookie。

### 8.4 重定向

重定向时必须沿用 `net/http` 的 Header 传播决定：

- `net/http` 因目标不可信而删除 `Cookie` Header 后，不得从 policy 重新注入原始 Options Cookie。
- 精确域或允许的子域跳转保留的 Options Cookie 继续作为显式层。
- Jar/default/frozen 从不进入原始 Header，每跳都由当前目标 URL 重新决定。
- `CheckRedirect` 明确新增或替换的 Cookie 必须作为显式层生效，包括与上一跳自动 Cookie 同 Name/Value 的等值重加。
- `CheckRedirect` 删除显式 Cookie 后，当前目标 Jar 仍像标准 `http.Client` 一样可以提供同名 Cookie。
- 当前目标的 Jar Cookie 无论原始 Options Cookie 是否传播都要正常发送。
- 协议默认值仅在原始请求允许默认值且当前目标仍在信任域时补入。

重定向到不同 Scheme、Host 或 Path 后，不复用首跳冻结值；重定向回与原始 Scheme、Host、Path 相同的 URL 时，仍需保持本次逻辑请求的协议身份一致。Query 差异不影响该匹配。

### 8.5 响应回写

下层 transport 返回后：

- `err != nil`：原样返回响应和错误，不写入响应 Cookie，即使响应非 nil。
- `err == nil && resp == nil`：返回明确的 nil response 错误，不写入 Jar。
- `err == nil && resp != nil`：无论状态码是 2xx、3xx、4xx 还是 5xx，都提取 `resp.Cookies()` 并调用 `Jar.SetCookies(currentURL, cookies)`。

回写发生在 `RoundTrip` 返回响应之前，因此重定向检查、下一跳和下一次重试都能看到本次更新。Cookie 是否接受、删除或更新由 Jar 按标准规则决定。

### 8.6 生命周期

保留现有 transport 生命周期：

- 新请求在 closing 后返回 `ErrClientClosed`。
- `Close` 等待已进入的 `RoundTrip` 完成 Cookie 回写。
- drain 完成后调用下层 transport 的 `CloseIdleConnections`，不在关闭期间修改 Resty 请求配置。
- 下层 transport 替换受互斥锁保护。
- 不扩大 `Close` 对完整 `Request`、Upload 或 Download 生命周期的承诺。

## 9. 错误与边界条件

| 场景 | 预期行为 |
| --- | --- |
| Options Cookie 非法 | Request 在 transport 前失败；错误不含 Value；不发送请求 |
| Options 同名多次设置 | 最后一次设置覆盖；只发送一个 Options 值 |
| Options 显式空值 | 覆盖所有低优先级同名值，不回退 |
| Jar 同名不同 Path | 没有 Options 覆盖时全部保留，最具体 Path 用于协议单值 |
| 首跳前 Jar 并发更新 | 普通 Cookie 使用发送时值；协议身份使用 Request 快照 |
| 中间响应删除 Cookie | Jar 接受删除；下一跳不得继续发送旧 Jar Header 值 |
| 中间响应更新普通 Cookie | 下一跳或重试使用新值 |
| 中间响应更新协议身份 | Jar 写入新值；本逻辑请求在原 URL 仍使用冻结值 |
| 跨域重定向 | 原 Options Cookie 按 `net/http` 规则移除；目标 Jar 独立发送 |
| `CheckRedirect` 等值重加 | 作为显式 Cookie 覆盖目标 Jar，不得被误判为上一跳自动值 |
| `CheckRedirect` 删除显式 Cookie | 不恢复原 Options；目标 Jar 仍可按标准行为提供同名值 |
| 自定义 Host | 线上 Host 改变，Jar 查询和回写 URL 不变 |
| 4xx/5xx 携带 Set-Cookie | 正常写入 Jar，然后由上层处理状态错误 |
| response 与 error 同时返回 | 不写入 Jar，返回下层错误 |
| nil response、nil error | 返回明确错误，不写入 Jar |
| policy 缺失 | 维持显式 Header Cookie 覆盖当前 URL Jar 的通用 transport 行为 |

## 10. 接口兼容与迁移

### 10.1 源码兼容性变化

`Options.Cookies` 私有化是有意的源码兼容性变化。以下代码不再支持：

```go
opts.Cookies = append(opts.Cookies, cookie)
opts := &api.Options{Cookies: cookies}
```

统一迁移为：

```go
opts := api.NewOptions()
opts.SetCookies(cookies...)
```

仓库内测试和调用点必须全部迁移，不能为了兼容重新暴露可变切片。

### 10.2 保持不变的接口

- `Options.SetCookies` 仍无返回值。
- `Options.GetCookie` 仍返回 `*http.Cookie`，但返回对象改为副本。
- `Client.GetCookies` 和 `Client.SetCookies` 继续用于持久化 Jar，不受 Options 私有化影响。
- `Client.GetClient`、`Client.NewRequest`、`Client.SetTransport` 和 `Client.Close` 的签名不变。

## 11. 实施顺序

本次按以下顺序实施：

1. **Options 边界**：私有化 Cookie 存储，实现原子校验、同名覆盖、延迟错误、深拷贝和只读 getter，迁移直接字段访问测试。
2. **Policy 简化**：调整构造输入和内部数据模型，移除 Options 校验/去重，加入默认 setter、分层单值读取和 frozen identity。
3. **Request 重排**：在最终 URL 后建立 policy，在模式分支设置动态 Cookie，switch 后只把显式 Options 写入 Header，policy 放入 context。
4. **Transport 重排**：每次发送重查 Jar，直接把当前 Header 当作显式层，合并 default/frozen 并保持重试、重定向、回写和生命周期行为。
5. **回归测试**：迁移现有测试，增加首跳并发更新、Options 边界、模式矩阵和逐跳行为覆盖。
6. **开发文档同步**：更新 API 与 Cookie 持久化开发参考，只记录已经实现并验证的行为。

每一步都必须在当前脏工作树上保留无关的已暂存、未暂存和未跟踪内容，不得重置或覆盖其他修改。

## 12. 验收矩阵

### 12.1 Options

- 零值 Options 可安全调用 setter。
- `nil` Cookie 被忽略。
- 同一调用和多次调用的同名值均为最后一次设置生效。
- Cookie Name 大小写不同视为不同名称。
- 空值覆盖 Jar/default。
- 非法批次原子失败，后续 Request 不调用 transport。
- 错误不泄漏 Cookie Value。
- 修改 setter 输入或 getter 返回值不影响 Options。
- `cloneOptions` 不共享 Header、Cookie 或索引存储。

### 12.2 Request 与模式

- 五种 CryptoMode 均通过 switch 后的统一路径冻结身份并写入显式 Cookie。
- WEAPI 正确处理 CSRF、访客 Cookie、登录 Token 和匿名回退。
- EAPI/XEAPI 的 `deviceId`、`sDeviceId` 和相关 Header 遵循同一 policy 快照。
- XEAPI 使用改写后的 `/xeapi/` URL 查询 Jar。
- 非信任域不注入设备、匿名或其他协议默认值。
- 自定义 Host 不改变 Cookie URL。
- Request 不修改调用方 Options 或业务 payload。

### 12.3 Transport

- 首跳发送前更新普通 Jar Cookie 时使用新值，冻结协议身份不变。
- Resty 重试能看到上一响应写入的普通 Cookie。
- 同域重定向按目标 Path 重查 Jar。
- 跨域重定向不泄漏 Options Cookie，但发送目标 Jar Cookie。
- `CheckRedirect` 的 Cookie 新增、替换及等值重加被保留，删除后按标准 Jar 行为补值。
- Jar 中合法同名多 Path Cookie 保持完整。
- 请求对象和原 Header 不被原地修改。
- 自定义下层 transport 仍经过 Cookie wrapper。

### 12.4 响应与关闭

- 2xx、3xx、4xx、5xx 响应都回写 Cookie。
- 重定向响应在检查下一跳前完成回写。
- transport error、response+error 和 nil response 不回写。
- `Close` 等待 in-flight 回写，drain 后关闭 idle connections，取消的 Close 调用不终止后台共享关闭。
- race 检测覆盖 Jar 更新、transport 替换和关闭并发。

## 13. 验证命令

实现阶段至少运行：

```bash
go test ./api -count=1 -run 'Test(CookieTransport|RequestCookiePolicy|Options)'
go test ./api ./pkg/cookie
go test -race ./api ./pkg/cookie
go test ./...
make lint
git diff --check
```

测试必须使用 fake `RoundTripper`、标准内存 Jar、临时目录或本地 `httptest`，不得读取用户真实 Cookie 文件。

不得设置 `NCMCTL_RUN_LIVE_TESTS`，不得运行 `make test-live` 或任何可能访问真实账号、签到、领取、上传、下载或修改远端状态的测试。普通全量测试若因当前工作树中无关修改失败，必须区分本次回归和既有失败。

## 14. 完成标准

只有同时满足以下条件，才能认为后续实现完成：

- Options、Request/policy 和 transport 的职责与本文一致。
- 成功请求的 Cookie、协议 Header、加密身份、重试和重定向行为满足兼容契约。
- 没有同名 Options/Jar/default 重复写入。
- 所有新增行为都有离线回归测试。
- 目标测试、race、lint 和 diff 检查通过，或已明确记录与本次无关的既有失败。
- 开发文档与实际实现一致，不把未验证的线上兼容性写成事实。
