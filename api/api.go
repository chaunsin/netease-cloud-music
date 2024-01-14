package api

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	neturl "net/url"

	"github.com/chaunsin/netease-cloud-music/config"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"

	"github.com/go-resty/resty/v2"
	"github.com/google/brotli/go/cbrotli"
)

type Client struct {
	cfg    *config.Config
	cli    *resty.Client
	cookie *cookie.PersistentJar
	// agent  *Agent
}

func New(cfg *config.Config) *Client {
	client, err := NewWithErr(cfg)
	if err != nil {
		panic(err)
	}
	return client
}

func NewWithErr(cfg *config.Config) (*Client, error) {
	if err := cfg.Valid(); err != nil {
		return nil, fmt.Errorf("valid: %w", err)
	}

	var opts = []cookie.PersistentJarOption{
		cookie.WithSyncInterval(cfg.Network.Cookie.Interval),
	}
	if cfg.Network.Cookie.Filepath != "" {
		opts = append(opts, cookie.WithFilePath(cfg.Network.Cookie.Filepath))
	}
	if cfg.Network.Cookie.PublicSuffixList != nil {
		opts = append(opts, cookie.WithPublicSuffixList(cfg.Network.Cookie.PublicSuffixList))
	}
	jar, err := cookie.NewPersistentJar(opts...)
	if err != nil {
		return nil, fmt.Errorf("NewPersistentJar: %w", err)
	}

	cli := resty.New()
	cli.SetRetryCount(cfg.Network.Retry)
	cli.SetTimeout(cfg.Network.Timeout)
	cli.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	cli.SetDebug(cfg.Network.Debug)
	cli.SetCookieJar(jar)
	cli.OnAfterResponse(dump)
	// cli.SetDebug(true)
	// cli.OnAfterResponse(decrypt)
	cli.OnAfterResponse(contentEncoding)
	// cli.OnBeforeRequest(encrypt)

	// c.cli.SetLogger(log.Logger)
	// c.cli.AddRetryHook(func(resp *resty.Response, err error) {
	// 	log.Logger.Warnf("URL:%s,RetryCount:%d,RequestBody:%+v StatusCode:%d,ResponseBody:%s CusumeTime:%s Err:%s",
	// 		resp.Request.URL, resp.Request.Attempt, resp.Request.Body, resp.StatusCode(), resp.Body(), resp.Time(), err)
	// })

	c := Client{
		cfg:    cfg,
		cli:    cli,
		cookie: jar,
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

func (c *Client) Request(ctx context.Context, method, url, cryptoMode string, req, reply interface{}) (*resty.Response, error) {
	var (
		encryptData map[string]string
		err         error
		csrf        string
	)

	uri, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}
	for _, c := range c.cookie.Cookies(uri) {
		if c.Name == "__crsf_token" {
			csrf = c.Value
			break
		}
	}

	var resp *resty.Response
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
	fmt.Printf("data: %+v\nencriypt: %+v\n", req, encryptData)

	switch method {
	case "POST":
		resp, err = request.SetFormData(encryptData).Post(url)
	case "GET":
		resp, err = request.Get(url)
	default:
		return nil, fmt.Errorf("%s not surpport http method", method)
	}
	fmt.Printf("response: %+v\n", string(resp.Body()))

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
	fmt.Printf("decrypt body:%s\n", string(decryptData))
	if err := json.Unmarshal(decryptData, &reply); err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	}
	return resp, nil
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
	fmt.Printf("data: %+v\nencriypt: %+v\n", req.Body, data)
	return nil
}

func decrypt(c *resty.Client, resp *resty.Response) error {
	raw, err := crypto.EApiDecrypt(string(resp.Body()), "")
	if err != nil {
		return fmt.Errorf("EApiDecrypt: %w", err)
	}
	fmt.Printf("raw: %+v\n", string(raw))
	if err := json.Unmarshal(raw, resp.Result()); err != nil {
		return fmt.Errorf("AAAAA:%w", err)
	}
	return nil
}

func contentEncoding(c *resty.Client, resp *resty.Response) error {
	kind := resp.Header().Get("Content-Encoding")
	fmt.Printf("Uncompressed: %v\n", resp.RawResponse.Uncompressed)
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
	fmt.Printf("rawbody:%s\n", string(d))
	fmt.Printf("----body:%s\n", string(resp.Body()))

	resp.RawResponse.Body = io.NopCloser(bytes.NewReader(resp.Body()))
	fmt.Println("############### http dump ################")
	dumpReq, err := httputil.DumpRequest(resp.Request.RawRequest, true)
	if err != nil {
		return fmt.Errorf("DumpRequest: %w", err)
	}
	fmt.Printf("---------------- request ----------------\n%s", string(dumpReq))

	dumpResp, err := httputil.DumpResponse(resp.RawResponse, true)
	if err != nil {
		return fmt.Errorf("DumpResponse: %w", err)
	}
	fmt.Printf("---------------- response ----------------\n%s\n", string(dumpResp))
	fmt.Printf("resp body byte: %v\n", resp.Body())
	return nil
}
