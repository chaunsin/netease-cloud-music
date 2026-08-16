// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/singleflight"
	"gopkg.in/yaml.v3"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

const (
	defaultXeapiUserAgent = "NeteaseMusic/9.2.85.250418145357(9002085);Dalvik/2.1.0 (Linux; U; Android 9; SM-S9180 Build/PQ3B.190801.10101846)"
	xeapiPublicKeyRoute   = "/api/gorilla/anti/crawler/security/key/get"
	xeapiPublicKeyURL     = "https://interface.music.163.com/eapi/gorilla/anti/crawler/security/key/get"
)

type xeapiRefreshPublicKeyResp struct {
	Code int `json:"code"`
	Data struct {
		EncryptedData string          `json:"encryptedData"` // plaintext is crypto.XeapiPublicKeyState
		Signature     string          `json:"signature"`
		Timestamp     json.RawMessage `json:"timestamp"`
	} `json:"data"`
	Message string `json:"message"`
}

type xeapiStateResult struct {
	PublicKeyState crypto.XeapiPublicKeyState `json:"publicKeyState" yaml:"publicKeyState"`
	Session        crypto.XeapiSession        `json:"session" yaml:"session"`
}

// xeapiStateSnapshot 将公钥和会话快照绑定到本地公钥修订号。
type xeapiStateSnapshot struct {
	xeapiStateResult

	keyRevision uint64
}

// xeapiRequestRevision 记录提交响应会话所需的乐观并发版本：公钥修订号
// 必须仍然匹配，且同一修订号内只允许较新的请求响应覆盖会话。
type xeapiRequestRevision struct {
	keyRevision     uint64 // 请求使用的本地公钥修订号，不是服务端的 PublicKeyState.Version。
	requestSequence uint64 // 请求发送前分配的单调序号，用来裁决并发响应的先后。
}

type xeapi struct {
	cli                     *resty.Client
	mux                     sync.Mutex
	group                   singleflight.Group
	storePath               string // 持久化 xeapiStateResult 存储位置
	keyRevision             uint64 // 本地乐观锁修订号；替换公钥时递增，使旧响应失效。
	requestSequence         uint64 // 已完成加密的请求序号；数值越大，请求越新。
	acceptedSessionSequence uint64 // 当前会话来源请求的序号；作为拒绝旧响应的高水位。
	xeapiStateResult               //nolint:embeddedstructfieldcheck // 不做检查
}

func newXeapi(cli *resty.Client, storePath string) *xeapi {
	return &xeapi{cli: cli, storePath: storePath}
}

// Sync 对内存中得公钥相关数据持久到磁盘中。
func (x *xeapi) Sync() error {
	file := x.storePath
	if file == "" {
		file = utils.BaseDir("xeapi.yaml")
	}

	x.mux.Lock()
	state := x.xeapiStateResult
	x.mux.Unlock()

	// 不完整的状态不能用于刷新，避免覆盖已有的可用缓存。
	if !xeapiPublicKeyComplete(state.PublicKeyState) {
		return nil
	}

	var marshal func(v any) ([]byte, error)

	switch ext := filepath.Ext(x.storePath); ext {
	case ".json":
		marshal = json.Marshal
	case ".yaml", ".yml":
		marshal = yaml.Marshal
	default:
		marshal = yaml.Marshal
	}

	data, err := marshal(state)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("MkdirAll: %w", err)
	}

	if err = os.WriteFile(file, data, 0o600); err != nil {
		return fmt.Errorf("xeapi write %s err: %w", file, err)
	}
	return nil
}

func (x *xeapi) LoadConfig() error {
	data, err := os.ReadFile(x.storePath)
	if err != nil {
		return fmt.Errorf("read xeapi public token err: %w", err)
	}

	var cfg xeapiStateResult

	switch ext := filepath.Ext(x.storePath); ext {
	case ".json":
		err = json.Unmarshal(data, &cfg)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &cfg)
	default:
		return fmt.Errorf("not support file ext type'%s'", ext)
	}

	if err != nil {
		return fmt.Errorf("unmarshal err: %w", err)
	}

	if !xeapiPublicKeyComplete(cfg.PublicKeyState) {
		return errors.New("public key state is incomplete")
	}

	cfg.Session.ID, cfg.Session.Key = strings.TrimSpace(cfg.Session.ID), strings.TrimSpace(cfg.Session.Key)
	if err := cfg.Session.Validate(); err != nil {
		return err
	}

	x.mux.Lock()
	x.xeapiStateResult = cfg
	x.keyRevision++
	x.acceptedSessionSequence = 0
	x.mux.Unlock()
	return nil
}

func (x *xeapi) Encrypt(ctx context.Context, req *crypto.XeapiEncryptRequest) (map[string]string, xeapiRequestRevision, error) {
	result, err := x.xeapiState(ctx, req)
	if err != nil {
		return nil, xeapiRequestRevision{}, fmt.Errorf("xeapiState: %w", err)
	}

	encryptData, err := crypto.XeapiEncrypt(req, result.PublicKeyState, result.Session)
	if err != nil {
		return nil, xeapiRequestRevision{}, fmt.Errorf("XeapiEncrypt: %w", err)
	}

	x.mux.Lock()
	x.requestSequence++
	revision := xeapiRequestRevision{
		keyRevision:     result.keyRevision,
		requestSequence: x.requestSequence,
	}
	x.mux.Unlock()

	return encryptData, revision, nil
}

func (x *xeapi) xeapiState(ctx context.Context, req *crypto.XeapiEncryptRequest) (*xeapiStateSnapshot, error) {
	x.mux.Lock()

	var (
		key          = x.PublicKeyState
		session      = x.Session
		keyRevision  = x.keyRevision
		needsRefresh = xeapiKeyNeedsRefresh(key, req.DeviceID) // 暂时采用严格模式当公钥过期或设备id不一致时则需要刷新token。
		groupKey     = strings.Join([]string{req.DeviceID, req.AppVersion, req.OS, req.UserAgent}, "\x00")
	)
	x.mux.Unlock()

	if !needsRefresh {
		return &xeapiStateSnapshot{
			xeapiStateResult: xeapiStateResult{PublicKeyState: key, Session: session},
			keyRevision:      keyRevision,
		}, nil
	}

	value, err, _ := x.group.Do(groupKey, func() (any, error) {
		x.mux.Lock()

		var (
			key            = x.PublicKeyState
			session        = x.Session
			keyRevision    = x.keyRevision
			needsRefresh   = xeapiKeyNeedsRefresh(key, req.DeviceID)
			currentVersion = key.Version
			currentSK      = key.SK
		)
		x.mux.Unlock()

		if !needsRefresh {
			return &xeapiStateSnapshot{
				xeapiStateResult: xeapiStateResult{PublicKeyState: key, Session: session},
				keyRevision:      keyRevision,
			}, nil
		}

		refreshed, err := x.refreshPublicKey(ctx, req, currentVersion)
		if err != nil {
			return nil, fmt.Errorf("refreshXeapiPublicKey: %w", err)
		}

		if strings.TrimSpace(refreshed.SK) == "" {
			refreshed.SK = currentSK
		}

		if err := refreshed.Validate(); err != nil {
			return nil, err
		}

		x.mux.Lock()
		// 新公钥不能沿用旧会话，同时使刷新前创建的响应修订号全部失效。
		x.PublicKeyState = *refreshed
		x.Session = crypto.XeapiSession{}
		x.keyRevision++
		x.acceptedSessionSequence = 0
		keyRevision = x.keyRevision
		x.mux.Unlock()

		return &xeapiStateSnapshot{
			xeapiStateResult: xeapiStateResult{PublicKeyState: *refreshed},
			keyRevision:      keyRevision,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	result, ok := value.(*xeapiStateSnapshot)
	if !ok {
		return nil, fmt.Errorf("unexpected xeapi state result %T", value)
	}
	return result, nil
}

func (x *xeapi) refreshPublicKey(ctx context.Context, req *crypto.XeapiEncryptRequest, currentKeyVersion string) (*crypto.XeapiPublicKeyState, error) {
	nonce, err := generateXeapiNonce()
	if err != nil {
		return nil, fmt.Errorf("generateXeapiNonce: %w", err)
	}

	osName := strings.TrimSpace(req.OS)
	if osName == "" {
		osName = "android"
	}

	var (
		timestamp = strconv.FormatInt(time.Now().UnixMilli(), 10)
		payload   = map[string]any{
			"appVersion":        req.AppVersion,
			"currentKeyVersion": currentKeyVersion,
			"deviceId":          req.DeviceID,
			"e_r":               true,
			"header":            "{}",
			"nonce":             nonce,
			"os":                osName,
			"requestType":       "active",
			"signature":         crypto.XeapiSign(timestamp, nonce),
			"timestamp":         timestamp,
			"t1":                req.T1,
			"t2":                req.T2,
			"uid":               req.UID,
		}
	)

	form, err := crypto.EApiEncrypt(xeapiPublicKeyRoute, payload)
	if err != nil {
		return nil, fmt.Errorf("EApiEncrypt xeapi public key request: %w", err)
	}

	request := x.cli.R().
		SetContext(ctx).
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Connection", "keep-alive").
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", req.UserAgent).
		SetHeader("x-aeapi", "true").
		SetFormData(form)
	if strings.TrimSpace(req.DeviceID) != "" {
		request.SetCookie(&http.Cookie{Name: "deviceId", Value: req.DeviceID})
	}

	response, err := request.Post(xeapiPublicKeyURL)
	if err != nil {
		return nil, fmt.Errorf("xeapi public key request: %w", err)
	}

	if response.StatusCode()/100 != 2 {
		return nil, fmt.Errorf("xeapi public key http status %d: %s", response.StatusCode(), string(response.Body()))
	}

	plaintext, err := crypto.EApiDecrypt(string(response.Body()), "")
	if err != nil {
		return nil, fmt.Errorf("EApiDecrypt xeapi public key response: %w", err)
	}

	plaintext, err = utils.GzipReader(plaintext)
	if err != nil {
		return nil, fmt.Errorf("decompress xeapi public key response: %w", err)
	}

	var reply xeapiRefreshPublicKeyResp

	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.UseNumber()

	if decodeErr := decoder.Decode(&reply); decodeErr != nil {
		return nil, fmt.Errorf("json.Decode xeapi public key response: %w", decodeErr)
	}

	if reply.Code != http.StatusOK || reply.Data.EncryptedData == "" {
		return nil, fmt.Errorf("xeapi public key failed: code=%d message=%s", reply.Code, reply.Message)
	}

	respTimestamp, err := rawTimestampString(reply.Data.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("rawTimestampString: %w", err)
	}

	expectedSignature := crypto.XeapiSign(respTimestamp, nonce)
	if reply.Data.Signature == "" || !hmac.Equal([]byte(expectedSignature), []byte(reply.Data.Signature)) {
		return nil, errors.New("xeapi public key response signature mismatch")
	}

	state, err := crypto.XeapiDecryptPublicKey(reply.Data.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("XeapiDecryptPublicKey: %w", err)
	}

	state.DeviceID = req.DeviceID
	return state, nil
}

// updateSession 使用乐观锁更新响应会话：公钥修订号必须仍然匹配，且同一公钥下
// 不能让较早请求的响应覆盖较晚请求已经写入的会话。
func (x *xeapi) updateSession(response *resty.Response, revision xeapiRequestRevision) error {
	if response == nil {
		return nil
	}

	session := crypto.XeapiSession{
		ID:  strings.TrimSpace(response.Header().Get("X-Encr-Ssid")),
		Key: strings.TrimSpace(response.Header().Get("X-Encr-Sskey")),
	}
	if session.ID == "" && session.Key == "" {
		return nil
	}

	x.mux.Lock()
	defer x.mux.Unlock()

	// 刷新公钥或重新加载状态后，旧公钥请求的响应不再有资格更新会话。
	if revision.keyRevision != x.keyRevision {
		return nil
	}

	if err := session.Validate(); err != nil {
		return err
	}

	// 同一公钥修订号内，只接受尚未被更新请求超越的响应。
	if revision.requestSequence < x.acceptedSessionSequence {
		return nil
	}

	x.Session = session
	x.acceptedSessionSequence = revision.requestSequence
	return nil
}

func rewriteXeapiURL(rawURL string) (string, string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("url.Parse xeapi url: %w", err)
	}

	var (
		envelopeURI   = *parsedURL
		requestURI    = *parsedURL
		envelopeParts = strings.Split(envelopeURI.Path, "/")
		xeapiParts    = append([]string(nil), envelopeParts...)
		replaced      = false
	)
	for i, part := range envelopeParts {
		switch part {
		case "eapi":
			envelopeParts[i] = "api"
			xeapiParts[i] = "xeapi"
			replaced = true
		case "api":
			xeapiParts[i] = "xeapi"
			replaced = true
		case "xeapi":
			replaced = true
		default:
			continue
		}
		break
	}

	if !replaced {
		return "", "", fmt.Errorf("xeapi url path must contain /api, /eapi, or /xeapi: %s", parsedURL.Path)
	}

	envelopeURI.Path = strings.Join(envelopeParts, "/")
	requestURI.Path = strings.Join(xeapiParts, "/")
	// Query parameters are moved into the encrypted plaintext envelope.
	requestURI.RawQuery = ""
	requestURI.Fragment = ""
	return envelopeURI.String(), requestURI.String(), nil
}

func generateXeapiNonce() (string, error) {
	nonce := make([]byte, 16)
	for i := range nonce {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("rand.Int xeapi nonce: %w", err)
		}

		nonce[i] = byte('0' + n.Int64())
	}
	return string(nonce), nil
}

func rawTimestampString(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}

	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		return num.String(), nil
	}
	return "", errors.New("xeapi public key response timestamp invalid")
}

func xeapiKeyNeedsRefresh(key crypto.XeapiPublicKeyState, deviceID string) bool {
	if key.Validate() != nil {
		return true
	}

	cachedDeviceID := strings.TrimSpace(key.DeviceID)
	return cachedDeviceID != "" && cachedDeviceID != strings.TrimSpace(deviceID)
}

func xeapiPublicKeyComplete(key crypto.XeapiPublicKeyState) bool {
	return strings.TrimSpace(key.PublicKey) != "" &&
		strings.TrimSpace(key.Version) != "" &&
		strings.TrimSpace(key.SK) != ""
}
