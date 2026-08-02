// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"path/filepath"
	"reflect"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/cheggaaa/pb/v3"
	"github.com/go-resty/resty/v2"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

const (
	defUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034"
	defDomain    = ".music.163.com"
)

var (
	_wnmcid    = utils.GenerateWNMCID()
	_deviceId  = utils.GenerateDeviceId() // todo: 设备id存在不同格式,考虑使用cryptoMode控制生成？
	_ntes_nnid string
	_ntes_nuid string

	domains = []string{
		"https://music.163.com",
		"https://interface.music.163.com",
		"https://interface3.music.163.com",
	}
)

func init() {
	var err error

	_ntes_nnid, _ntes_nuid, err = utils.GenerateFakeNVID()
	if err != nil {
		panic(fmt.Sprintf("init GenerateFakeNVID err: %s", err))
	}
}

type Config struct {
	Debug   bool          `json:"debug" yaml:"debug"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	Retry   int           `json:"retry" yaml:"retry"`
	Cookie  cookie.Config `json:"cookie" yaml:"cookie"`
	HomeDir string        `json:"-" yaml:"-"`
}

func (c *Config) Validate() error {
	if c.Retry < 0 {
		return errors.New("retry is < 0")
	}

	if c.Timeout < 0 {
		return errors.New("timeout is < 0")
	}
	return nil
}

type Client struct {
	cfg       *Config
	cli       *resty.Client
	cookie    *cookie.Cookie // is coookiejar
	defHeader *Headers
	l         *log.Logger
	xeapi     *xeapi
	anonymous *Anonymous
}

func New(cfg *Config) *Client {
	client, err := NewClient(cfg, log.Default)
	if err != nil {
		panic(err)
	}
	return client
}

func NewClient(cfg *Config, l *log.Logger) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	if l == nil {
		l = log.Default
	}

	stateDir := utils.BaseDir()
	if cfg.HomeDir != "" {
		stateDir = filepath.Join(cfg.HomeDir, ".ncmctl")
	}

	defHeader := defaultHeaders.clone()
	if err := defHeader.Validate(); err != nil {
		return nil, fmt.Errorf("default headers validate: %w", err)
	}

	// 读取header、cookie配置文件并覆盖硬编码默认配置如果文件存。
	cfgPath := filepath.Join(stateDir, "header.yaml")
	if utils.FileExists(cfgPath) {
		if err := defHeader.LoadConfig(cfgPath); err != nil {
			return nil, fmt.Errorf("load '%s' header config err: %w", cfgPath, err)
		}

		l.Info("load header config success")
	}

	// 初始化匿名token
	var (
		anonymousFile = filepath.Join(stateDir, "anonymous_token")
		anonymous     = NewAnonymous(anonymousFile)
	)
	if utils.FileExists(anonymousFile) {
		if err := anonymous.LoadConfig(); err != nil {
			l.Warnf("load '%s' anonymous token config err: %s", anonymousFile, err)
		} else {
			l.Info("load anonymous token config success")
		}
	}

	opts := []cookie.Option{cookie.WithSyncInterval(cfg.Cookie.Interval)}
	if cfg.Cookie.Filepath != "" {
		opts = append(opts, cookie.WithFilePath(cfg.Cookie.Filepath))
	}

	if opt := cfg.Cookie.Options; opt != nil && opt.PublicSuffixList != nil {
		opts = append(opts, cookie.WithPublicSuffixList(cfg.Cookie.PublicSuffixList))
	}

	jar, err := cookie.NewCookie(opts...)
	if err != nil {
		return nil, fmt.Errorf("NewCookie: %w", err)
	}

	cli := resty.New()
	cli.SetRetryCount(cfg.Retry)
	cli.SetTimeout(cfg.Timeout)
	cli.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	cli.SetDebug(cfg.Debug)
	cli.SetCookieJar(jar)
	cli.OnAfterResponse(contentEncoding)
	// cli.OnAfterResponse(dump)
	// cli.SetLogger(l)
	// cli.AddRetryHook(func(resp *resty.Response, err error) {
	// 	l.Warnf("URL:%s,RetryCount:%d,RequestBody:%+v StatusCode:%d,ResponseBody:%s CusumeTime:%s Err:%s",
	// 		resp.Request.URL, resp.Request.Attempt, resp.Request.Body, resp.StatusCode(), resp.Body(), resp.Time(), err)
	// })

	// 初始化xeapi管理器
	xeapi := newXeapi(cli, filepath.Join(stateDir, "xeapi.yaml"))
	if utils.FileExists(xeapi.storePath) {
		if err := xeapi.LoadConfig(); err != nil {
			l.Warnf("load '%s' xeapi config err: %s", xeapi.storePath, err)
		} else {
			l.Info("load xeapi config success")
		}
	}

	c := Client{
		cfg:       cfg,
		cli:       cli,
		cookie:    jar,
		defHeader: &defHeader,
		l:         l,
		xeapi:     xeapi,
		anonymous: anonymous,
	}
	return &c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	// todo: implement
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	c.cli.SetCloseConnection(true)

	// if lerr := c.l.Close(); lerr != nil {
	// 	err = errors.Join(lerr)
	// }
	return errors.Join(c.xeapi.Sync(), c.anonymous.Sync(), c.cookie.Close(ctx))
}

func (c *Client) NewRequest() *resty.Request {
	return c.cli.NewRequest()
}

func (c *Client) GetClient() *http.Client {
	return c.cli.GetClient()
}

// GetAnonymous 获取匿名token管理对象.
func (c *Client) GetAnonymous() *Anonymous {
	return c.anonymous
}

// Cookie 根据url和cookie name获取cookie,从cookiejar中.
func (c *Client) Cookie(url, name string) (http.Cookie, bool) {
	uri, err := neturl.Parse(url)
	if err != nil {
		c.l.Warnf("cookie parse(%v) err: %s", url, err)
		return http.Cookie{}, false
	}

	for _, c := range c.cookie.Cookies(uri) {
		if c.Name == name {
			return *c, true
		}
	}
	return http.Cookie{}, false
}

// GetCookies 获取cookies.
func (c *Client) GetCookies(url *neturl.URL) []*http.Cookie {
	return c.cookie.Cookies(url)
}

// SetCookies 设置cookies.
func (c *Client) SetCookies(url *neturl.URL, cookies []*http.Cookie) {
	c.cookie.SetCookies(url, cookies)
}

// GetCookieValue 从网易云 domains 列表中获取对应的cookie值.
func (c *Client) GetCookieValue(names ...string) string {
	if len(names) == 0 {
		return ""
	}

	for _, domain := range domains {
		if v := c.getCookieValue(domain, names...); v != "" {
			return v
		}
	}
	return ""
}

//nolint:funcorder // 忽略校验.
func (c *Client) getCookieValue(url string, names ...string) string {
	for _, name := range names {
		if ck, ok := c.Cookie(url, name); ok && ck.Value != "" {
			value, err := neturl.QueryUnescape(ck.Value)
			if err != nil {
				value = ck.Value
			}
			return value
		}
	}
	return ""
}

// GetCSRF 获取csrf 一般用于weapi接口中使用.
func (c *Client) GetCSRF(url string) (string, bool) {
	uri, err := neturl.Parse(url)
	if err != nil {
		c.l.Warnf("GetCSRF parse(%v) err: %s", url, err)
		return "", false
	}

	for _, c := range c.cookie.Cookies(uri) {
		if c.Name == "__csrf" && c.Value != "" {
			return c.Value, true
		}

		if c.Name == "__csrf_token" && c.Value != "" {
			return c.Value, true
		}
	}
	return "", false
}

// GetDeviceId 从当前客户端的 Cookie 中获取设备 ID.
func (c *Client) GetDeviceId() string {
	return c.GetCookieValue("deviceId")
}

// GetSDeviceId 从当前客户端的 Cookie 中获取设备 ID.
// NOTE: GetDeviceId 和  GetSDeviceId 二者使用场景以及差异需要明确.
func (c *Client) GetSDeviceId() string {
	return c.GetCookieValue("sDeviceId", "sdeviceId")
}

// GetMusicU 从cookies中获取MUSIC_U或MUSIC_R_U值,也就是正常token.
func (c *Client) GetMusicU() string {
	return c.GetCookieValue("MUSIC_U", "MUSIC_R_U")
}

// GetMusicA 从cookies中获取MUSIC_A也就是匿名token.
func (c *Client) GetMusicA() string {
	return c.GetCookieValue("MUSIC_A")
}

// Request 接口请求.
func (c *Client) Request(ctx context.Context, url string, req, resp any, opts *Options) (*resty.Response, error) {
	if url == "" || req == nil || resp == nil {
		return nil, errors.New("request args invalid")
	}

	if reqValue := reflect.ValueOf(req); reqValue.Kind() == reflect.Pointer && reqValue.IsNil() {
		return nil, errors.New("request args invalid")
	}

	if respValue := reflect.ValueOf(resp); respValue.Kind() == reflect.Pointer && respValue.IsNil() {
		return nil, errors.New("request args invalid")
	}

	if opts == nil {
		opts = NewOptions()
	}

	if opts.Method == "" {
		opts.SetMethod(http.MethodPost)
	}

	def := c.defHeader.HeaderItemForCryptoMode(opts.CryptoMode)
	if def == nil {
		return nil, fmt.Errorf("%s crypto mode unknown", opts.CryptoMode)
	}

	uri, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	var (
		encryptData   map[string]string
		response      *resty.Response
		eapiEncrypted bool
		xeapiRevision xeapiRequestRevision
		requestURL    = url
		cryptoMode    = opts.CryptoMode
		userHeader    = opts.Headers
		userCookie    = opts.Cookies
		defDeviceId   = def.GetCookie("deviceId")
		execudeCookie = []string{"__csrf", "appver", "channel", "deviceId", "ntes_kaola_ad", "os", "osver", "WEVNSM", "WNMCID"}
		execudeHeader = []string{"User-Agent", "X-antiCheatToken"}
		ua            = c.resolveHeaderValue("User-Agent", userHeader, def.GetHeader("User-Agent"))
		appver        = c.resolveRequestCookie("appver", userCookie, def.GetCookie("appver"))
		channel       = c.resolveRequestCookie("channel", userCookie, def.GetCookie("channel"))
		osVal         = c.resolveRequestCookie("os", userCookie, def.GetCookie("os"))
		osver         = c.resolveRequestCookie("osver", userCookie, def.GetCookie("osver"))
		csrf          = c.resolveRequestCookie("__csrf", userCookie, "")
		nka           = c.resolveRequestCookie("ntes_kaola_ad", userCookie, def.GetCookie("ntes_kaola_ad"))
		wevnsm        = c.resolveRequestCookie("WEVNSM", userCookie, def.GetCookie("WEVNSM"))
		wnmcid        = c.resolveRequestCookie("WNMCID", userCookie, _wnmcid)
		musicU        = c.resolveRequestCookie("MUSIC_U", userCookie, def.GetCookie("MUSIC_U"))
		musicRU       = c.resolveRequestCookie("MUSIC_R_U", userCookie, def.GetCookie("MUSIC_R_U"))
	)

	if defDeviceId == "" {
		defDeviceId = _deviceId
	}

	deviceId := c.resolveRequestCookie("deviceId", userCookie, defDeviceId)

	// Host请求头根据请求地址交给底层自己装配
	request := c.cli.R().
		SetContext(ctx).
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept-language", "zh-CN,zh-Hans;q=0.9").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", ua)

	c.setCookie(request, csrf, appver, channel, deviceId, osVal, osver, nka, wevnsm, wnmcid)
	// c.setCookie(request, &http.Cookie{Name: "NMTID", Value: ""}) // 应该是NetEase Machine Token ID,风控是由服务器下发。

	// 当没有正常登录token信息时，则使用匿名登录token
	if (musicU == nil || musicU.Value == "") && (musicRU == nil || musicRU.Value == "") {
		anonymous := c.anonymous.Get()
		if anonymous == "" {
			anonymous = def.GetCookie("MUSIC_A")
		}

		c.setCookie(request, c.resolveRequestCookie("MUSIC_A", userCookie, anonymous))
	}

	// 如果用户传入了token则优先级高于默认token配置
	antiToken := c.resolveCookieOrHeaderValue("x-antiCheatToken", opts, def)
	if t := opts.Headers.Get("X-Anticheattoken"); t != "" {
		antiToken = t
	}

	if antiToken != "" {
		request.SetHeader("X-antiCheatToken", antiToken)
	}

	switch cryptoMode {
	case CryptoModeEAPI:
		c.setCookie(request, c.resolveRequestCookie("buildver", userCookie, def.GetCookie("buildver")))
		c.setCookie(request, c.resolveRequestCookie("sDeviceId", userCookie, deviceId.Value)) // 和deviceId为一个值
		c.setCookie(request, c.resolveRequestCookie("mobilename", userCookie, def.GetCookie("mobilename")))
		c.setCookie(request, c.resolveRequestCookie("resolution", userCookie, def.GetCookie("resolution")))
		c.setCookie(request, c.resolveRequestCookie("versioncode", userCookie, def.GetCookie("versioncode")))
		// c.setCookie(request, c.resolveRequestCookie("requestId", userCookie, utils.GenerateRequestId())) // 是否需要

		request.SetHeader("x-mam-custommark", "okhttp") // 网络栈可能会影响风控策略 eg: okhttp(HTTP/2,TLS Socket)、cronet(HTTP/3,QUIC)
		request.SetHeader("x-aeapi", "true")            // 解密后的响应值为gzip压缩数据
		request.SetHeader("cm_no_encrypt_native_tag_20220105", "false")
		// request.SetHeader("mconfig-info","{\"IuRPVVmc3WWul9fT\":{\"version\":\"86118336\",\"appver\":\"9.2.85\"},\"tPJJnts2H31BZXmp\":{\"version\":\"4077568\",\"appver\":\"4.48.0\"},\"c0Ve6C0uNl2Am0Rl\":{\"version\":\"272384\",\"appver\":\"1.4.30\"},\"zr4bw6pKFDIZScpo\":{\"version\":\"2502656\",\"appver\":\"2.28.0\"}}")
		// request.SetHeader("X-MUSIC-LOC-SITE", "300_rn-vip-growth-level@index/home") // 会跟随改变，貌似和app页面导航索引有关
		// request.SetHeader("CMPageId", "MainProcessRNActivity")                      // 会跟随改变

		_execudeCookie := []string{"buildver", "sDeviceId", "mobilename", "resolution", "versioncode"}
		execudeCookie = append(execudeCookie, _execudeCookie...)

		eapiPayload, encrypted, eapiErr := prepareEAPIRequest(req)
		if eapiErr != nil {
			return nil, fmt.Errorf("prepare EAPI request: %w", eapiErr)
		}

		eapiEncrypted = encrypted

		encryptData, err = crypto.EApiEncrypt(uri.Path, eapiPayload)
		if err != nil {
			return nil, fmt.Errorf("EApiEncrypt: %w", err)
		}
	case CryptoModeWEAPI:
		// TODO: 需要替换？因为有些 https://interface.music.163.com/api 得接口也会走这个逻辑
		// reg, _ := regexp.Compile(`\w*api`)
		// url = reg.ReplaceAllString(url, "weapi")
		// url = strings.ReplaceAll(url, "api", "weapi")
		csrf, has := c.GetCSRF(url)
		if !has {
			c.l.Debugf("get csrf token not found: %s", url)
		}

		request.SetQueryParam("csrf_token", csrf).
			SetCookie(&http.Cookie{Name: "__remember_me", Value: "true", Domain: defDomain, HttpOnly: true})
		c.setCookie(request, c.resolveRequestCookie("_iuqxldmzr_", userCookie, def.GetCookie("_iuqxldmzr_"))) // eg: 33
		c.setCookie(request, c.resolveRequestCookie("_ntes_nnid", userCookie, _ntes_nnid))
		c.setCookie(request, c.resolveRequestCookie("_ntes_nuid", userCookie, _ntes_nuid))
		c.setCookie(request, c.resolveRequestCookie("JSESSIONID-WYYY", userCookie, def.GetCookie("JSESSIONID-WYYY"))) // 生成规则未知,但留出配置选项。
		// request.SetHeader("nm-gcore-status", "1")
		// request.SetHeader("mconfig-info", "{\"IuRPVVmc3WWul9fT\":{\"version\":614400,\"appver\":\"3.0.14.202884\"}}")

		execudeCookie = append(execudeCookie, "_iuqxldmzr_", "JSESSIONID-WYYY")

		encryptData, err = crypto.WeApiEncrypt(req)
		if err != nil {
			return nil, fmt.Errorf("WeApiEncrypt: %w", err)
		}
	case CryptoModeLinux:
		encryptData, err = crypto.LinuxApiEncrypt(req)
		if err != nil {
			return nil, fmt.Errorf("LinuxApiEncrypt: %w", err)
		}
	case CryptoModeXEAPI:
		var (
			buildver   = c.resolveRequestCookie("buildver", userCookie, def.GetCookie("buildver"))
			sDeviceId  = c.resolveRequestCookie("sDeviceId", userCookie, deviceId.Value) // 和deviceId为一个值
			mobilename = c.resolveRequestCookie("mobilename", userCookie, def.GetCookie("mobilename"))
			// reqId       = c.resolveRequestCookie("requestId", userCookie, utils.GenerateRequestId()) // todo: 多余？
			// resolution  = c.resolveRequestCookie("resolution", userCookie, def.GetCookie("resolution"))
			// versioncode = c.resolveRequestCookie("versioncode", userCookie, def.GetCookie("versioncode"))
		)

		c.setCookie(request, buildver)
		c.setCookie(request, sDeviceId) // 和deviceId为一个值
		c.setCookie(request, mobilename)
		// c.setCookie(request, reqId)
		// c.setCookie(request, resolution)
		// c.setCookie(request, versioncode)

		request.SetHeader("x-appver", appver.Value)
		request.SetHeader("x-buildver", buildver.Value)
		request.SetHeader("x-channel", channel.Value)
		request.SetHeader("x-deviceId", deviceId.Value)
		request.SetHeader("x-sDeviceId", sDeviceId.Value)
		request.SetHeader("x-mobilename", mobilename.Value)
		request.SetHeader("x-os", osVal.Value)
		request.SetHeader("x-osver", osver.Value)
		request.SetHeader("x-aeapi", "true")            // 解密后的响应值为gzip压缩数据
		request.SetHeader("x-mam-custommark", "okhttp") // 网络栈可能会影响风控策略 eg: okhttp(HTTP/2,TLS Socket)、cronet(HTTP/3,QUIC)
		request.SetHeader("X-Client-Enc-State", "ENCRYPTED")
		// request.SetHeader("x-resolution", c.getDefCookieVal("resolution", userCookie, def.GetCookie("resolution")))
		// request.SetHeader("x-versioncode", c.getDefCookieVal("versioncode", userCookie, def.GetCookie("versioncode")))

		if musicU != nil && musicU.Value != "" {
			request.SetHeader("x-music-u", musicU.Value)
		}

		_execudeCookie := []string{"buildver", "sDeviceId", "mobilename" /*,"requestId", "resolution", "versioncode"*/}
		_execudeHeader := []string{"x-appver", "x-buildver", "x-channel", "x-deviceId", "x-sDeviceId", "x-mobilename", "x-os", "x-osver", "x-music-u"}

		execudeCookie = append(execudeCookie, _execudeCookie...)
		execudeHeader = append(execudeHeader, _execudeHeader...)

		envelopeURL, xeapiURL, rerr := rewriteXeapiURL(url) // TODO: url xeapi todo
		if rerr != nil {
			return nil, rerr
		}

		requestURL = xeapiURL

		encryptReq := &crypto.XeapiEncryptRequest{
			URI:         envelopeURL,
			Data:        req,
			Method:      opts.Method,
			ContentType: "application/x-www-form-urlencoded",
			OS:          osVal.Value,
			AppVersion:  appver.Value,
			DeviceID:    deviceId.Value,
			UserAgent:   ua,
			T1:          "",
			T2:          "",
			UID:         "", // TODO: 后续处理
		}

		encryptData, xeapiRevision, err = c.xeapi.Encrypt(ctx, encryptReq)
		if err != nil {
			return nil, fmt.Errorf("xeapi encrypt: %w", err)
		}

	case CryptoModeAPI:
		// 不需要加密处理请求
		// TODO: 待处理,在/api/xx/接口请求时则不需要参数加密处理,此处需要对结构体转换成map[string]string类型
		// b, err := json.Marshal(req)
		// if err != nil {
		// 	return nil, fmt.Errorf("json.Marshal: %w", err)
		// }
		// var m map[string]interface{}
		// if err := json.Unmarshal(b, &m); err != nil {
		// 	return nil, fmt.Errorf("json.Unmarshal: %w", err)
		// }
		// encryptData = make(map[string]string)
		// for k, v := range m {
		// 	encryptData[k] = fmt.Sprint(v)
		// }
	default:
		return nil, fmt.Errorf("%s crypto mode unknown", opts.CryptoMode)
	}

	// 设置cookie和header值，但排除execudeCookie和execudeHeader值
	c.setHeaderAndCookie(request, execudeCookie, execudeHeader, opts, def)

	c.l.Debugf("[request] method=%s crypto=%s url=%s payload_type=%T eapiEncrypted=%v encrypted_fields=%d",
		opts.Method, opts.CryptoMode, requestURL, req, eapiEncrypted, len(encryptData))

	// 注意: 大多数请求都是post请求,如果是get请求会丢弃 encryptData 使用者要注意。
	switch opts.Method {
	case http.MethodPost:
		response, err = request.SetFormData(encryptData).Post(requestURL)
	case http.MethodGet:
		response, err = request.Get(requestURL)
	default:
		return nil, fmt.Errorf("%s not surpport http method", opts.Method) // TODO: 需要适配PUT等方法
	}

	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	c.l.Debugf("[response.raw] status=%d bytes=%d", response.StatusCode(), len(response.Body()))

	var decryptData []byte

	switch cryptoMode {
	case CryptoModeAPI:
		// tips: api接口返回数据是明文
		decryptData = response.Body()
	case CryptoModeEAPI:
		decryptData = response.Body()

		if eapiEncrypted {
			xaeapi := c.resolveHeaderValue("x-aeapi", userHeader, def.GetHeader("x-aeapi"))
			c.l.Debugf("x-aepai value %v", xaeapi)

			plaintext, decryptErr := crypto.EApiDecrypt(string(decryptData), "") // 二进制方式解密
			if decryptErr != nil {
				return nil, newAPIError(response.StatusCode(), fmt.Errorf("EApiDecrypt: %w", decryptErr))
			}

			// 如果请求头中传递了 x-aeapi = true 并请求包含 e_r = true
			// 则说明解密后的内容里面gzip压缩需要进行解压缩,这里采用自动识别判断处理。
			decryptData, err = utils.GzipReader(plaintext)
			if err != nil {
				return nil, newAPIError(response.StatusCode(), fmt.Errorf("GzipReader: %w", err))
			}
		}
	case CryptoModeWEAPI:
		decryptData = response.Body()
	case CryptoModeLinux:
		decryptData, err = crypto.LinuxApiDecrypt(string(response.Body()))
		if err != nil {
			return nil, fmt.Errorf("LinuxApiDecrypt: %w", err)
		}
	case CryptoModeXEAPI:
		if err = c.xeapi.updateSession(response, xeapiRevision); err != nil {
			c.l.Warnf("update xeapi session %+v err: %s", xeapiRevision, err)
			// return nil, newAPIError(response.StatusCode(), fmt.Errorf("update xeapi session: %w", err))
		}

		decryptData, err = crypto.XeapiDecrypt(response.Body())
		if err != nil {
			return nil, newAPIError(response.StatusCode(), fmt.Errorf("XeapiDecrypt: %w", err))
		}
	default:
		return nil, fmt.Errorf("%s crypto mode unknown", opts.CryptoMode)
	}

	c.l.Debugf("[response.decrypt] crypto=%s bytes=%d", opts.CryptoMode, len(decryptData))

	decode := json.NewDecoder(bytes.NewReader(decryptData))
	// decode.DisallowUnknownFields()
	if err := decode.Decode(&resp); err != nil {
		return nil, newAPIError(response.StatusCode(), fmt.Errorf("json.NewDecoder: %w", err))
	}

	if response.StatusCode() != http.StatusOK {
		return nil, newAPIError(response.StatusCode(), nil)
	}
	return response, nil
}

// prepareEAPIRequest derives the EAPI defaults from the final wire JSON without
// modifying the caller's request value.
func prepareEAPIRequest(req any) (json.RawMessage, bool, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, false, fmt.Errorf("json.Marshal: %w", err)
	}

	data = bytes.TrimSpace(data)
	if len(data) < 2 || data[0] != '{' || data[len(data)-1] != '}' {
		return nil, false, errors.New("payload must be a JSON object")
	}

	var fields map[string]json.RawMessage
	if err = json.Unmarshal(data, &fields); err != nil {
		return nil, false, fmt.Errorf("json.Unmarshal: %w", err)
	}

	responseEncrypted := true

	if raw, ok := fields["e_r"]; ok {
		raw = bytes.TrimSpace(raw)
		if bytes.Equal(raw, []byte("null")) {
			return nil, false, errors.New("e_r must be a boolean")
		}

		if err = json.Unmarshal(raw, &responseEncrypted); err != nil {
			return nil, false, fmt.Errorf("decode e_r: %w", err)
		}
	} else {
		fields["e_r"] = json.RawMessage("true")
	}

	rawHeader, hasHeader := fields["header"]
	rawHeader = bytes.TrimSpace(rawHeader)

	if !hasHeader || bytes.Equal(rawHeader, []byte("null")) || bytes.Equal(rawHeader, []byte(`""`)) {
		// Header is a JSON string containing JSON, not a JSON object.
		fields["header"] = json.RawMessage(`"{}"`)
	}

	payload, err := json.Marshal(fields)
	if err != nil {
		return nil, false, fmt.Errorf("json.Marshal normalized payload: %w", err)
	}

	return json.RawMessage(payload), responseEncrypted, nil
}

func (c *Client) Upload(ctx context.Context, method, url string, headers map[string]string, data io.Reader, resp any, bar *pb.ProgressBar) (*resty.Response, error) {
	var body any = data
	if bar != nil {
		body = bar.NewProxyReader(data)
	}

	var (
		err      error
		response *resty.Response
		request  = c.cli.R().
				SetContext(ctx).
				SetHeader("Connection", "keep-alive").
				SetHeader("Accept", "*/*").
				SetHeader("Referer", "https://music.163.com").
				SetHeader("User-Agent", defUserAgent). // TODO: 考虑使用defaultHeaders.HeaderItemForCryptoMode(cryptoMode) 方式来控制不同的UA值。
				SetBody(body)
	)

	if len(headers) > 0 {
		request.SetHeaders(headers)
	}

	switch method {
	case http.MethodPost, "":
		response, err = request.Post(url)
	case http.MethodPut:
		response, err = request.Put(url)
	default:
		return nil, fmt.Errorf("%s not surpport http method", method)
	}

	if err != nil {
		return nil, err
	}

	c.l.Debugf("[upload.response] status=%d bytes=%d", response.StatusCode(), len(response.Body()))

	if resp != nil {
		if err := json.Unmarshal(response.Body(), &resp); err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %w", err)
		}
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d response bytes: %d", response.StatusCode(), len(response.Body()))
	}
	return response, nil
}

// Download streams the response body into resp and closes it before returning.
// The returned response is metadata-only; callers must not read response.Body.
func (c *Client) Download(ctx context.Context, url string, headers map[string]string, reqBody io.Reader, resp io.Writer, bar *pb.ProgressBar) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("NewRequestWithContext: %w", err)
	}

	request.Header.Set("Connection", "keep-alive")
	request.Header.Set("Accept", "*/*")
	request.Header.Set("Referer", "https://music.163.com")
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set("Accept-Language", "zh-CN,zh-Hans;q=0.9")
	request.Header.Set("User-Agent", defUserAgent) // TODO: 考虑使用defaultHeaders.HeaderItemForCryptoMode(cryptoMode) 方式来控制不同的UA值。
	request.Header.Set("Range", "bytes=0-")

	for k, v := range headers {
		request.Header.Set(k, v)
	}

	// 下载文件通常非常耗时，为了和正常api区分超时时间此处独立出来咱设置永不超时。
	response, err := c.cli.Clone().SetTimeout(0).GetClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			log.Errorf("close download response body: %v", closeErr)
		}
	}()

	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("http status code: %d", response.StatusCode)
	}

	var body io.Reader = response.Body
	if bar != nil {
		body = bar.NewProxyReader(response.Body)
	}

	n, err := io.Copy(resp, body)
	if err != nil {
		return nil, err
	}

	if n != response.ContentLength {
		return nil, errors.New("file transfer interrupted")
	}
	return response, nil
}

func (c *Client) setCookie(request *resty.Request, ck ...*http.Cookie) {
	for _, c := range ck {
		if c != nil {
			request.SetCookie(c)
		}
	}
}

// setHeaderAndCookie 设置用户传入得cookie和header，以及系统配置的默认cookie、header值，
// 注意: 用户传入值优先级大于系统默认。
func (c *Client) setHeaderAndCookie(req *resty.Request, cookieName, headerName []string, user *Options, def *HeaderItem) {
	var defCookie []*http.Cookie
	if def != nil {
		defCookie = make([]*http.Cookie, 0, len(def.Cookie))
		for name, value := range def.Cookie {
			defCookie = append(defCookie, &http.Cookie{Name: name, Value: value, Domain: defDomain})
		}

		defCookie = c.excludeCookie(cookieName, defCookie)
	}

	var userCookie []*http.Cookie
	if user != nil {
		userCookie = c.excludeCookie(cookieName, user.Cookies)
	}

	userCookieName := make(map[string]struct{}, len(userCookie))
	for _, ck := range userCookie {
		userCookieName[ck.Name] = struct{}{}
	}

	cookies := make([]*http.Cookie, 0, len(defCookie)+len(userCookie))
	for _, ck := range defCookie {
		if _, overridden := userCookieName[ck.Name]; !overridden {
			cookies = append(cookies, ck)
		}
	}

	// 用户传入cookie覆盖系统默认配置的token
	cookies = append(cookies, userCookie...)
	if len(cookies) > 0 {
		req.SetCookies(cookies)
	}

	// 系统默认
	defHeader := c.excludeHeader(headerName, def.Header)
	if len(defHeader) > 0 {
		req.SetHeaderMultiValues(defHeader)
	}

	// 用户设置，覆盖系统默认,底层是map操作会覆盖。
	userHeader := c.excludeHeader(headerName, user.Headers)
	if len(userHeader) > 0 {
		req.SetHeaderMultiValues(userHeader)
	}
}

// excludeHeader 返回 h 中排除指定名称后的 header。
// name 中的字符串会按 HTTP header 规范（首字母大写）进行标准化。
func (c *Client) excludeHeader(name []string, h http.Header) http.Header {
	if len(name) == 0 || len(h) == 0 {
		return h
	}

	exclude := make(map[string]bool, len(name))
	for _, n := range name {
		exclude[http.CanonicalHeaderKey(n)] = true
	}

	resp := make(http.Header, len(h))
	for k, v := range h {
		if !exclude[k] {
			resp[k] = v
		}
	}
	return resp
}

// excludeCookie 返回 cookies 中排除指定名称后的切片。
func (c *Client) excludeCookie(name []string, cookies []*http.Cookie) []*http.Cookie {
	if len(cookies) == 0 {
		return nil
	}

	exclude := make(map[string]bool, len(name))
	for _, n := range name {
		exclude[n] = true
	}

	var resp []*http.Cookie

	for _, ck := range cookies {
		if ck == nil {
			continue
		}

		if !exclude[ck.Name] {
			resp = append(resp, ck)
		}
	}
	return resp
}

func contentEncoding(c *resty.Client, resp *resty.Response) error {
	kind := resp.Header().Get("Content-Encoding")
	// log.Debugf("Content-Encoding: %s Uncompressed: %v", kind, resp.RawResponse.Uncompressed)
	switch kind {
	case "deflate":
		// 为何使用zlib库: https://zlib.net/zlib_faq.html#faq39
		data, err := zlib.NewReader(bytes.NewReader(resp.Body()))
		if err != nil {
			return fmt.Errorf("zlib.NewReader: %w", err)
		}
		defer func() {
			if closeErr := data.Close(); closeErr != nil {
				log.Errorf("deflate.Close: %s", closeErr)
			}
		}()

		bodyBytes, readErr := io.ReadAll(data)
		if readErr != nil {
			return fmt.Errorf("deflate.ReadAll: %w", readErr)
		}

		resp.SetBody(bodyBytes)
	case "br":
		bodyBytes, err := io.ReadAll(brotli.NewReader(bytes.NewReader(resp.Body())))
		if err != nil {
			return fmt.Errorf("cbrotli.Decode: %w", err)
		}

		resp.SetBody(bodyBytes)
	case "gzip":
		// tips: restry 自身已经实现gzip解压缩
	case "":
		// 空则代表是gzip,golang底层会做相应得解压缩处理,为空得原因是,
		// 收到请求后进行解压, 同时删除 Content-Encoding: gzip请求头。
		// 如果想关闭自动解压缩,则可以设置Transport.DisableCompression=true
	default:
		return fmt.Errorf("not supported yet Content-Encoding: %s", kind)
	}
	return nil
}

// func dump(c *resty.Client, resp *resty.Response) error {
// 	// d, err := io.ReadAll(resp.RawBody())
// 	// if err != nil {
// 	// 	return fmt.Errorf("ReadAll: %w", err)
// 	// }
// 	// log.Debugf("rawbody:%s", string(d))

// 	resp.RawResponse.Body = io.NopCloser(bytes.NewReader(resp.Body()))
// 	log.Debugf("############### http dump ################")

// 	dumpReq, err := httputil.DumpRequest(resp.Request.RawRequest, true)
// 	if err != nil {
// 		return fmt.Errorf("DumpRequest: %w", err)
// 	}
// 	log.Debugf("---------------- request ----------------\n%s", string(dumpReq))

// 	dumpResp, err := httputil.DumpResponse(resp.RawResponse, true)
// 	if err != nil {
// 		return fmt.Errorf("DumpResponse: %w", err)
// 	}
// 	log.Debugf("---------------- response ----------------\n%s\n", string(dumpResp))
// 	return nil
// }
