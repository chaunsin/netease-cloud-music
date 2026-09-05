# ncmctl 使用指南

本文介绍 `ncmctl` 的常用命令和安全边界。安装方式见[项目首页](../README.md#安装)。命令参数可能随版本变化，遇到差异时以本机的 `ncmctl <command> --help` 为准。

## 目录

- [开始之前](#开始之前)
- [命令速查](#命令速查)
- [登录与退出](#登录与退出)
- [每日任务](#每日任务)
- [音乐下载](#音乐下载)
- [云盘上传](#云盘上传)
- [NCM 文件解密](#ncm-文件解密)
- [HTTP(S) 监控代理](#https-监控代理)
- [调试和辅助命令](#调试和辅助命令)
- [常见问题](#常见问题)

## 开始之前

- `sign`、`partner`、`scrobble`、`share`、`fansgroup`、`cloud` 和部分 `curl` 调用会修改账号数据，不要把它们当作连通性测试。
- `scrobble` 存在较高的账号风控风险；`sign --automatic` 也会执行额外的奖励领取操作；`fansgroup` 会修改播放记录和点赞、切换歌曲红心状态，并发布公开动态。
- 当前网易 API 和 CookieCloud HTTP 客户端未校验服务端 TLS 证书，只应在可信网络中使用。
- 全局 `--debug` 会记录 API 请求和响应，其中可能包含 Cookie、Token、设备标识等敏感数据。调试日志和重定向输出应按凭据文件保护。



## 命令速查


| 命令                                   | 需要登录  | 用途                             |
| ------------------------------------ | ----- | ------------------------------ |
| `ncmctl login <method>`              | 否     | 通过手机、Cookie、CookieCloud 或二维码登录 |
| `ncmctl logout`                      | 已有会话  | 远端退出并删除默认 Cookie 和 XEAPI 会话状态  |
| `ncmctl task [flags]`                | 是     | 按 cron 长期调度账号任务                |
| `ncmctl share [flags]`    | 是     | 发布每日推歌公开动态，可查询状态或抽奖    |
| `ncmctl fansgroup [status]`          | 是     | 执行乐迷团每日任务，可只读查询状态          |
| `ncmctl sign [flags]`                | 是     | 立即执行一次云贝签到和黑胶乐签                |
| `ncmctl partner [flags]`             | 是     | 立即上报音乐合伙人测评                    |
| `ncmctl scrobble [flags]`            | 是     | 提交播放日志并在本地去重                   |
| `ncmctl download <id-or-url>...`     | 是     | 下载歌曲、专辑、歌手或歌单                  |
| `ncmctl cloud <path>`                | 是     | 上传音乐文件或扫描目录                    |
| `ncmctl ncm <input>...`              | 否     | 解密 `.ncm` 文件或目录                |
| `ncmctl crypto <encrypt-or-decrypt>` | 否     | 调试 API 加解密格式                   |
| `ncmctl curl [method]`               | 取决于接口 | 调用导出的 Go API wrapper           |
| `ncmctl proxy [flags]`               | 否     | 启动 HTTP(S) 监控代理                |
| `ncmctl update [flags]`              | 否     | 从 GitHub Releases 升级并替换本地可执行文件  |
| `ncmctl completion <shell>`          | 否     | 生成 shell 补全脚本                  |




## 登录与退出

`ncmctl` 支持二维码、Cookie、CookieCloud、短信和手机号密码五种登录流程。优先使用二维码或受保护的 Cookie 文件；密码和直接传入的 Cookie 可能出现在 shell 历史或进程参数中。

### 二维码登录

```bash
ncmctl login qrcode
```

命令会在当前目录生成 `qrcode.png`，同时在终端打印二维码。使用网易云音乐 App 扫码并确认后，登录信息会写入本地，二维码图片会被删除。登录失败、取消或超时后，图片可能保留，需要手动清理。

```bash
# 设置超时时间和二维码目录
ncmctl login qrcode --timeout 5m --dir ./private-qr
```


| 参数              | 默认值  | 说明                             |
| --------------- | ---- | ------------------------------ |
| `-t, --timeout` | `5m` | 等待扫码确认的最长时间                    |
| `-d, --dir`     | 当前目录 | `qrcode.png` 的输出目录             |
| `-l, --level`   | `1`  | 二维码容错等级：0=7%、1=15%、2=25%、3=30% |


状态码 `800` 表示二维码过期或取消，`801` 表示等待扫码，`802` 表示已扫码等待确认，`803` 表示登录成功。二维码过期后重新运行命令即可。

### Cookie 登录

从已登录的浏览器导出 Cookie，内容必须包含 `MUSIC_U`。推荐使用权限为 `0600` 的文件：

```bash
chmod 600 cookie.txt
ncmctl login cookie --file cookie.txt
```

也可以直接传入 Cookie 字符串，但通常会留在 shell 历史中：

```bash
ncmctl login cookie 'MUSIC_U=<浏览器导出的值>; __csrf=<浏览器导出的值>'
```

支持 `header`、`json` 和 `netscape` 三种格式。不指定 `--format` 时会自动识别：

```bash
ncmctl login cookie --format json --file cookies.json
ncmctl login cookie --format netscape --file cookies.txt
```

可以使用 [Cookie Editor](https://chromewebstore.google.com/detail/cookie-editor/ookdjilphngeeeghgngjabigmpepanpl) 等浏览器扩展导出 Cookie。不要将导出的内容粘贴到 Issue、日志、截图或聊天记录中。

### CookieCloud 登录

[CookieCloud](https://github.com/easychen/CookieCloud/blob/master/README_cn.md) 可将浏览器 Cookie 加密同步到自建服务。先在网页端登录网易云音乐并执行一次手动同步，再运行：

**操作流程：**

1. 📥 安装 CookieCloud 浏览器插件
2. ⚙️ 完成插件配置
3. 🎵 在网页端登录网易云音乐
4. 🔄 点击【手动同步】按钮同步到云端
5. 🖥️ 执行登录命令

```bash
ncmctl login cookiecloud \
  --uuid '<uuid>' \
  --password '<密码>' \
  --server 'http://127.0.0.1:8088'
```


| 参数               | 默认值                     | 说明                    |
| ---------------- | ----------------------- | --------------------- |
| `-u, --uuid`     | 必填                      | CookieCloud UUID      |
| `-p, --password` | 必填                      | CookieCloud 端到端密码     |
| `-s, --server`   | `http://127.0.0.1:8088` | 服务端地址                 |
| `-t, --timeout`  | `30s`                   | 请求超时时间                |
| `-H, --headers`  | 空                       | 逗号分隔的 `key=value` 请求头 |


UUID 和密码只能通过命令行参数传入，可能出现在 shell 历史和进程列表中。CookieCloud 服务承载加密后的 Cookie 数据，应按凭据系统管理；使用第三方服务前应确认其可信度。

### 手机登录

不传密码时使用短信验证码：

```bash
ncmctl login phone 18800008888
```

收到提示后输入短信验证码。短信发送次数有限，反复尝试可能触发风控。`--timeout` 只限制网络请求，不能中断终端中正在等待的验证码输入。

已在网易云音乐中开启手机号密码登录时，也可以运行：

```bash
ncmctl login phone 18800008888 --password '<密码>'
```

密码参数可能出现在 shell 历史和进程列表中，并且该方式可能返回 `8821` 行为验证错误，只建议作为备选方案。

### 退出登录

```bash
ncmctl logout
```

远端退出成功后，命令会删除 `<home>/.ncmctl/cookie.json` 和 `<home>/.ncmctl/xeapi.yaml`。匿名 token 默认保留，需要一并删除时运行：

```bash
ncmctl logout --clear-anonymous-token
```

`<home>` 由全局 `--home` 指定，默认是当前操作系统用户目录。通过自定义配置指定的 Cookie 文件不会自动删除。

## 每日任务

`task` 是长期运行的调度服务；`sign`、`partner`、`scrobble`、`share` 和 `fansgroup` 则执行一次后退出。


| 命令         | 作用              | 默认调度时间 |
| ---------- | --------------- | ------ |
| `sign`     | 云贝签到和黑胶乐签       | 10:00  |
| `partner`  | 音乐合伙人测评         | 18:00  |
| `scrobble` | 上报播放日志，最多 300 首 | 18:00  |
| `share` | 发布每日推歌公开动态并按服务端机会抽奖 | 09:00  |
| `fansgroup` | 执行乐迷团每日任务 | 10:30  |


指定 `--runAll` 注册全部五项任务；不带任何任务开关时 `task` 会直接报错退出。启动后持续运行到收到 `Ctrl+C`：

```bash
ncmctl task --runAll
```

也可以只调度需要的任务：

```bash
# 只调度签到
ncmctl task --sign

# 调度签到和播放日志上报
ncmctl task --sign --scrobble

# 每天 20:00 执行播放日志上报
ncmctl task --scrobble \
  --scrobble.cron '0 20 * * *' \
  --location Asia/Shanghai
```

调度表达式采用五段式 cron。可以使用 [crontab.guru](https://crontab.guru/) 检查表达式。

立即执行一次任务：

```bash
ncmctl sign
ncmctl partner
ncmctl scrobble --num 200
ncmctl fansgroup status
ncmctl fansgroup
```

`sign --automatic` 会在签到后领取可用的云贝和符合条件的 VIP 奖励，因此会执行更多账号操作。`partner` 会在每个测评项目之间等待 15 至 24 秒。`scrobble` 风控风险较高，且可用歌曲不足或本地去重命中时，实际完成数可能少于请求数。

### 每日推歌挑战

发布命令会修改账号动态，笔记默认公开并默认抽奖，默认不会删除动态。先查看活动状态：

```bash
ncmctl share status
ncmctl share draw --count 1
```

发布每日推荐歌曲：

```bash
ncmctl share
ncmctl share --song-id 1820944399 --draw=false
```

`--image` 可指定本地非空图片；否则命令下载歌曲封面。`--dry-run` 只读取状态并准备歌曲和文案，不报名、不上传、不发布。`--delete` 只删除本次发布的动态，且必须与抽奖一起使用；删除可能影响全勤奖励资格。活动进度以服务端状态为准，发布后的后续失败不会自动重发。

### 乐迷团任务

`fansgroup` 一次性命令会查询并执行乐迷团的每日任务：播放歌曲、上报歌曲分享、点赞乐迷笔记、发布图文笔记，以及通过红心操作完成今日加速任务。它会按未完成任务修改播放记录、点赞和歌曲红心状态，并可能发布公开动态。发布的笔记默认保留，自动化行为存在账号风控风险，使用频率请自行决定。

先只读查看任务状态：

```bash
ncmctl fansgroup status
```

`status` 仍然需要登录，可用 `--group-id` 查询指定乐迷团；它不能与 `--title`、`--message`、`--image` 或 `--delete` 一起使用。

执行乐迷团任务。命令会串行处理乐迷团和任务，并在操作之间随机等待：任务间 2~5 秒、点赞间 1~3 秒、多乐迷团之间 3~10 秒、删除动态前 5~30 秒。

```bash
# 默认乐迷团
ncmctl fansgroup

# 指定乐迷团 ID（可逗号分隔或重复传参）
ncmctl fansgroup --group-id 1872529203038486609,1872529203038486610

# 自定义笔记文案、图片，并在任务完成后删除本次发布的笔记
ncmctl fansgroup --group-id 1872529203038486609 \
  --title '我的打卡标题' \
  --message '这是一段至少十个字符的打卡正文' \
  --image ./fansgroup-cover.jpg \
  --delete

# 等价的短参数写法（-g/-t/-m/-i/-d）
ncmctl fansgroup -g 1872529203038486609 \
  -t '我的打卡标题' \
  -m '这是一段至少十个字符的打卡正文' \
  -i ./fansgroup-cover.jpg \
  -d
```

说明：

- `--group-id` 默认使用 `1872529203038486609`，每个值必须为纯数字；可以用逗号分隔，也可以重复传参。
- 短参数取 flag 名称首字母：`-g`/`-t`/`-m`/`-i`/`-d` 分别等价于 `--group-id`/`--title`/`--message`/`--image`/`--delete`。`task --fansgroup.*` 调度参数仍只有长参数形式。
- `--title` 和 `--message` 缺省时使用内置文案；`--message` 覆盖时 TrimSpace 后至少 10 个字符。
- `--image` 需为本地非空常规文件；缺省时下载乐迷团头像作为笔记图片。
- `--delete` 只删除本次执行链内发布成功的动态，未发布时不删除；删除失败会逐行输出但不影响退出码。
- 命令不会自动加入乐迷团；未加入的乐迷团会被跳过。
- 今日加速任务会按“取消红心、添加红心、再次取消红心”的顺序操作歌曲。最后一次取消失败时，命令会提示手动清理可能残留的红心。
- 任务进度以服务端返回为准；已完成、未知类型或无可点赞帖子会被跳过。读取乐迷团失败，或某团执行过任务且全部失败时，命令会在处理完其他乐迷团后退出非零。

调度到 `task` 服务：

```bash
# 每天 10:30 执行乐迷团任务
ncmctl task --fansgroup

# 自定义调度与笔记选项
ncmctl task --fansgroup \
  --fansgroup.cron '30 10 * * *' \
  --fansgroup.group-id 1872529203038486609 \
  --fansgroup.delete
```

`task --runAll` 会注册包括 `fansgroup` 在内的全部五项任务。

## 音乐下载

`download` 接受歌曲 ID，以及歌曲、专辑、歌手或歌单链接。命令需要登录，文件通过服务端 MD5 校验后才会写入目标目录。

```bash
# 通过歌曲 ID 下载
ncmctl download --level hires 1820944399

# 下载无损歌曲到指定目录
ncmctl download \
  --level lossless \
  --output ./music \
  'https://music.163.com/song?id=1820944399'

# 下载专辑，并发数设为 5
ncmctl download --parallel 5 \
  'https://music.163.com/#/album?id=34608111'

# 严格按指定品质下载歌手歌曲，缺少该品质时跳过
ncmctl download --strict \
  'https://music.163.com/#/artist?id=33400892'

# 下载歌单
ncmctl download 'https://music.163.com/playlist?id=593617579'
```


| 参数               | 默认值          | 说明                |
| ---------------- | ------------ | ----------------- |
| `-o, --output`   | `./download` | 输出目录              |
| `-p, --parallel` | `5`          | 并发下载数，范围 1-20     |
| `-l, --level`    | `lossless`   | 请求的音乐品质           |
| `--strict`       | `false`      | 没有指定品质时跳过，不自动降级   |
| `--tag`          | `true`       | 兼容参数，当前不会写入下载文件标签 |


品质参数：


| 品质     | 可用值                 | 说明      |
| ------ | ------------------- | ------- |
| 标准     | `standard`、`128`    | 128kbps |
| 较高     | `higher`、`192`      | 192kbps |
| 极高     | `exhigh`、`HQ`、`320` | 320kbps |
| 无损     | `lossless`、`SQ`     | FLAC    |
| Hi-Res | `hires`、`HR`        | 高解析度    |




## 云盘上传

`cloud` 接受一个本地音乐文件或一个目录。目录会递归扫描，深度不能超过 3 层；单个文件不能超过 500 MB。

```bash
# 上传单个文件
ncmctl cloud '/path/to/music.mp3'

# 扫描目录并上传
ncmctl cloud '/path/to/music/'

# 只上传不小于 1 MB 的 FLAC 文件
ncmctl cloud \
  --parallel 5 \
  --minsize 1MB \
  --regexp '.*\.flac$' \
  '/path/to/music/'
```


| 参数               | 默认值 | 说明             |
| ---------------- | --- | -------------- |
| `-p, --parallel` | `3` | 并发上传数，范围 1-10  |
| `-m, --minsize`  | 空   | 跳过小于指定大小的文件    |
| `-r, --regexp`   | 空   | 用于筛选文件路径的正则表达式 |


上传会修改账号云盘，不能用于测试登录状态或网络连通性。

## NCM 文件解密

`ncm` 在本地将 `.ncm` 文件解密为 `.mp3` 或 `.flac`，不需要登录。所有位置参数都会作为输入路径，输出目录必须通过 `-o` 或 `--output` 指定。

```bash
# 解密单个文件
ncmctl ncm '/path/to/file.ncm' --output ./decoded

# 批量扫描目录
ncmctl ncm '/path/to/ncm/files' \
  --output ./decoded \
  --parallel 10

# 多个输入写入当前目录
ncmctl ncm first.ncm second.ncm --output .
```


| 参数               | 默认值     | 说明               |
| ---------------- | ------- | ---------------- |
| `-o, --output`   | `./ncm` | 输出目录             |
| `-p, --parallel` | `10`    | 并发解密数，范围 1-50    |
| `--tag`          | `false` | 历史反向参数：设置后关闭标签写入 |


音频标签默认写入。不存在的路径或显式传入的非 `.ncm` 文件会在创建输出目录前报错，目录扫描深度不能超过 3 层。

## HTTP(S) 监控代理

`proxy` 用于监控网易云音乐客户端 HTTP(S) 流量，可用于帮助开发接口使用。它只记录网易相关域名，其他流量会继续转发但不会输出。捕获内容默认对 Cookie、Token、手机号、邮箱、设备标识和密码等字段脱敏。

### 启动代理

```bash
# 只监听本机 127.0.0.1:9000
ncmctl proxy

# 将捕获内容保存到文件；启动信息和错误仍写入终端
ncmctl proxy > capture.log

# 允许局域网设备连接，仅限可信网络
ncmctl proxy --listen 0.0.0.0:9000

# 使用已有 CA，证书和私钥必须同时提供
ncmctl proxy --ca-cert ./ca.crt --ca-key ./ca.key

# 改变运行数据和自动生成 CA 的根目录
ncmctl --home /srv/ncmctl proxy
```

如果没有提供证书私钥内容，则首次启动会生成一组用户专属 CA：

- 证书：`<home>/.ncmctl/proxy/ca.crt`
- 私钥：`<home>/.ncmctl/proxy/ca.key`

启动信息会在 stderr 中显示证书路径和 SHA-256 指纹。将 `ca.crt` 安装到需要监控的客户端并设为受信任证书，再将其 HTTP/HTTPS 代理指向监听地址。`ncmctl` 不会自动修改系统信任库。不要复制或泄露 `ca.key`。

### 常用参数


| 参数                    | 默认值              | 说明                                    |
| --------------------- | ---------------- | ------------------------------------- |
| `--listen`            | `127.0.0.1:9000` | 监听地址                                  |
| `--ca-cert`           | 自动生成             | 已有 CA 证书，必须与 `--ca-key` 同时使用          |
| `--ca-key`            | 自动生成             | 已有 CA 私钥，必须与 `--ca-cert` 同时使用         |
| `--max-body`          | `1MB`            | 每个请求或响应最多显示的正文大小，不影响转发                |
| `--show-sensitive`    | `false`          | 关闭脱敏并显示敏感字段                           |
| `--xeapi-state-file`  | 空                | 显式读取 `xeapi.yaml` 作为会话种子              |
| `--xeapi-session-id`  | 空                | XEAPI session ID，必须与 session key 同时提供 |
| `--xeapi-session-key` | 空                | 16、24 或 32 字节的原始 ASCII key            |


如需被动解密 XEAPI 请求，可以显式加载已有状态：

```bash
ncmctl proxy --xeapi-state-file ~/.ncmctl/xeapi.yaml
```

也可以直接传入一组 session。key 按原始 ASCII 使用，不解析十六进制；命令行参数可能被 shell 历史和进程列表记录。

```bash
ncmctl proxy \
  --xeapi-session-id SESSION_ID \
  --xeapi-session-key '0123456789abcdef'
```

代理不会因为设置了 `--home` 就自动读取 `<home>/.ncmctl/xeapi.yaml`。状态文件种子和命令行种子只用于当前进程，运行期间也会从完整有效的响应头学习 session，退出时不会写回状态文件。

### 捕获边界

- HTTPS MITM 内层支持 HTTP/2，客户端到代理、代理到上游会分别通过 ALPN 协商 `h2` 或 `http/1.1`。外层代理监听仍是 HTTP/1.1 和 CONNECT，不支持 h2c CONNECT。
- IP 地址形式的 CONNECT 只有在 TLS ClientHello 的 SNI 命中网易域名时才会进入 MITM，并继续向原 IP 转发。无 SNI、ECH、证书固定、Android 用户 CA 限制、QUIC/HTTP3 或绕过系统代理都可能导致无法捕获。
- WEAPI 请求使用随机密钥，被动代理无法恢复。XEAPI 只有在 session ID 命中已知 key 时才能解密请求正文；协议细节见 [XEAPI 研究记录](xeapi.md)。
- 音视频、图片、multipart、未知长度请求和无法安全结构化脱敏的正文只输出摘要，当前不解析 WebSocket 帧。
- `--listen 0.0.0.0:9000` 会开放一个无认证代理，只能在可信网络和防火墙保护下临时使用。
- stdout 阻塞时，代理会优先保证真实流量转发。`CAPTURE_DROPPED` 表示部分捕获块被丢弃，`CAPTURE_OUTPUT_ERROR` 表示输出发生错误，后续记录可能不完整。

使用 `ncmctl --debug proxy` 可以查看 `TLS_DIAGNOSTIC` 连接决策。按 `Ctrl+C` 或发送 `SIGTERM` 可停止代理。

## 调试和辅助命令



### API 加解密

`crypto` 只处理本地输入，不用于绕过认证。常用示例：

```bash
# 加密 WEAPI、EAPI 或 Linux API 参数
ncmctl crypto encrypt --kind weapi '{"key":"value"}'
ncmctl crypto encrypt --kind eapi \
  --url /eapi/v3/song/detail '{"c":[]}'
ncmctl crypto encrypt --kind linux '{"method":"POST"}'

# 解密 EAPI 请求或 XEAPI 响应
ncmctl crypto decrypt --kind eapi --encode hex 'CIPHERTEXT'
ncmctl crypto decrypt --kind xeapi --encode hex 'CIPHERTEXT'

# 从 HAR 中选择 XEAPI 条目
ncmctl crypto decrypt --url '/xeapi/*' capture.har
```

XEAPI 请求 `B` 需要显式提供 dynamic/session key。缺少 key 或请求字段不完整时，命令会写出 `partial` 结果并以非零状态退出。HAR、密钥和解密输出可能包含账号凭据和个人数据，应在受控环境中处理。

### 调用 API wrapper

`ncmctl curl` 不是系统的 `curl`。它按导出的 Go 方法名调用指定 API wrapper：

```bash
ncmctl curl --kind weapi --data '{}' GetUserInfo
```

接口是否需要登录、是否修改账号取决于所选方法。调用不熟悉的方法前应先检查对应 Go 实现。

### Shell 补全

```bash
ncmctl completion bash
ncmctl completion fish
ncmctl completion powershell
ncmctl completion zsh
```

命令会将脚本写到标准输出。具体安装位置见 `ncmctl completion <shell> --help`。

## 常见问题



### 为什么实际下载品质低于指定品质？

未启用 `--strict` 时，如果歌曲没有指定品质，命令会向下选择最接近的可用品质。需要严格匹配时加上 `--strict`；对应品质不存在时，该歌曲会被跳过。

### `scrobble` 为什么没有完成 300 首？

命令会在 `<home>/.ncmctl/database/badger/` 保存本地去重和当日计数。榜单中可用歌曲不足，或歌曲已存在于本地记录时，实际完成数会少于请求数。删除数据库只会丢失本地历史，不会重置网易服务端的计数。

### 如何确认参数是否适用于当前版本？

```bash
ncmctl --version
ncmctl <command> --help
```

项目文档对应仓库当前版本；旧版二进制的参数和默认值可能不同。
