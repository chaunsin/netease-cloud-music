# netease-cloud-music

[![GoDoc](https://godoc.org/github.com/chaunsin/netease-cloud-music?status.svg)](https://godoc.org/github.com/chaunsin/netease-cloud-music) [![Go Report Card](https://goreportcard.com/badge/github.com/chaunsin/netease-cloud-music)](https://goreportcard.com/report/github.com/chaunsin/netease-cloud-music)

网易云音乐 Golang API 接口 + 命令行工具

# 重要声明

本项目仅供个人学习使用,切勿用于商业用途使用！！！

# 功能

### ncmctl

命令行支持以下功能

- [x] 支持接口参数加解密便于调试
- [x] `curl`子命令调用网易云音乐接口,无需关心接口入参出参加解密问题便于调试
    - [ ] 支持动态链接请求
- [x] “音乐合伙人”自动测评
- [x] 云盘上传
- [x] 每日刷歌300首
- [x] 云贝每日签到
- [ ] 音乐下载
- [ ] vip每日签到
- [ ] vip日常任务完成(待考虑)
- [ ] .ncm文件解析转换为.mp3/.flac

### api

- weapi 网页端、小程序使用
- eapi PC端、移动端使用

待补充

# 要求

- golang >= 1.21

# 使用示例

参考如下

- [登录](example%2Fexample_login_test.go)
- [云盘上传](example%2Fexample_cloud_upload_test.go)

# 项目参考

感谢

- https://github.com/Binaryify/NeteaseCloudMusicApi
- https://github.com/mos9527/pyncm
- https://github.com/naruto2o2o/musicdump
