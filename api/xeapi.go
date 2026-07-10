// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"

	"github.com/go-resty/resty/v2"
)

const (
	defaultXeapiAppVer    = "9.1.65"
	defaultXeapiOS        = "android"
	defaultXeapiUserAgent = "NeteaseMusic/9.1.65.240927161425(9001065);Dalvik/2.1.0 (Linux; U; Android 14; 23013RK75C Build/UKQ1.230804.001)"
)

type xeapiKeyResponse struct {
	Code int `json:"code"`
	Data struct {
		EncryptedData string          `json:"encryptedData"`
		Signature     string          `json:"signature"`
		Timestamp     json.RawMessage `json:"timestamp"`
	} `json:"data"`
	Message string `json:"message"`
}

type xeapiKeyRefreshRequest struct {
	DeviceID          string
	CurrentKeyVersion string
	AppVersion        string
	UserAgent         string
	OS                string
}

type xeapiStateResult struct {
	key     crypto.PublicKeyState
	session crypto.Session
}

func (c *Client) xeapiEncrypt(ctx context.Context, rawURL string, req interface{}, opts *Options, contentType string) (string, map[string]string, error) {
	envelopeURL, xeapiURL, err := rewriteXeapiURL(rawURL) // todo: url xeapi todo
	if err != nil {
		return "", nil, err
	}

	encryptReq := crypto.EncryptRequest{
		URI:         envelopeURL,
		Data:        req,
		Method:      opts.Method,
		ContentType: contentType,
		OS:          firstNonEmpty(opts.XeapiOS, defaultXeapiOS),
		AppVersion:  firstNonEmpty(opts.XeapiAppVer, defaultXeapiAppVer),
		DeviceID:    c.GetDeviceId(),
		UserAgent:   xeapiUserAgent(opts),
	}
	result, err := c.xeapiState(ctx, encryptReq)
	if err != nil {
		return "", nil, fmt.Errorf("xeapiState: %w", err)
	}
	encryptData, err := crypto.XeapiEncrypt(encryptReq, result.key, result.session)
	if err != nil {
		return "", nil, fmt.Errorf("XeapiEncrypt: %w", err)
	}
	return xeapiURL, encryptData, nil
}

func (c *Client) xeapiState(ctx context.Context, req crypto.EncryptRequest) (*xeapiStateResult, error) {
	c.xeapiMu.Lock()
	var (
		key          = c.xeapiKey
		session      = c.xeapiSession
		needsRefresh = key.PublicKey == "" || key.SK == "" || xeapiKeyExpired(key)
		groupKey     = strings.Join([]string{req.DeviceID, req.AppVersion, req.OS, req.UserAgent}, "\x00")
	)
	c.xeapiMu.Unlock()
	if !needsRefresh {
		return &xeapiStateResult{key: key, session: session}, nil
	}

	value, err, _ := c.xeapiRefresh.Do(groupKey, func() (interface{}, error) {
		c.xeapiMu.Lock()
		var (
			key            = c.xeapiKey
			session        = c.xeapiSession
			needsRefresh   = key.PublicKey == "" || key.SK == "" || xeapiKeyExpired(key)
			currentVersion = key.Version
			currentSK      = key.SK
		)
		c.xeapiMu.Unlock()
		if !needsRefresh {
			return &xeapiStateResult{key: key, session: session}, nil
		}

		refreshed, err := c.refreshXeapiPublicKey(ctx, xeapiKeyRefreshRequest{
			DeviceID:          req.DeviceID,
			CurrentKeyVersion: currentVersion,
			AppVersion:        req.AppVersion,
			UserAgent:         req.UserAgent,
			OS:                req.OS,
		})
		if err != nil {
			return nil, fmt.Errorf("refreshXeapiPublicKey: %w", err)
		}
		if strings.TrimSpace(refreshed.SK) == "" {
			refreshed.SK = currentSK
		}
		if strings.TrimSpace(refreshed.SK) == "" {
			return nil, crypto.ErrServerKeyMissing
		}

		c.xeapiMu.Lock()
		c.xeapiKey = *refreshed
		session = c.xeapiSession
		c.xeapiMu.Unlock()
		return &xeapiStateResult{key: *refreshed, session: session}, nil
	})
	if err != nil {
		return nil, err
	}
	result := value.(*xeapiStateResult)
	return result, nil
}

func (c *Client) refreshXeapiPublicKey(ctx context.Context, req xeapiKeyRefreshRequest) (*crypto.PublicKeyState, error) {
	nonce, err := generateXeapiNonce()
	if err != nil {
		return nil, err
	}

	var (
		timestamp = strconv.FormatInt(time.Now().UnixMilli(), 10)
		form      = map[string]string{
			"appVersion":        firstNonEmpty(req.AppVersion, defaultXeapiAppVer),
			"currentKeyVersion": req.CurrentKeyVersion,
			"deviceId":          req.DeviceID,
			"nonce":             nonce,
			"os":                firstNonEmpty(req.OS, defaultXeapiOS),
			"requestType":       "active",
			"signature":         crypto.XeapiSign(timestamp, nonce),
			"timestamp":         timestamp,
			"t1":                "", // todo: Filling
			"t2":                "",
			"uid":               "",
		}
	)

	request := c.cli.R().
		SetContext(ctx).
		SetHeader("Host", "interface.music.163.com").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("User-Agent", firstNonEmpty(req.UserAgent, defaultXeapiUserAgent)).
		SetFormData(form)
	if strings.TrimSpace(req.DeviceID) != "" {
		request.SetCookie(&http.Cookie{Name: "deviceId", Value: req.DeviceID})
	}

	response, err := request.Post("https://interface.music.163.com/api/gorilla/anti/crawler/security/key/get")
	if err != nil {
		return nil, fmt.Errorf("xeapi public key request: %w", err)
	}
	if response.StatusCode()/100 != 2 {
		return nil, fmt.Errorf("xeapi public key http status %d: %s", response.StatusCode(), string(response.Body()))
	}

	var reply xeapiKeyResponse
	decoder := json.NewDecoder(bytes.NewReader(response.Body()))
	decoder.UseNumber()
	if err := decoder.Decode(&reply); err != nil {
		return nil, fmt.Errorf("json.Decode xeapi public key response: %w", err)
	}
	if reply.Code != http.StatusOK || reply.Data.EncryptedData == "" {
		return nil, fmt.Errorf("xeapi public key failed: code=%d message=%s", reply.Code, reply.Message)
	}

	respTimestamp, err := rawTimestampString(reply.Data.Timestamp)
	if err != nil {
		return nil, err
	}
	expectedSignature := crypto.XeapiSign(respTimestamp, nonce)
	if reply.Data.Signature == "" || !hmac.Equal([]byte(expectedSignature), []byte(reply.Data.Signature)) {
		return nil, fmt.Errorf("xeapi public key response signature mismatch")
	}

	state, err := crypto.XeapiDecryptPublicKey(reply.Data.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("XeapiDecryptPublicKey: %w", err)
	}
	state.DeviceID = req.DeviceID
	return state, nil
}

func (c *Client) updateXeapiSession(response *resty.Response) {
	if response == nil {
		return
	}

	sessionID := strings.TrimSpace(response.Header().Get("x-encr-ssid"))
	sessionKey := strings.TrimSpace(response.Header().Get("x-encr-sskey"))
	if sessionID == "" || sessionKey == "" {
		return
	}

	c.xeapiMu.Lock()
	c.xeapiSession = crypto.Session{ID: sessionID, Key: sessionKey}
	c.xeapiMu.Unlock()
}

func rewriteXeapiURL(rawURL string) (envelopeURL string, requestURL string, err error) {
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

func xeapiKeyExpired(key crypto.PublicKeyState) bool {
	if key.NextUpdateTime <= 0 {
		return false
	}
	return time.Now().UnixMilli() >= key.NextUpdateTime
}

func generateXeapiNonce() (string, error) {
	var nonce = make([]byte, 16)
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
	return "", fmt.Errorf("xeapi public key response timestamp invalid")
}

func xeapiUserAgent(opts *Options) string {
	if opts != nil {
		for key, value := range opts.Headers {
			if strings.EqualFold(key, "User-Agent") && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return defaultXeapiUserAgent
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
