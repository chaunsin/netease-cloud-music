# Cookie 请求组装职责重构实施记录

> 状态：已实施
>
> 更新日期：2026-08-16
>
> 事实来源：[Cookie 请求组装职责重构 SPEC](../spec/cookie-request-assembly-refactor.md)

本文只记录实施范围和顺序。Cookie 优先级、重试、重定向、回写、关闭和验收契约统一由配套 SPEC 维护，不在已完成的计划中重复一份。

## 修改范围

- `api/options.go`：请求级 Cookie 的私有存储、校验、覆盖、错误和复制。
- `api/api.go`：最终 URL 解析、模式默认值、协议身份快照和显式 Cookie Header。
- `api/cookie_transport.go`：每次发送的 Jar/default/frozen 合并、重试、重定向、回写和关闭。
- 相关测试和 `.claude/skills/ncmctl-dev/references/`：回归覆盖与开发约束同步。

`pkg/cookie` 持久化格式、API 加解密算法、Resty 重试策略和 `net/http` 重定向策略不在修改范围内。

## 实施顺序

1. 私有化 `Options.Cookies`，将校验、同名覆盖和副本隔离收口到 `Options`。
2. 调整 `requestCookiePolicy`，只保留分层单值解析、模式默认值和 frozen identity。
3. 调整 `Client.Request`，在 XEAPI URL 改写后建立快照，并且只把 Options 显式 Cookie 写入原始 Header。
4. 调整 `cookieTransport`，每次 `RoundTrip` 按当前 URL 重查 Jar，合并 default/frozen，再即时回写有效 HTTP 响应的 Cookie。
5. 关闭时先拒绝新 `RoundTrip`，等待 in-flight 回写，再关闭 idle connections 并持久化状态。
6. 迁移 fake transport 注入、隔离 live 测试状态，补齐 Cookie 重定向、重试、并发替换和关闭回归。

## 验证边界

- 先运行 `api` 及 Cookie 相关的离线定向测试，再扩大到仓库测试、race、lint 和 diff 检查。
- 测试使用 fake `RoundTripper`、内存 Jar 和临时目录；未设置 `NCMCTL_RUN_LIVE_TESTS`。
- 未运行登录、签到、领取、上传、下载或其他真实账号操作。

后续修改直接更新 SPEC 和回归测试；只有在需要重新拆分实施阶段时才更新本记录。
