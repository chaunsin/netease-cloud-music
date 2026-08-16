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
	"slices"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/cheggaaa/pb/v3"
	"github.com/go-resty/resty/v2"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
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
	domainURLs []*neturl.URL

	protocolCookieNames = []string{
		"__csrf", "__csrf_token", "MUSIC_U", "MUSIC_R_U", "MUSIC_A",
		"appver", "buildver", "channel", "deviceId", "sDeviceId", "sdeviceId",
		"mobilename", "ntes_kaola_ad", "os", "osver", "resolution", "versioncode",
		"WEVNSM", "WNMCID", "x-antiCheatToken",
	}
)

func init() {
	var err error

	_ntes_nnid, _ntes_nuid, err = utils.GenerateFakeNVID()
	if err != nil {
		panic(fmt.Sprintf("init GenerateFakeNVID err: %s", err))
	}

	domainURLs = make([]*neturl.URL, 0, len(domains))
	for _, domain := range domains {
		u, err := neturl.Parse(domain)
		if err != nil {
			panic(fmt.Sprintf("init parse domain %q err: %s", domain, err))
		}

		domainURLs = append(domainURLs, u)
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
	cfg             *Config
	cli             *resty.Client
	cookieJar       *cookie.Cookie
	cookieTransport *cookieTransport
	defHeader       *Headers
	l               *log.Logger
	xeapi           *xeapi
	anonymous       *Anonymous
	closeOnce       sync.Once
	closed          chan struct{}
	closeErr        error
}

func New(cfg *Config) *Client {
	client, err := NewClient(cfg, log.GetDefault())
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
		l = log.GetDefault()
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
	cli.SetCookieJar(nil) // resty 默认使用cookiejar
	cli.SetRetryCount(cfg.Retry)
	cli.SetTimeout(cfg.Timeout)
	cli.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	cli.SetDebug(cfg.Debug)
	cli.OnAfterResponse(contentEncoding)
	transport := newCookieTransport(jar, cli.GetClient().Transport)
	cli.SetTransport(transport)
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
		cfg:             cfg,
		cli:             cli,
		cookieJar:       jar,
		cookieTransport: transport,
		defHeader:       &defHeader,
		l:               l,
		xeapi:           xeapi,
		anonymous:       anonymous,
		closed:          make(chan struct{}),
	}
	return &c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	// todo: implement
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("api client close context is nil")
	}

	c.closeOnce.Do(func() {
		if c.closed == nil {
			c.closed = make(chan struct{})
		}

		if c.cookieTransport != nil {
			c.cookieTransport.startClose()
		}

		go func() {
			if c.cookieTransport != nil {
				c.cookieTransport.Close()
				c.cookieTransport.CloseIdleConnections()
			}

			var errs []error
			if c.xeapi != nil {
				errs = append(errs, c.xeapi.Sync())
			}

			if c.anonymous != nil {
				errs = append(errs, c.anonymous.Sync())
			}

			if c.cookieJar != nil {
				errs = append(errs, c.cookieJar.Close(context.Background()))
			}

			c.closeErr = errors.Join(errs...)
			close(c.closed)
		}()
	})

	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case <-c.closed:
		if err := ctx.Err(); err != nil {
			return err
		}
		return c.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) NewRequest() *resty.Request {
	return c.cli.NewRequest()
}

// GetClient returns the underlying HTTP client for advanced configuration.
// Its Jar must remain nil, and its Transport must not be replaced; use
// SetTransport to preserve Cookie management.
func (c *Client) GetClient() *http.Client {
	return c.cli.GetClient()
}

// SetTransport replaces the transport below Cookie management. A nil value is
// ignored. Custom transports must honor request context cancellation.
func (c *Client) SetTransport(transport http.RoundTripper) *Client {
	if c.cookieTransport != nil {
		c.cookieTransport.SetTransport(transport)
	}
	return c
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
	return c.cookieByURL(uri, name)
}

//nolint:funcorder // 忽略校验.
func (c *Client) cookieByURL(uri *neturl.URL, name string) (http.Cookie, bool) {
	if uri == nil || c.cookieJar == nil {
		return http.Cookie{}, false
	}

	for _, ck := range c.cookieJar.Cookies(uri) {
		if ck != nil && ck.Name == name {
			return *ck, true
		}
	}
	return http.Cookie{}, false
}

// GetCookies 获取cookies.
func (c *Client) GetCookies(url *neturl.URL) []*http.Cookie {
	return c.cookieJar.Cookies(url)
}

// SetCookies 设置cookies.
func (c *Client) SetCookies(url *neturl.URL, cookies []*http.Cookie) {
	c.cookieJar.SetCookies(url, cookies)
}

// GetCookieValue 从网易云 domains 列表中获取第一个不为空的cookie值.
func (c *Client) GetCookieValue(names ...string) string {
	if len(names) == 0 {
		return ""
	}

	for _, uri := range domainURLs {
		if v := c.getCookieValueByURL(uri, names...); v != "" {
			return v
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

	if val := c.getCookieValueByURL(uri, "__csrf", "__csrf_token"); val != "" {
		return val, true
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
	if ctx == nil {
		return nil, errors.New("request context is nil")
	}

	if url == "" || req == nil || resp == nil {
		return nil, errors.New("request args invalid")
	}

	if reqValue := reflect.ValueOf(req); reqValue.Kind() == reflect.Pointer && reqValue.IsNil() {
		return nil, errors.New("request args invalid")
	}

	if respValue := reflect.ValueOf(resp); respValue.Kind() == reflect.Pointer && respValue.IsNil() {
		return nil, errors.New("request args invalid")
	}

	opts = cloneOptions(opts)
	if opts.cookieErr != nil {
		return nil, fmt.Errorf("prepare request Cookies: %w", opts.cookieErr)
	}

	var (
		envelopeURL string
		requestURL  = url
	)

	// 根据加密类型加载默认header和cookie值。
	def := c.defHeader.HeaderItemForCryptoMode(opts.CryptoMode)
	if def == nil {
		return nil, fmt.Errorf("%s crypto mode unknown", opts.CryptoMode)
	}

	if opts.CryptoMode == CryptoModeXEAPI {
		var rewriteErr error

		envelopeURL, requestURL, rewriteErr = rewriteXeapiURL(url)
		if rewriteErr != nil {
			return nil, rewriteErr
		}
	}

	uri, err := neturl.Parse(requestURL)
	if err != nil {
		return nil, err
	}

	requestHeaders := mergeRequestHeaders(def.Header, opts.Headers)
	cookieURL := cookieURLForHost(uri, requestHeaders.Get("Host"))

	policy, cookieErr := newRequestCookiePolicy(cookieURL, opts.cookieSnapshot(), c.cookieJar.Cookies(cookieURL))
	if cookieErr != nil {
		return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
	}

	if cookieErr := policy.setDefaultCookies(def.Cookie); cookieErr != nil {
		return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
	}

	if cookieErr := policy.setDefaultIfMissing("WNMCID", _wnmcid); cookieErr != nil {
		return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
	}

	if cookieErr := policy.setDefaultIfMissing("deviceId", _deviceId); cookieErr != nil {
		return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
	}

	policy.deleteDefault("MUSIC_A")

	if _, hasMusicU := policy.cookieScalar("MUSIC_U", "MUSIC_R_U"); !hasMusicU {
		anonymous := ""
		if c.anonymous != nil {
			anonymous = c.anonymous.Get()
		}

		if anonymous == "" {
			anonymous = def.GetCookie("MUSIC_A")
		}

		if cookieErr := policy.setDefaultCookie("MUSIC_A", anonymous); cookieErr != nil {
			return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
		}
	}

	var (
		encryptData     map[string]string
		response        *resty.Response
		eapiEncrypted   bool
		xeapiRevision   xeapiRequestRevision
		cryptoMode      = opts.CryptoMode
		excludedHeaders = []string{"User-Agent", "Cookie", "X-antiCheatToken"}
		ua              = requestHeaders.Get("User-Agent")
	)

	// 默认 Host 由底层根据 URL 装配；自定义 Host 与 net/http 一样参与 Cookie 定域。
	request := c.cli.R().
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept-language", "zh-CN,zh-Hans;q=0.9").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", ua)

	switch cryptoMode {
	case CryptoModeEAPI:
		deviceID, hasDeviceID := policy.cookieScalar("deviceId")
		if _, hasSDeviceID := policy.cookieScalar("sDeviceId", "sdeviceId"); !hasSDeviceID && hasDeviceID {
			if cookieErr := policy.setDefaultCookie("sDeviceId", deviceID); cookieErr != nil {
				return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
			}
		}

		request.SetHeader("x-mam-custommark", "okhttp") // 网络栈可能会影响风控策略 eg: okhttp(HTTP/2,TLS Socket)、cronet(HTTP/3,QUIC)
		request.SetHeader("x-aeapi", "true")            // 解密后的响应值为gzip压缩数据
		request.SetHeader("cm_no_encrypt_native_tag_20220105", "false")
		// request.SetHeader("mconfig-info","{\"IuRPVVmc3WWul9fT\":{\"version\":\"86118336\",\"appver\":\"9.2.85\"},\"tPJJnts2H31BZXmp\":{\"version\":\"4077568\",\"appver\":\"4.48.0\"},\"c0Ve6C0uNl2Am0Rl\":{\"version\":\"272384\",\"appver\":\"1.4.30\"},\"zr4bw6pKFDIZScpo\":{\"version\":\"2502656\",\"appver\":\"2.28.0\"}}")
		// request.SetHeader("X-MUSIC-LOC-SITE", "300_rn-vip-growth-level@index/home") // 会跟随业务场景改变，貌似和app页面导航索引有关
		// request.SetHeader("CMPageId", "MainProcessRNActivity")                      // 同上

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
		if cookieErr := policy.setDefaultCookie("__remember_me", "true"); cookieErr != nil {
			return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
		}

		if cookieErr := policy.setDefaultIfMissing("_ntes_nnid", _ntes_nnid); cookieErr != nil {
			return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
		}

		if cookieErr := policy.setDefaultIfMissing("_ntes_nuid", _ntes_nuid); cookieErr != nil {
			return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
		}

		csrf, hasCSRF := policy.cookieScalar("__csrf", "__csrf_token")
		if csrf == "" {
			c.l.Debugf("get csrf token not found: %s", url)
		}

		if hasCSRF {
			request.SetQueryParam("csrf_token", unescapeCookieValue(csrf))
		}
		// request.SetHeader("nm-gcore-status", "1")
		// request.SetHeader("mconfig-info", "{\"IuRPVVmc3WWul9fT\":{\"version\":614400,\"appver\":\"3.0.14.202884\"}}")

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
			appver, hasAppver         = policy.cookieScalar("appver")
			buildver, hasBuildver     = policy.cookieScalar("buildver")
			channel, hasChannel       = policy.cookieScalar("channel")
			mobilename, hasMobilename = policy.cookieScalar("mobilename")
			osVal, hasOS              = policy.cookieScalar("os")
			osver, hasOSVer           = policy.cookieScalar("osver")
			musicU, hasMusicU         = policy.cookieScalar("MUSIC_U", "MUSIC_R_U")
			deviceID, hasDeviceID     = policy.cookieScalar("deviceId")
			sDeviceID, hasSDeviceID   = policy.cookieScalar("sDeviceId", "sdeviceId")
			// reqId       = utils.GenerateRequestId() // todo: 多余？
			// resolution, hasResolution  = policy.cookieScalar("resolution")
			// versioncode, hasVersioncode = policy.cookieScalar("versioncode")
		)

		if !hasSDeviceID && hasDeviceID {
			if cookieErr := policy.setDefaultCookie("sDeviceId", deviceID); cookieErr != nil {
				return nil, fmt.Errorf("prepare request Cookies: %w", cookieErr)
			}

			sDeviceID = deviceID
			hasSDeviceID = true
		}

		if hasSDeviceID || hasDeviceID {
			request.SetHeader("x-sDeviceId", sDeviceID)
		}

		setHeader(request, "x-appver", hasAppver, appver)
		setHeader(request, "x-buildver", hasBuildver, buildver)
		setHeader(request, "x-channel", hasChannel, channel)
		setHeader(request, "x-deviceId", hasDeviceID, deviceID)
		setHeader(request, "x-mobilename", hasMobilename, mobilename)
		setHeader(request, "x-os", hasOS, osVal)
		setHeader(request, "x-osver", hasOSVer, osver)
		setHeader(request, "x-music-u", hasMusicU, musicU)
		// setHeader(request, "x-resolution", hasResolution, resolution)
		// setHeader(request, "x-versioncode", hasVersioncode, versioncode)

		request.SetHeader("x-aeapi", "true")            // 解密后的响应值为gzip压缩数据
		request.SetHeader("x-mam-custommark", "okhttp") // 网络栈可能会影响风控策略 eg: okhttp(HTTP/2,TLS Socket)、cronet(HTTP/3,QUIC)
		request.SetHeader("X-Client-Enc-State", "ENCRYPTED")

		excludedHeaders = append(excludedHeaders, "x-appver", "x-buildver", "x-channel", "x-deviceId", "x-sDeviceId", "x-mobilename", "x-os", "x-osver", "x-music-u")

		encryptReq := &crypto.XeapiEncryptRequest{
			URI:         envelopeURL,
			Data:        req,
			Method:      opts.Method,
			ContentType: "application/x-www-form-urlencoded",
			OS:          osVal,
			AppVersion:  appver,
			DeviceID:    deviceID,
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

	antiToken, hasAntiToken := policy.cookieScalar("x-antiCheatToken")
	if !hasAntiToken {
		antiToken = requestHeaders.Get("X-Anticheattoken")
	}

	if antiToken != "" {
		request.SetHeader("X-antiCheatToken", antiToken)
	}

	setRequestHeaders(request, requestHeaders, excludedHeaders)
	policy.finalize(protocolCookieNames)
	request.Header.Del("Cookie")
	request.SetCookies(policy.options)
	request.SetContext(withRequestCookiePolicy(ctx, policy))

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
			xaeapi := requestHeaders.Get("X-Aeapi")
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

// Upload 上传文件或数据.
func (c *Client) Upload(ctx context.Context, url string, data io.Reader, resp any, opts *Options, bar *pb.ProgressBar) (*resty.Response, error) {
	if ctx == nil {
		return nil, errors.New("upload context is nil")
	}

	if url == "" || data == nil {
		return nil, errors.New("upload args invalid")
	}

	if dataValue := reflect.ValueOf(data); dataValue.Kind() == reflect.Pointer && dataValue.IsNil() {
		return nil, errors.New("upload args invalid")
	}

	if resp != nil {
		if respValue := reflect.ValueOf(resp); respValue.Kind() == reflect.Pointer && respValue.IsNil() {
			return nil, errors.New("upload args invalid")
		}
	}

	opts = cloneOptions(opts)
	if opts.cookieErr != nil {
		return nil, fmt.Errorf("prepare upload Cookies: %w", opts.cookieErr)
	}

	// 根据加密类型加载默认header和cookie值。
	def := c.defHeader.HeaderItemForCryptoMode(opts.CryptoMode)
	if def == nil {
		return nil, fmt.Errorf("%s crypto mode unknown", opts.CryptoMode)
	}

	uri, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	requestHeaders := mergeRequestHeaders(def.Header, opts.Headers)
	cookieURL := cookieURLForHost(uri, requestHeaders.Get("Host"))

	policy, cookieErr := newRequestCookiePolicy(cookieURL, opts.cookieSnapshot(), c.cookieJar.Cookies(cookieURL))
	if cookieErr != nil {
		return nil, fmt.Errorf("prepare upload Cookies: %w", cookieErr)
	}

	if cookieErr := policy.setDefaultCookies(def.Cookie); cookieErr != nil {
		return nil, fmt.Errorf("prepare upload Cookies: %w", cookieErr)
	}

	if cookieErr := policy.setDefaultIfMissing("WNMCID", _wnmcid); cookieErr != nil {
		return nil, fmt.Errorf("prepare upload Cookies: %w", cookieErr)
	}

	if cookieErr := policy.setDefaultIfMissing("deviceId", _deviceId); cookieErr != nil {
		return nil, fmt.Errorf("prepare upload Cookies: %w", cookieErr)
	}

	policy.deleteDefault("MUSIC_A")

	if _, hasMusicU := policy.cookieScalar("MUSIC_U", "MUSIC_R_U"); !hasMusicU {
		anonymous := ""
		if c.anonymous != nil {
			anonymous = c.anonymous.Get()
		}

		if anonymous == "" {
			anonymous = def.GetCookie("MUSIC_A")
		}

		if cookieErr := policy.setDefaultCookie("MUSIC_A", anonymous); cookieErr != nil {
			return nil, fmt.Errorf("prepare upload Cookies: %w", cookieErr)
		}
	}

	policy.finalize(protocolCookieNames)

	var (
		body            any = data
		excludedHeaders     = []string{"User-Agent", "Cookie"}
		ua                  = requestHeaders.Get("User-Agent")
	)

	if bar != nil {
		body = bar.NewProxyReader(data)
	}

	request := c.cli.R().
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "*/*").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", ua).
		SetBody(body)

	setRequestHeaders(request, requestHeaders, excludedHeaders)
	request.Header.Del("Cookie")
	request.SetCookies(policy.options)
	request.SetContext(withRequestCookiePolicy(ctx, policy))

	var response *resty.Response

	switch opts.Method {
	case http.MethodPost, "":
		response, err = request.Post(url)
	case http.MethodPut:
		response, err = request.Put(url)
	default:
		return nil, fmt.Errorf("%s not surpport http method", opts.Method)
	}

	if err != nil {
		return nil, fmt.Errorf("do upload: %w", err)
	}

	c.l.Debugf("[upload.response] method=%s url=%s status=%d bytes=%d", opts.Method, url, response.StatusCode(), len(response.Body()))

	if resp != nil {
		decode := json.NewDecoder(bytes.NewReader(response.Body()))
		if err := decode.Decode(&resp); err != nil {
			return nil, newAPIError(response.StatusCode(), fmt.Errorf("json.NewDecoder: %w", err))
		}
	}

	if response.StatusCode() != http.StatusOK {
		return nil, newAPIError(response.StatusCode(), nil)
	}

	return response, nil
}

// Download streams the response body into resp and closes it before returning.
// The returned response is metadata-only; callers must not read response.Body.
func (c *Client) Download(ctx context.Context, url string, headers map[string]string, reqBody io.Reader, resp io.Writer, bar *pb.ProgressBar) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("download context is nil")
	}

	if resp == nil {
		return nil, errors.New("download writer is nil")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, reqBody)
	if err != nil {
		if reqBody != nil {
			if closer, ok := reqBody.(io.Closer); ok {
				_ = closer.Close()
			}
		}
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
	client := *c.cli.GetClient()
	client.Timeout = 0

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			c.l.Errorf("close download response body: %v", closeErr)
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

	if response.ContentLength >= 0 && n != response.ContentLength {
		return nil, errors.New("file transfer interrupted")
	}
	return response, nil
}

// getCookieValueByURL returns the first non-empty matching Cookie value.
func (c *Client) getCookieValueByURL(uri *neturl.URL, names ...string) string {
	if uri == nil || c.cookieJar == nil || len(names) == 0 {
		return ""
	}

	cookies := c.cookieJar.Cookies(uri)
	for _, name := range names {
		for _, ck := range cookies {
			if ck != nil && ck.Name == name && ck.Value != "" {
				return unescapeCookieValue(ck.Value)
			}
		}
	}
	return ""
}

func mergeRequestHeaders(defaults, user http.Header) http.Header {
	merged := make(http.Header, len(defaults)+len(user))
	for layer, source := range []http.Header{defaults, user} {
		names := make([]string, 0, len(source))
		for name := range source {
			names = append(names, name)
		}

		slices.Sort(names)

		for _, name := range names {
			values := source[name]
			if layer == 0 && len(values) == 0 {
				continue
			}

			merged[http.CanonicalHeaderKey(name)] = slices.Clone(values)
		}
	}
	return merged
}

func setRequestHeaders(request *resty.Request, headers http.Header, excluded []string) {
	exclude := make(map[string]struct{}, len(excluded))
	for _, name := range excluded {
		exclude[http.CanonicalHeaderKey(name)] = struct{}{}
	}

	for key, values := range headers {
		if _, ok := exclude[key]; !ok {
			request.Header[key] = slices.Clone(values)
		}
	}
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

func setHeader(req *resty.Request, name string, exist bool, value string) {
	if !exist {
		return
	}

	req.Header.Set(name, value)
}

func unescapeCookieValue(value string) string {
	if value == "" {
		return ""
	}

	if unescaped, err := neturl.QueryUnescape(value); err == nil {
		return unescaped
	}
	return value
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
