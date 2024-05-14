// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

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
	"net/http/httputil"
	neturl "net/url"
	"time"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/go-resty/resty/v2"
	"github.com/google/brotli/go/cbrotli"
)

type Config struct {
	Debug   bool                       `json:"debug" yaml:"debug"`
	Timeout time.Duration              `json:"timeout" yaml:"timeout"`
	Retry   int                        `json:"retry" yaml:"retry"`
	Cookie  cookie.PersistentJarConfig `json:"cookie" yaml:"cookie"`
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
	cfg    *Config
	cli    *resty.Client
	cookie *cookie.PersistentJar
	l      *log.Logger
	// agent  *Agent
}

func New(cfg *Config) *Client {
	client, err := NewWithErr(cfg, log.Default)
	if err != nil {
		panic(err)
	}
	return client
}

func NewWithErr(cfg *Config, l *log.Logger) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	var opts = []cookie.PersistentJarOption{
		cookie.WithSyncInterval(cfg.Cookie.Interval),
	}
	if cfg.Cookie.Filepath != "" {
		opts = append(opts, cookie.WithFilePath(cfg.Cookie.Filepath))
	}
	if opt := cfg.Cookie.Options; opt != nil && opt.PublicSuffixList != nil {
		opts = append(opts, cookie.WithPublicSuffixList(cfg.Cookie.PublicSuffixList))
	}
	jar, err := cookie.NewPersistentJar(opts...)
	if err != nil {
		return nil, fmt.Errorf("NewPersistentJar: %w", err)
	}

	cli := resty.New()
	cli.SetRetryCount(cfg.Retry)
	cli.SetTimeout(cfg.Timeout)
	cli.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	cli.SetDebug(cfg.Debug)
	cli.SetCookieJar(jar)
	cli.OnAfterResponse(dump)
	// cli.OnAfterResponse(decrypt)
	cli.OnAfterResponse(contentEncoding)
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

func (c *Client) Cookie(url, name string) (http.Cookie, bool) {
	uri, err := neturl.Parse(url)
	if err != nil {
		log.Warn("cookie parse(%v) err: ", url, err)
		return http.Cookie{}, false
	}
	for _, c := range c.cookie.Cookies(uri) {
		if c.Name == name {
			return *c, true
		}
	}
	return http.Cookie{}, false
}

func (c *Client) GetCSRF(url string) (string, bool) {
	uri, err := neturl.Parse(url)
	if err != nil {
		log.Warn("GetCSRF parse(%v) err: ", url, err)
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

func (c *Client) Request(ctx context.Context, method, url, cryptoMode string, req, reply interface{}) (*resty.Response, error) {
	var (
		encryptData map[string]string
		err         error
		csrf        string
		resp        *resty.Response
	)

	uri, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	csrf, has := c.GetCSRF(url)
	if !has {
		log.Debug("get csrf token not found")
	}

	request := c.cli.R().
		SetContext(ctx).
		SetHeader("Host", "music.163.com").
		SetHeader("Connection", "keep-alive").
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept-language", "zh-CN,zh-Hans;q=0.9").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034")

	switch cryptoMode {
	case "eapi":
		encryptData, err = crypto.EApiEncrypt(uri.Path, req)
		if err != nil {
			return nil, fmt.Errorf("EApiEncrypt: %w", err)
		}
	case "weapi":
		request.SetQueryParam("csrf_token", csrf)
		encryptData, err = crypto.WeApiEncrypt(req)
		if err != nil {
			return nil, fmt.Errorf("EApiEncrypt: %w", err)
		}
	case "linux":
		encryptData, err = crypto.LinuxApiEncrypt(req)
		if err != nil {
			return nil, fmt.Errorf("LinuxApiEncrypt: %w", err)
		}
	default:
		return nil, fmt.Errorf("%s crypto mode unknown", cryptoMode)
	}
	log.Debug("data: %+v\nencriypt: %+v\n", req, encryptData)

	switch method {
	case "POST":
		resp, err = request.SetFormData(encryptData).Post(url)
	case "GET":
		resp, err = request.Get(url)
	default:
		return nil, fmt.Errorf("%s not surpport http method", method)
	}
	log.Debug("response: %+v\n", string(resp.Body()))

	var decryptData []byte
	switch cryptoMode {
	case "eapi":
		decryptData, err = crypto.EApiDecrypt(string(resp.Body()), "")
		if err != nil {
			return nil, fmt.Errorf("EApiDecrypt: %w", err)
		}
	case "weapi":
		// tips: weapi接口返回数据是明文,另外即使是加密数据也拿不到私钥.
		decryptData = resp.Body()
	case "linux":
		decryptData, err = crypto.LinuxApiDecrypt(string(resp.Body()))
		if err != nil {
			return nil, fmt.Errorf("LinuxApiDecrypt: %w", err)
		}
	}
	log.Debug("decrypt body:%s\n", string(decryptData))
	if err := json.Unmarshal(decryptData, &reply); err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	}
	return resp, nil
}

func (c *Client) NeedLogin(ctx context.Context) bool {
	// todo:
	return false
}

func encrypt(c *resty.Client, req *resty.Request) error {
	u, err := neturl.Parse(req.URL)
	if err != nil {
		return err
	}
	data, err := crypto.EApiEncrypt(u.Path, req.Body)
	if err != nil {
		return fmt.Errorf("EApiEncrypt: %w", err)
	}
	log.Debug("data: %+v\nencriypt: %+v\n", req.Body, data)
	return nil
}

func decrypt(c *resty.Client, resp *resty.Response) error {
	raw, err := crypto.EApiDecrypt(string(resp.Body()), "")
	if err != nil {
		return fmt.Errorf("EApiDecrypt: %w", err)
	}
	log.Debug("raw: %+v\n", string(raw))
	if err := json.Unmarshal(raw, resp.Result()); err != nil {
		return err
	}
	return nil
}

func contentEncoding(c *resty.Client, resp *resty.Response) error {
	kind := resp.Header().Get("Content-Encoding")
	log.Debug("Uncompressed: %v\n", resp.RawResponse.Uncompressed)
	switch kind {
	case "deflate":
		// 为何使用zlib库: https://zlib.net/zlib_faq.html#faq39
		data, err := zlib.NewReader(bytes.NewReader(resp.Body()))
		if err != nil {
			return err
		}
		defer data.Close()
		bodyBytes, err := io.ReadAll(data)
		if err != nil {
			return err
		}
		resp.SetBody(bodyBytes)
		// reader:=flate.NewReader(bytes.NewReader(resp.Body()))
		// defer reader.Close()
		// bodyBytes, err := io.ReadAll(reader)
		// if err != nil {
		// 	return err
		// }
		// resp.SetBody(bodyBytes)
	case "gzip":
		// TODO: restry 自身已经实现gzip解压缩
		// reader, err := gzip.NewReader(bytes.NewReader(resp.Body()))
		// if err != nil {
		// 	return err
		// }
		// defer reader.Close()
		// bodyBytes, err := io.ReadAll(reader)
		// if err != nil {
		// 	return err
		// }
		// resp.SetBody(bodyBytes)
	case "br":
		bodyBytes, err := cbrotli.Decode(resp.Body())
		if err != nil {
			return err
		}
		resp.SetBody(bodyBytes)
	case "":
		// 空则代表是gzip,golang底层会做相应得解压缩处理,为空得原因是,
		// 收到请求后进行解压, 同时删除 Content-Encoding: gzip请求头。
		// 如果想关闭自动解压缩,则可以设置Transport.DisableCompression=true
	default:
		return fmt.Errorf("not supported yet Content-Encoding: %s", kind)
	}
	return nil
}

func dump(c *resty.Client, resp *resty.Response) error {
	d, err := io.ReadAll(resp.RawBody())
	if err != nil {
		return fmt.Errorf("ReadAll: %w", err)
	}
	log.Debug("rawbody:%s\n", string(d))
	log.Debug("----body:%s\n", string(resp.Body()))

	resp.RawResponse.Body = io.NopCloser(bytes.NewReader(resp.Body()))
	log.Debug("############### http dump ################")
	dumpReq, err := httputil.DumpRequest(resp.Request.RawRequest, true)
	if err != nil {
		return fmt.Errorf("DumpRequest: %w", err)
	}
	log.Debug("---------------- request ----------------\n%s", string(dumpReq))

	dumpResp, err := httputil.DumpResponse(resp.RawResponse, true)
	if err != nil {
		return fmt.Errorf("DumpResponse: %w", err)
	}
	log.Debug("---------------- response ----------------\n%s\n", string(dumpResp))
	log.Debug("resp body byte: %v\n", resp.Body())
	return nil
}
