# 在青龙(qinglong)中运行

基本思路是在青龙容器中安装`ncmctl`二进制包，利用青龙的拉取命令或订阅功能，拉取本仓库源码，添加cron定时任务，并定时运行相应的Task。

## 1. 准备工作

### 1.1 什么是青龙(qinglong)面板

[青龙面板](https://github.com/whyour/qinglong) 是一个定时任务管理面板，可以定时执行各种任务，比如定时更新订阅，定时执行脚本，定时执行命令等。

在实际场景中广泛用于自动化任务调度和脚本运行管理，特别是在与电商相关的自动化脚本领域（如京东签到、淘宝任务等）。

### 1.2 安装青龙(qinglong)面板

需提前安装好青龙面板工具,如果没安装则移步参考官方安装教程 [青龙面板安装](https://github.com/whyour/qinglong)。

本人示例安装的版本为`2.17.12`

如果已经安装则跳过此步骤。

## 2. 安装

### 2.1 在青龙面板中添加拉库定时任务

两种方式，任选其一即可：

#### 2.1.1 方式一：订阅管理(推荐方式)

在`订阅管理`管理导航栏中，右上角`创建订阅`，填入以下信息：

```text
名称：网易云音乐
类型：公开仓库
链接：https://github.com/chaunsin/netease-cloud-music.git
定时类型：interval
定时规则：1天
白名单：qinglong_ncmctl_
文件后缀：sh
```

定时规则可根据自己得需求进行设置，没提到的最好不要动除非你知道你在干什么。

保存后，点击`运行`按钮，运行拉库,并注意运行状态及日志，如果拉库成功，会自动添加ncmctl相关的task任务。

#### 2.1.2 方式二：定时任务拉库

打开青龙面板，`定时任务`页，右上角`创建任务`，填入以下信息：

```text
名称: 网易云音乐
命令: ql repo https://github.com/chaunsin/netease-cloud-music.git qinglong_ncmctl_ "" "" "" sh
定时类型: 常规定时
定时规则: 0 10 * * *
```

保存后，点击`运行`按钮，运行拉库,并注意运行状态及日志，如果拉库成功，会自动添加ncmctl相关的task任务。

`ql`命令使用介绍: https://qinglong.online/guide/user-guide/basic-explanation

### 2.2 检查定时任务

如果正常，拉库成功后，会自动添加ncmctl相关的task任务。

![qinglong-1.png](images/qinglong-1.png)

### 2.3 安装ncmctl

首次添加需要手动安装ncmctl,不然需要定时任务时间到了才会去安装。

在青龙面板中，`定时任务`页，找到`ncmctl安装`脚本，并点击`运行`。需要注意安装是否成功。

![qinglong-3.png](images/qinglong-3.png)

### 2.4 登录

目前支持5种登录方式

#### 2.4.1 短信登录

由于青龙脚本执行时没有地方可以输入短信验证码，没法完整自动化登录流程，因此需要手动进入青龙终端中才能执行短信登录方式。

流程如下:

1. 进入青龙终端中
2. 找到`ncmctl`安装目录,通常在`/usr/local/bin`运行`ncmctl`，成功之后终端会输出以下内容

```shell
# 替换成你自己得手机号
ncmctl login phone 188xxxx8888
send sms success
please input sms captcha: 
```

3. 根据上述内容提示，输入短信验证码进行登录,成功内容如下

```shell
verify sms success
login success: &{RespCommon:{Code:200 Message: Msg: Data:<nil>} Account:0xc00036a070 Profile:0xc0005a8180}
```

**注意:**

1. 发送短信每日有限制,请不要频繁登录避免风控。
2. 有时显示`send sms success`
   但等了很久依然没有收到短信,可能是短信运营商抽风,可以重新发送短信或者稍后再试。如果尝试多次还是失败，可能账号因某些原因入了黑名单,具体验证方式可以登录网易云网页端走短信登录正规流程看是否能收到短信。

#### 2.4.2 手机号密码登录

环境变量配置：

```shell
# 登录方式手机号
export NCMCTL_QINGLONG_LOGIN_MODE=phone
# 登录手机号,替换成你自己的手机号。
export NCMCTL_QINGLONG_LOGIN_ACCOUNT=188xxxx8888
# 登录密码,替换成你自己的实际密码
export NCMCTL_QINGLONG_LOGIN_PASSWORD=123456
```

使用密码登录方式,需要在网易云中设置账号允许手机号密码登录方式,如果未设置请先设置。

密码登录方式容易出现安全风险相关问题,`8821 需要行为验证码验证`未必会成功,可作为尝试登录的一种方式。

**注意: 不要泄露密码。**

#### 2.4.3 cookie登录

当使用`ncmctl`进行手机号登录、扫码登录等场景失败时，可以尝试使用cookie登录,cookie登录属于保底方案。

cookie内容得获取方式有很多，比如可以通过浏览器安装插件的方式进行获取，可参考使用工具 [Cookie Editor](https://chromewebstore.google.com/detail/cookie-editor/ookdjilphngeeeghgngjabigmpepanpl)
或其他cookie导出工具。

环境变量配置：

以下二选一

```shell
# 导入cookie字符串文本内容
export NCMCTL_QINGLONG_LOGIN_COOKIE=string_content
# 导入cookie文件内容
export NCMCTL_QINGLONG_LOGIN_COOKIE=./cookie.text
```

cookie内容支持三种类型格式

- header
- json
- [Netscape](https://docs.cyotek.com/cyowcopy/1.10/netscapecookieformat.html)

详情使用，以及文件格式规则可查看 `ncmctl login cookie -h` 介绍

#### 2.4.3 cookiecloud登录(默认登录方式)

cookiecloud还是cookie另一种登录方式,cookiecloud也是浏览器cookie管理插件工具得一种，它得特点是浏览器可以自动同步cookie到云端，并对cookie内容进行加密存储，业务场景上可直接从云端拉取cookie内容到本地,以供后续使用。

cookiecloud详细介绍:

- https://github.com/easychen/CookieCloud/blob/master/README_cn.md 介绍
- https://juejin.cn/post/7190963442017108027 使用教程
- https://chromewebstore.google.com/detail/cookiecloud/ffjiejobkoibkjlhjnlgmcnnigeelbdl chrome插件地址

操作流程:

1. 安装cookiecloud插件
2. 配置好cookiecloud相关配置
3. 网页端保证成功登录到网易云音乐
4. 为了保证即时同步,点击【手动同步】按钮同步到服务器。
5. 配置环境变量(环境变量内容要和步骤2中得内容要一致)
6. 执行`ncmctl登录`任务

环境变量配置：

```shell
# 登录方式cookiecloud
export NCMCTL_QINGLONG_LOGIN_MODE=cookiecloud
# 登录cookiecloud账号,替换成你自己的cookiecloud账号,也就是 "用户KEY · UUID"
export NCMCTL_QINGLONG_LOGIN_ACCOUNT=qZMVzxGoybHYbYEJM12345
# 登录cookiecloud密码,替换成你自己的实际密码 "端对端加密密码"
export NCMCTL_QINGLONG_LOGIN_PASSWORD=kTduz4A61D4a9LwS712345
# cookiecloud 服务端访问地址,替换成你自己的服务端地址
export NCMCTL_QINGLONG_LOGIN_COOKIECLOUD_SERVER=http://127.0.0.1:8088
```

cookiecloud登录方式跟cookie方式相比会方便很多,不需要手动拷贝cookie内容,只需要配置好账号、密码、服务端地址,直接从云端拉取cookie内容到本地。

**注意:**

1. 保证服务端地址、账号、密码正确性,否则登录失败。
2. 如果登录出现cookie找不到等相关错误,请在浏览器插件中手动同步cookie到云端，或退出网易云账号,重新登录重复上述操作流程。
3. 如果使用第三方未知安全的cookiecloud服务器,请自行承担风险。

#### ~~2.4.4 手机扫码登录~~

> ⚠️ **Warning:** 目前由于网易云风控严重, 暂不支持扫码登录,会出现`8821 需要行为验证码验证`
> 错误.[相关详情](https://github.com/chaunsin/netease-cloud-music/issues/26)

环境变量配置：

```shell
export NCMCTL_QINGLONG_LOGIN_MODE=qrcode
```

设置完环境变量,在青龙定时任务中，点击运行`ncmctl登录`任务，查看运行日志，扫描日志中的二维码进行登录。

![qinglong-2.png](images/qinglong-2.png)

**提示:** 使用手机登录网易云音乐app进行扫码授权登录，如果不能识别终端打印的二维码可根据终端输出得文件路径提示找到二维码图片进行扫描,或者copy终端输出得
`qrcode content: https://www.163.com/xxx` 内容自己生成二维码再进行扫描(_粘贴时不要包含`qrcode content: `
以及结尾空格_)。扫描有时效性,默认超时时间为5分钟,另外扫码过程中
**不能退出**!!! 如有问题可重复此流程,为避免被风控不要频繁登录。

在线生成二维码工具: https://www.bejson.com/convert/qrcode/#google_vignette

### 2.5 定时任务相关环境变量配置

默认情况下,此脚本会执行所有定时任务，如需关闭某些任务可以添加环境变量进行相应的控制。

环境变量主要有

- `NCMCTL_QINGLONG_SIGN` 是否开启签到任务 true: 开启(默认) false: 关闭
- `NCMCTL_QINGLONG_SIGN_AUTOMATIC` 每日签到任务是否自动领取奖励，目前建议关闭避免封号,默认关闭 true: 开启 false: 关闭(默认)
- `NCMCTL_QINGLONG_SCROBBLE` 是否开启刷歌 true: 开启(默认) false: 关闭
- `NCMCTL_QINGLONG_PARTNER` 是否开启音乐合伙人 true: 开启(默认) false: 关闭

**提示**:
如果没有相应的权限，或已经彻底完成得任务，建议关闭不然会有封号的风险,相关问题参考: https://github.com/chaunsin/netease-cloud-music/issues/24

## 3. 常见问题

### 3.1 github访问失败超时等问题

拉库时，受到国内网络限制影响，访问GitHub速度慢或者错误，可在仓库地址前加上代理进行加速访问。

如：

```text
https://ghproxy.cn/https://github.com/chaunsin/netease-cloud-music.git
https://ghproxy.cc/https://github.com/chaunsin/netease-cloud-music.git
https://ghproxy.net/https://github.com/chaunsin/netease-cloud-music.git
https://github.moeyy.xyz/https://github.com/chaunsin/netease-cloud-music.git
```

加速代理地址通常不能保证长期有效，请自行查找或参考以下使用。

https://github.com/hunshcn/gh-proxy/issues/116

### 3.2 not found command 错误

通常情况下在首次安装时会出现此问题,原因是没有先执行`ncmctl安装`任务。先执行此任务然后在尝试登录、一键任务等其他操作做。

如果碰到以下错误：

```text
inglong_ncmctl_install.sh: line 127: /usr/local/bin/ncmctl: cannot execute: required file not found
qinglong_ncmctl_install.sh: line 252: pop_var_context: head of shell_variables not a function context
/ql/shell/otask.sh: line 286: pop_var_context: head of shell_variables not a function context
```

这个错误可能是`ncmctl`命令不能正确执行,对应得命令可能不适配当前的系统架构或者版本、或脚本有问题, 可进入到青龙部署所在的服务器内查看
`ncmctl`命令是否存在、能否正常运行。
