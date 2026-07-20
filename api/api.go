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
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/cheggaaa/pb/v3"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/singleflight"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type Config struct {
	Debug   bool          `json:"debug" yaml:"debug"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	Retry   int           `json:"retry" yaml:"retry"`
	Cookie  cookie.Config `json:"cookie" yaml:"cookie"`
	// Agent   *Agent                     `json:"agent" yaml:"agent"`
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
	cfg          *Config
	cli          *resty.Client
	cookie       *cookie.Cookie
	l            *log.Logger
	xeapiMu      sync.Mutex
	xeapiRefresh singleflight.Group
	xeapiKey     crypto.PublicKeyState
	xeapiSession crypto.Session
	// agent  *Agent
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

	opts := []cookie.Option{
		cookie.WithSyncInterval(cfg.Cookie.Interval),
	}
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
	// cli.OnBeforeRequest(encrypt)
	// cli.SetLogger(l)
	// cli.AddRetryHook(func(resp *resty.Response, err error) {
	// 	l.Warnf("URL:%s,RetryCount:%d,RequestBody:%+v StatusCode:%d,ResponseBody:%s CusumeTime:%s Err:%s",
	// 		resp.Request.URL, resp.Request.Attempt, resp.Request.Body, resp.StatusCode(), resp.Body(), resp.Time(), err)
	// })

	c := Client{
		cfg:    cfg,
		cli:    cli,
		cookie: jar,
		l:      l,
		// agent:  NewAgent(),
	}
	return &c, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	c.cli.SetCloseConnection(true)
	return c.cookie.Close(ctx)
}

func (c *Client) NewRequest() *resty.Request {
	return c.cli.NewRequest()
}

func (c *Client) GetClient() *http.Client {
	return c.cli.GetClient()
}

// Cookie 根据url和cookie name获取cookie.
func (c *Client) Cookie(url, name string) (http.Cookie, bool) {
	uri, err := neturl.Parse(url)
	if err != nil {
		log.Warnf("cookie parse(%v) err: %s", url, err)
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

// GetCSRF 获取csrf 一般用于weapi接口中使用.
func (c *Client) GetCSRF(url string) (string, bool) {
	uri, err := neturl.Parse(url)
	if err != nil {
		log.Warnf("GetCSRF parse(%v) err: %s", url, err)
		return "", false
	}

	for _, c := range c.cookie.Cookies(uri) {
		if c.Name == "__csrf_token" && c.Value != "" {
			return c.Value, true
		}

		if c.Name == "__csrf" && c.Value != "" {
			return c.Value, true
		}
	}
	return "", false
}

// GetDeviceId 从当前客户端的 Cookie 中获取设备 ID.
func (c *Client) GetDeviceId() string {
	for _, name := range []string{"deviceId", "sDeviceId"} {
		for _, cookieURL := range []string{
			"https://music.163.com",
			"https://interface.music.163.com",
			"https://interface3.music.163.com",
		} {
			if ck, ok := c.Cookie(cookieURL, name); ok && ck.Value != "" {
				return ck.Value
			}
		}
	}
	return ""
}

// Request 接口请求.
func (c *Client) Request(ctx context.Context, url string, req, resp any, opts *Options) (*resty.Response, error) {
	if url == "" || req == nil || resp == nil {
		return nil, errors.New("request args invalid")
	}

	if opts == nil {
		opts = NewOptions()
	}

	if opts.Method == "" {
		opts.SetMethod(http.MethodPost)
	}

	var (
		encryptData map[string]string
		err         error
		response    *resty.Response
		requestURL  = url
	)

	uri, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	// Pending: set User-Agent config

	request := c.cli.R().
		SetContext(ctx).
		SetHeader("Host", "music.163.com").
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept-language", "zh-CN,zh-Hans;q=0.9").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034").
		SetCookie(&http.Cookie{Name: "__remember_me", Value: "true", Domain: ""})
	// SetHeader("User-Agent", "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.1 Chrome/121.0.0.0 Mobile Safari/537.36")

	switch opts.CryptoMode {
	case CryptoModeEAPI:
		// Pending: set common params
		// var dataHeader = http.Header{}
		// dataHeader.Add("osver", getCookie(options.cookies, "osver"))
		// dataHeader.Add("deviceId", getCookie(options.cookies, "deviceId"))
		// dataHeader.Add("appver", getCookie(options.cookies, "appver", "6.1.1"))
		// dataHeader.Add("versioncode", getCookie(options.cookies, "versioncode", "140"))
		// dataHeader.Add("mobilename", getCookie(options.cookies, "mobilename"))
		// dataHeader.Add("buildver", getCookie(options.cookies, "buildver"))
		// dataHeader.Add("resolution", getCookie(options.cookies, "resolution", "1920x1080"))
		// dataHeader.Add("__csrf", getCookie(options.cookies, "__csrf"))
		// dataHeader.Add("os", getCookie(options.cookies, "os", "android"))
		// dataHeader.Add("channel", getCookie(options.cookies, "channel"))
		// dataHeader.Add("requestId", fmt.Sprintf("%d_%04d", time.Now().UnixNano()/1000000, r.Intn(1000)))
		// if c := getCookie(options.cookies, "MUSIC_U"); c != "" {
		// 	dataHeader.Add("MUSIC_U", c)
		// }
		// if c := getCookie(options.cookies, "MUSIC_A"); c != "" {
		// 	dataHeader.Add("MUSIC_A", c)
		// }
		// req.Header.Set("Cookie", "")
		// for k, v := range dataHeader {
		// 	req.AddCookie(&http.Cookie{
		// 		Name:  k,
		// 		Value: v[0],
		// 	})
		// }
		// data["header"] = dataHeader
		encryptData, err = crypto.EApiEncrypt(uri.Path, req)
		if err != nil {
			return nil, fmt.Errorf("EApiEncrypt: %w", err)
		}
	case CryptoModeWEAPI:
		// Pending: 需要替换？因为有些 https://interface.music.163.com/api 得接口也会走这个逻辑
		// reg, _ := regexp.Compile(`\w*api`)
		// url = reg.ReplaceAllString(url, "weapi")
		// url = strings.ReplaceAll(url, "api", "weapi")
		csrf, has := c.GetCSRF(url)
		if !has {
			log.Debugf("get csrf token not found")
		}

		request.SetQueryParam("csrf_token", csrf)

		// // request.SetCookie(&http.Cookie{Name: "appver", Value: "2.3.17"})
		// request.SetCookie(&http.Cookie{Name: "appver", Value: "9.0.95"})
		// // request.SetCookie(&http.Cookie{Name: "os", Value: "osx"})
		// request.SetCookie(&http.Cookie{Name: "os", Value: "android"})
		// // request.SetCookie(&http.Cookie{Name: "deviceId", Value: "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B"})
		// // request.SetCookie(&http.Cookie{Name: "WEVNSM", Value: "1.0.0"})
		// // request.SetCookie(&http.Cookie{Name: "channel", Value: "netease"})
		// // request.SetHeader("nm-gcore-status", "1")
		// request.SetHeader("appver", "9.0.95")
		// request.SetHeader("os", "android")

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
		requestURL, encryptData, err = c.xeapiEncrypt(ctx, url, req, opts, request.Header.Get("Content-Type"))
		if err != nil {
			return nil, fmt.Errorf("xeapiEncrypt: %w", err)
		}

		request.SetHeader("User-Agent", xeapiUserAgent(opts))
		request.SetHeader("X-Client-Enc-State", "ENCRYPTED")

		if xeapiURI, parseErr := neturl.Parse(requestURL); parseErr == nil && xeapiURI.Host != "" {
			request.SetHeader("Host", xeapiURI.Host)
		}
	case CryptoModeAPI:
		// 不需要加密处理请求
		// Pending: 待处理,在/api/xx/接口请求时则不需要参数加密处理,此处需要对结构体转换成map[string]string类型
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

	log.Debugf("[request] method=%s crypto=%s url=%s payload_type=%T encrypted_fields=%d",
		opts.Method, opts.CryptoMode, requestURL, req, len(encryptData))

	// append user options config
	if len(opts.Headers) > 0 {
		request = request.SetHeaders(opts.Headers)
	}

	if len(opts.Cookies) > 0 {
		request = request.SetCookies(opts.Cookies)
	}

	switch opts.Method {
	case http.MethodPost:
		response, err = request.SetFormData(encryptData).Post(requestURL)
	case http.MethodGet:
		response, err = request.Get(requestURL)
	default:
		return nil, fmt.Errorf("%s not surpport http method", opts.Method) // Pending: 需要适配PUT等方法
	}

	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	log.Debugf("[response.raw] status=%d bytes=%d", response.StatusCode(), len(response.Body()))

	var decryptData []byte

	switch opts.CryptoMode {
	case CryptoModeAPI:
		// tips: api接口返回数据是明文
		decryptData = response.Body()
	case CryptoModeEAPI:
		// Pending: 貌似eapi接口返回数据是否是是明文,跟传入参数e_r: true有关,true为加密，false为明文。此处考虑采用反射req中得字段处理。
		// see: https://gitlab.com/Binaryify/neteasecloudmusicapi/-/commit/58e9865b70e41197c2ab75c46a775fc45d6efa6e
		// decryptData, err = crypto.EApiDecrypt(string(response.Body()), "")
		// if err != nil {
		// 	return nil, fmt.Errorf("EApiDecrypt: %w", err)
		// }
		decryptData = response.Body()
		log.Debugf("[response.decrypt] crypto=%s bytes=%d", opts.CryptoMode, len(decryptData))
	case CryptoModeWEAPI:
		// tips: weapi接口返回数据是明文
		decryptData = response.Body()
	case CryptoModeLinux:
		decryptData, err = crypto.LinuxApiDecrypt(string(response.Body()))
		if err != nil {
			return nil, fmt.Errorf("LinuxApiDecrypt: %w", err)
		}

		log.Debugf("[response.decrypt] crypto=%s bytes=%d", opts.CryptoMode, len(decryptData))
	case CryptoModeXEAPI:
		c.updateXeapiSession(response)

		decryptData, err = crypto.XeapiDecryptResponse(response.Body())
		if err != nil {
			return nil, fmt.Errorf("XeapiDecryptResponse: %w", err)
		}

		log.Debugf("[response.decrypt] crypto=%s bytes=%d", opts.CryptoMode, len(decryptData))
	default:
		return nil, fmt.Errorf("%s crypto mode unknown", opts.CryptoMode)
	}

	decode := json.NewDecoder(bytes.NewReader(decryptData))
	// decode.DisallowUnknownFields()
	if err := decode.Decode(&resp); err != nil {
		return nil, fmt.Errorf("json.NewDecoder: %w", err)
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d response bytes: %d", response.StatusCode(), len(decryptData))
	}
	return response, nil
}

func (c *Client) Upload(ctx context.Context, url string, headers map[string]string, data io.Reader, resp any, bar *pb.ProgressBar) (*resty.Response, error) {
	var body any = data
	if bar != nil {
		body = bar.NewProxyReader(data)
	}

	response, err := c.cli.R().
		SetContext(ctx).
		SetHeaders(headers).
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "*/*").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034").
		SetBody(body).
		Post(url)
	if err != nil {
		return nil, err
	}

	log.Debugf("[upload.response] status=%d bytes=%d", response.StatusCode(), len(response.Body()))

	if err := json.Unmarshal(response.Body(), &resp); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
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
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034")
	request.Header.Set("Range", "bytes=0-")

	for k, v := range headers {
		request.Header.Set(k, v)
	}

	response, err := c.cli.GetClient().Do(request)
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
