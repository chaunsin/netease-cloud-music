# netease-cloud-music

[![GoDoc](https://godoc.org/github.com/chaunsin/netease-cloud-music?status.svg)](https://godoc.org/github.com/chaunsin/netease-cloud-music) [![Go Report Card](https://goreportcard.com/badge/github.com/chaunsin/netease-cloud-music)](https://goreportcard.com/report/github.com/chaunsin/netease-cloud-music) [![ci](https://github.com/chaunsin/netease-cloud-music/actions/workflows/ci.yml/badge.svg)](https://github.com/chaunsin/netease-cloud-music/actions/workflows/ci.yml) [![deploy image](https://github.com/chaunsin/netease-cloud-music/actions/workflows/deploy_image.yml/badge.svg)](https://github.com/chaunsin/netease-cloud-music/actions/workflows/deploy_image.yml)

网易云音乐 Golang API 接口 + 命令行工具套件 + 一键完成任务

## ⚠️ 声明

> **2025-06-03:** 目前风控特别严重容易封号，签到、刷歌、音乐合伙人都不建议使用，如要执意使用收到 [非法挂机行警告](https://github.com/chaunsin/netease-cloud-music/issues/34) 最好终止，否则后果自负！

**本项目仅供个人学习使用,切勿用于商业用途、非法用途使用！！！**

**使用此项目遇到封号等问题概不负责,使用前请谨慎考虑！！！**

**如有侵权即删！！！**

## ✨ 功能

### 命令行 (ncmctl)
- [x] 登录
  - [x] 短信登录
  - [x] cookie方式登录
  - [x] [cookiecloud](https://github.com/easychen/CookieCloud/blob/master/README_cn.md)方式登录
  - [x] ~~扫码登录~~
  - [x] ~~手机号密码登录~~    
- [x] 一键每日任务完成(音乐合伙人、云贝签到、vip签到、刷歌300首)
- [x] 云贝签到(自动领取签到奖励)
- [x] “音乐合伙人”自动测评(5首基础歌曲 + 2到7首随机额外歌曲测评，不包含"歌曲推荐"测评)
  2025年3月[公告](https://music.163.com/#/event?id=30336457500&uid=7872690377)、[规则](https://y.music.163.com/g/yida/9fecf6a378be49a7a109ae9befb1b8d3)
- [x] 每日刷歌300首(带去重功能)
- [x] 云盘上传(支持并行批量上传)
- [x] 解密.ncm文件为.mp3/.flac可播放歌曲(支持并行批量解析)。
- [x] 音乐下载，支持标准、高品质、极高(HQ)、无损(SQ)、Hi-Res品质下载      
- [x] `crypto`支持接口参数加解密便于调试
- [x] `curl`子命令调用网易云音乐API,无需关心出入参数加解密问题便于调试
    - [ ] 支持动态链接请求
- [x] vip每日签到
- [ ] vip日常任务完成(待考虑)
- [ ] “音乐人”任务自动完成(待考虑)
- [ ] proxy 代理

### api

- weapi 网页端、小程序使用
- eapi PC端、移动端使用

目前由于本人时间精力有限,暂未书写文档,不过可以参考`api`目录下代码,代码通俗易懂,且有注释.

**提示:**
目前主要实现了weapi也推荐使用weapi,接口相对较全，如需要其他接口可提 [issue](https://github.com/chaunsin/netease-cloud-music/issues)。

## 💻 要求

- golang >= 1.23
- makefile (可选)
- git (可选)
- docker (可选)

## ncmctl

### 🔨 安装

**可执行文件安装**

```shell
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest
```

或

```shell
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make install
```

**提示:** 默认会安装到`$GOPATH/bin`目录下

**docker版本镜像获取方式**

```shell
docker pull chaunsin/ncmctl:latest # dockerhub镜像仓库
docker pull ghcr.io/chaunsin/ncmctl:latest # github镜像仓库
```

镜像仓库以及docker使用方式: https://hub.docker.com/r/chaunsin/ncmctl

如有条件自编译镜像

```shell
git clone https://github.com/chaunsin/netease-cloud-music.git
cd netease-cloud-music && make build-iamge
```

**提示:** 自行编译需要安装docker环境,另外受国服环境影响最好开梯子。

**青龙脚本使用方式请参考:**

[>> 点我 <<](docs/qinglong.md)

### 🚀 使用

**一、登录**

目前支持5种登录方式

<details>
  <summary>点击查看详情</summary>
  <pre>

**一、短信登录**

```shell
ncmctl login phone 188xxx8888
```

执行成功并发送了短信验内容如下：

```shell
send sms success
please input sms captcha: 
```

根据上述内容提示，请在终端输入短信验证码进行登录,成功内容如下：

```shell
verify sms success
login success: &{RespCommon:{Code:200 Message: Msg: Data:<nil>} Account:0xc00036a070 Profile:0xc0005a8180}
```

**注意:**

1. 发送短信每日有限制,请不要频繁登录避免风控。
2. 有时显示**send sms success**
   但等了很久依然没有收到短信,可能是短信运营商抽风,可以重新发送短信或者稍后再试。如果尝试多次还是失败，可能账号因某些原因入了黑名单,具体验证方式可以登录网易云网页端走短信登录正规流程看是否能收到短信。

**二、手机号密码登录**

使用密码登录方式,需要在网易云中设置账号允许手机号密码登录方式,如果未设置请先设置。

```shell
ncmctl login phone 188xxx8888 -p 123456
```

密码登录方式容易出现安全风险相关问题: **8821 需要行为验证码验证** 未必会成功,可作为尝试登录的一种方式。

**注意: 不要泄露密码。**

**三、cookie登录**

当使用此工具按照正常流程登录失败、或者因风控等原因不能登录，可以尝试使用cookie登录,cookie登录属于保底方案。

cookie内容得获取方式有很多，比如可以通过浏览器安装插件的方式进行获取，可参考使用工具 [Cookie Editor](https://chromewebstore.google.com/detail/cookie-editor/ookdjilphngeeeghgngjabigmpepanpl)
或其他cookie导出工具。

```shell
# 以下二选一
# 导入cookie字符串文本内容
ncmctl login cookie 'cookie字符串内容'
# 导入cookie文件内容
ncmctl login cookie -f cookie.txt
```

cookie内容支持三种类型格式 `-f`

- header
- json
- [netscape](https://docs.cyotek.com/cyowcopy/1.10/netscapecookieformat.html)

详细使用，以及文件格式规则可查看 `ncmctl login cookie -h` 介绍

**四、cookiecloud登录**

cookiecloud
还是cookie另一种登录方式,cookiecloud也是浏览器cookie管理插件工具得一种，它得特点是浏览器可以自动同步cookie到云端，并对cookie内容进行加密存储，业务场景上可直接从云端拉取cookie内容到本地,以供后续使用。

cookiecloud详细介绍:

- https://github.com/easychen/CookieCloud/blob/master/README_cn.md 介绍
- https://juejin.cn/post/7190963442017108027 使用教程
- https://chromewebstore.google.com/detail/cookiecloud/ffjiejobkoibkjlhjnlgmcnnigeelbdl chrome插件地址

操作流程:

1. 安装cookiecloud插件
2. 配置好cookiecloud相关配置
3. 网页端保证成功登录到网易云音乐
4. 为了保证即时同步,点击【手动同步】按钮同步到服务器。
5. 执行`ncmctl`并指定账号、密码、服务端地址,执行登录操作。

```shell
ncmctl login cookiecloud -u <user> -p <password> -s http://0.0.0.0:8088
```

cookiecloud登录方式跟cookie方式相比会方便很多,不需要手动拷贝cookie内容,只需要配置好账号、密码、服务端地址,直接从云端拉取cookie内容到本地。

**注意:**

1. 保证服务端地址、账号、密码正确性,否则登录失败。
2. 如果登录出现cookie找不到等相关错误,请在浏览器插件中手动同步cookie到云端，或退出网易云账号,重新登录重复上述操作流程。
3. 如果使用第三方未知安全的cookiecloud服务器,请自行承担风险。

**五、~~二维码登录~~**

> ⚠️ **Warning:** 目前由于网易云风控严重, 暂不支持扫码登录,会出现 **8821 需要行为验证码验证**
> 错误.[相关详情](https://github.com/chaunsin/netease-cloud-music/issues/26)

```shell
ncmctl login qrcode
```

**提示:** 使用手机登录网易云音乐app进行扫码授权登录，如果不能识别终端打印的二维码可根据终端输出得文件路径提示找到二维码图片进行扫描,或者copy终端输出得
`qrcode content: https://www.163.com/xxx` 内容自己生成二维码再进行扫描(_粘贴时不要包含`qrcode content: `
以及结尾空格_)。扫描有时效性,默认超时时间为5分钟,另外扫码过程中
**不能退出终端**!!! 如有问题可重复此流程,为避免被风控不要频繁登录。

在线生成二维码工具: https://www.bejson.com/convert/qrcode/#google_vignette
</pre>
</details>

**二、一键执行每日所有任务**

```shell
ncmctl task
```

默认task包含

- sign (云贝签到+vip签到)
- partner (音乐合伙人)
- scrobble (刷歌300首)

如果只运行某一个任务,比如签到:

```shell
ncmctl task --sign
````

另外`partner`"音乐合伙人"资格不是所有人都有，网易会不定期私信邀请一部分人成为音乐合伙人。由于`ncmctl task`
默认是执行所有任务，因此没有该资格得人执行如下

```shell
ncmctl task --sign --scrobble 
```

如果想更改某一个任务默认运行时间,比如刷歌(scrobble)在每天晚上20:00:00点执行.

```shell
ncmctl task --scrobble.cron "0 20 * * *"
```

提示:

- 需要登录
- 本命令会以服务得方式持续运行,如要退出,请使用`ctrl+c`退出。
- `ncmctl` 采用标准的[crontab](https://zh.wikipedia.org/wiki/Cron)
  表达式进行管理。crontab表达式编写工具[>>>点我<<<](https://crontab.guru/)

> ⚠️ **Warning:** 目前每日签到任务默认关闭自动领取奖励功能,原因是有封号风险,如果还是想开启则指定`--sign.automatic` 参数进行开启。

**三、音乐下载**

1. 下载Hi-Res品质音乐

```shell
# 指定歌曲分享链接
ncmctl download -l hires 'https://music.163.com/song?id=1820944399'
# 指定歌曲id
ncmctl download -l hires '1820944399'
```

**提示:** url地址获取方式可以从分享中获取。如果知道歌曲id可以省略url地址，目前id仅支持歌曲id，不支持其他例如歌手、专辑、歌单id等。

2. 下载无损品质(SQ)音乐,到当前`download`目录下

```shell
ncmctl download -l SQ 'https://music.163.com/song?id=1820944399' -o ./download/ 
```

支持得音质有(从低到高) `standard/128 < higher/192 < exhigh/HQ/320 < lossless/SQ < hires/HR` 参数可指定任意别名。

3. 下载某一张专辑所有音乐,批量下载数量5(最大值20)

```shell
ncmctl download -p 5 'https://music.163.com/#/album?id=34608111'
```

默认批量下载到当前`download`目录下面，音质为无损(SQ)

4. 下载某一歌手的所有音乐

```shell
ncmctl download --strict 'https://music.163.com/#/artist?id=33400892'
```

**提示:** `--strict`为严格默认,当歌曲没有对应品质的音乐时则会忽略下载,如果不指定`--strict`
则默认下载次一级的音乐品质。比如指定HR品质没有对应得资源则下载SQ。

5. 下载某一歌单

```shell
# web端链接
ncmctl download 'https://music.163.com/#/my/m/music/playlist?id=593617579'
# pc端链接 
ncmctl download 'https://music.163.com/playlist?id=593617579'
```

**四、云盘上传**

指定文件上传

```shell
ncmctl cloud '/Users/chaunsin/Music/谁为我停留 - 田震.mp3' 
```

指定目录上传(批量上传)

```shell
ncmctl cloud '/Users/chaunsin/Music/' 
```

默认批量上传数为3,最大为10,可指定`-p`参数设置,同时cloud支持按照自定义过滤条件进行上传详情可使用`-h`
参考命令行。另外输入的目录深度不能超过3层。

**五、.ncm文件解析**

批量解析`/Users/chaunsin/Music/`目录输出到`./ncm`目录下

```shell
ncmctl ncm '/Users/chaunsin/Music/' -o ./ncm
```

支持批量解析,默认参数为10，可以指定`-p`参数设置数量。同样输入的目录深度不能超过3层。

**六、其他命令**

使用以下命令查看帮助

```shell
ncmctl -h
```

## api

参考如下

- [登录](example%2Fexample_login_test.go)
- [云盘上传](example%2Fexample_cloud_upload_test.go)(需要登录)
- [音乐下载](example%2Fexample_download_test.go)(需要登录)

## ❓ 已知问题

### 1.下载无损音乐品质不准确

当使用`ncmctl`下载无损音乐指定`-l lossless`时,会存在下载Hi-Res品质音乐情况,如果歌曲不支持Hi-Res品质音乐,同时有无损品质音乐则正常下载无损音乐,问题还需要排查。

### 2.每日刷歌300首为啥达不到300首

`scrobble`是支持去重功能的,会在`$HOME/.ncmctl/database/`记录听过哪些歌曲记录，但是目前没有找到这样的一个接口,判断当前账户听过哪些歌曲,因此这就会造成每日听歌达不到300首的情况。

举个例子,在使用本程序之前,你听过某一首歌曲比如`反方向的钟 - 周杰伦`
,由于此歌曲没有记录到数据库中,即视为未听过该歌曲造成了重复播放,进而导致不满足300首。

综上所述强烈建议***不要清理`$HOME/.ncmctl/database/`目录下的文件数据***,除非你知道你在干什么。

另外还有一种极端情况,刷歌采用的歌单是top榜单歌曲(top榜单歌曲相对来说都是新歌,不同得歌单更新频率不一样)
，top榜单有50个左右，虽然看起来很多,但实际上还是存在不满足300首新歌情况,如果网易新歌曲更新得不及时,由于有判重复逻辑,因此还是会存在不满足300首得情况。

### 3.ncmctl task和scrobble、sign、partner子命令有啥区别？

task命令是一个服务，默认执行是包含了scrobble、sign、partner子命令功能，启动之后会每天定时执行,如果把此命令部署到服务器上并配合
`nohup`命令去启动就不用每天手动去执行一遍任务了。

再说一下scrobble、sign、partner。这几个子命令不是服务，执行之后会立刻执行相应得任务并返回结果，不像task执行需要”到点了“才会执行。

## ❤️ 鸣谢

### 人员

- [sjpqxuzdly03646](https://github.com/sjpqxuzdly03646) 对"音乐合伙人"功能得支持与帮助
- [stkevintan](https://github.com/stkevintan) 提供cookiecloud登录方式

### 代码库

- https://github.com/Binaryify/NeteaseCloudMusicApi
- https://github.com/mos9527/pyncm
- https://github.com/naruto2o2o/musicdump
- https://crontab.guru

以及本项目所依赖的三方优秀库。
