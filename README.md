# netease-cloud-music

[![GoDoc](https://godoc.org/github.com/chaunsin/netease-cloud-music?status.svg)](https://godoc.org/github.com/chaunsin/netease-cloud-music) [![Go Report Card](https://goreportcard.com/badge/github.com/chaunsin/netease-cloud-music)](https://goreportcard.com/report/github.com/chaunsin/netease-cloud-music)

网易云音乐 Golang API 接口 + 命令行工具套件 + 一键完成任务

# 声明

本项目仅供个人学习使用,切勿用于商业用途、非法用途使用！！！

# 功能

## 命令行 (ncmctl)

- [x] 一键每日任务完成(音乐合伙人、云贝签到、刷歌300首)
- [x] 每日签到(云贝签到)
- [x] “音乐合伙人”自动测评
- [x] 每日刷歌300首(带去重功能)
- [x] 云盘上传(支持并行批量上传)
- [x] .ncm文件解析转换为.mp3/.flac(支持批量解析)
- [x] 支持接口参数加解密便于调试
- [x] `curl`子命令调用网易云音乐API,无需关心出入参数加解密问题便于调试
    - [ ] 支持动态链接请求
- [ ] 音乐下载
- [ ] vip每日签到
- [ ] vip日常任务完成(待考虑)

## api

- weapi 网页端、小程序使用
- eapi PC端、移动端使用

**提示:**
由于时间精力有限,目前主要实现了weapi也推荐使用weapi,接口相对较全。

# 要求

- golang >= 1.21
- git (可选)

# ncmctl

## 安装

```shell
go install github.com/chaunsin/netease-cloud-music/cmd/ncmctl@latest
```

或

```shell
git clone https://github.com/chaunsin/netease-cloud-music.git
make install
```

提示: 会安装到`$GOPATH/bin`下

## 使用

**一、登录**

```shell
ncmctl login qrcode
```

**提示:** 使用手机登录网易云音乐app进行扫码授权登录，如果不能识别终端打印的二维码可根据终端输出得文件路径找到二维码图片文件进行扫描。切记扫码过程中
**不能退出终端**!!! 如有问题可重复此流程。

**二、一键执行每日所有任务**

```shell
ncmctl task
```

**提示:** 默认task包含

- sign (签到)
- partner (音乐合伙人)
- scrobble (刷歌300首)

如果只运行某一个任务,比如签到:

```shell
ncmctl task --sign
````

更改某一个任务默认运行时间,比如刷歌(scrobble)在每天晚上20:00:00点执行.

```shell
ncmctl task --scrobble.cron "0 20 * * *"
```

提示:

- 需要登录
- 本命令会一直持续运行,如要退出,请使用`ctrl+c`退出。
- `ncmctl` 采用标准的[crontab](https://zh.wikipedia.org/wiki/Cron)
  表达式进行管理。crontab表达式编写工具[>>>点我<<<](https://crontab.guru/)

**三、云盘上传**

指定目录上传(批量上传)

```shell
ncmctl cloud -i '/Users/chaunsin/Music/' 
```

指定文件上传

```shell
ncmctl cloud '/Users/chaunsin/Music/谁为我停留 - 田震.mp3' 
```

**四、.ncm文件解析**

```shell
ncmctl ncm -i '/Users/chaunsin/Music/' -o ./ncm
```

**五、其他命令**

```shell
./ncmctl -h
ncmctl is a toolbox for netease cloud music.

Usage:
  ncmctl [command]

Examples:
  ncmctl cloud
  ncmctl crypto
  ncmctl login
  ncmctl curl
  ncmctl partner

Available Commands:
  cloud       [need login] Used to upload music files to netease cloud disk
  completion  Generate the autocompletion script for the specified shell
  crypto      Crypto is a tool for encrypting and decrypting the http data
  curl        Like curl invoke netease cloud music api
  help        Help about any command
  login       Login netease cloud music
  ncm         Automatically parses .ncm to mp3/flac
  partner     [need login] Executive music partner daily reviews
  scrobble    [need login] Scrobble execute refresh 300 songs
  sign        [need login] Sign perform daily cloud shell check-in and vip check-in
  task        [need login] Daily tasks are executed asynchronously [partner、scrobble、sign]

Flags:
  -c, --config string   configuration file path
      --debug           
  -h, --help            help for ncmctl
  -v, --version         version for ncmctl

Use "ncmctl [command] --help" for more information about a command.
```

# api

参考如下

- [登录](example%2Fexample_login_test.go)
- [云盘上传](example%2Fexample_cloud_upload_test.go)(需要登录)
- [音乐下载](example%2Fexample_download_test.go)(需要登录)

# 鸣谢

- https://github.com/Binaryify/NeteaseCloudMusicApi
- https://github.com/mos9527/pyncm
- https://github.com/naruto2o2o/musicdump
- https://crontab.guru
