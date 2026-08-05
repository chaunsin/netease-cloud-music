// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

// crypto xeapi implements the algorithm documented in
// https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced/issues/174.

const xeapiDefaultContentType = "application/x-www-form-urlencoded;charset=utf-8"

var (
	ErrEncryptRequestMissing   = errors.New("xeapi encrypt request is missing")
	ErrPublicKeyMissing        = errors.New("xeapi public key is missing")
	ErrPublicKeyVersionMissing = errors.New("xeapi public key version is missing")
	ErrPublicKeyExpired        = errors.New("xeapi public key is expired")
	ErrServerKeyMissing        = errors.New("xeapi server key is missing")
	ErrSessionIncomplete       = errors.New("xeapi session is incomplete")
	ErrSessionKeyLength        = errors.New("xeapi session key length is invalid")
)

var (
	xeapiRandomBytes = func(length int) ([]byte, error) {
		data := make([]byte, length)
		if _, err := cryptorand.Read(data); err != nil {
			return nil, err
		}
		return data, nil
	}
	xeapiGenerateX25519Key = func(curve ecdh.Curve) (*ecdh.PrivateKey, error) {
		return curve.GenerateKey(cryptorand.Reader)
	}
)

// XeapiPublicKeyState 是 xeapi 公钥刷新接口返回并缓存的服务端密钥状态。
type XeapiPublicKeyState struct {
	PublicKey      string `json:"publicKey"      yaml:"publicKey"`
	Version        string `json:"version"        yaml:"version"`
	NextUpdateTime int64  `json:"nextUpdateTime" yaml:"nextUpdateTime"`
	SK             string `json:"sk"             yaml:"sk"`
	DeviceID       string `json:"deviceId,omitempty" yaml:"deviceId,omitempty"`
}

// Validate verifies that the cached public-key state is safe to use for a request.
func (p XeapiPublicKeyState) Validate() error {
	return p.validate(true)
}

func (p XeapiPublicKeyState) validate(requireServerKey bool) error {
	if strings.TrimSpace(p.PublicKey) == "" {
		return ErrPublicKeyMissing
	}

	if strings.TrimSpace(p.Version) == "" {
		return ErrPublicKeyVersionMissing
	}

	if requireServerKey && strings.TrimSpace(p.SK) == "" {
		return ErrServerKeyMissing
	}

	if p.NextUpdateTime > 0 && time.Now().UnixMilli() >= p.NextUpdateTime {
		return ErrPublicKeyExpired
	}

	return nil
}

// XeapiSession 保存 xeapi 响应头下发的会话信息。
type XeapiSession struct {
	ID  string `json:"id"  yaml:"id"`
	Key string `json:"key" yaml:"key"`
}

// Validate accepts an empty session or a complete session with a raw AES key.
func (s XeapiSession) Validate() error {
	var (
		hasID  = strings.TrimSpace(s.ID) != ""
		hasKey = strings.TrimSpace(s.Key) != ""
	)

	if !hasID && !hasKey {
		return nil
	}

	if !hasID || !hasKey {
		return ErrSessionIncomplete
	}

	switch len([]byte(s.Key)) {
	case 16, 24, 32:
		return nil
	default:
		return fmt.Errorf("%w: got %d bytes", ErrSessionKeyLength, len([]byte(s.Key)))
	}
}

// XeapiEncryptRequest 描述待封装的原始 API 请求及公钥失效时使用的刷新上下文。
type XeapiEncryptRequest struct {
	URI         string
	Data        any
	Body        []byte
	Method      string
	ContentType string
	OS          string
	AppVersion  string
	DeviceID    string
	UserAgent   string
	T1          string
	T2          string
	UID         string
}

// XeapiEncryptedRequest contains the three encrypted XEAPI request fields.
type XeapiEncryptedRequest struct {
	B string
	S string
	R string
}

// XeapiDecryptedRequest describes the metadata from R and, when a dynamic key
// is available, the plaintext request envelope from B.
type XeapiDecryptedRequest struct {
	PublicKeyVersion string
	SessionID        string
	Plaintext        []byte
	SFrameValid      bool
}

type xeapiPlaintextEnvelope struct {
	Body        *string `json:"body,omitempty"`
	Method      string  `json:"method,omitempty"`
	ContentType string  `json:"contentType,omitempty"`
	QueryString string  `json:"queryString,omitempty"`
}

// XeapiSign 生成公钥刷新请求/响应校验用的 HMAC-SHA256 签名。
func XeapiSign(timestamp, nonce string) string {
	mac := hmac.New(sha256.New, []byte(xeapiSignKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(nonce))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// XeapiDecrypt解密 xeapi 业务响应，明文为 gzip 时会继续解压。
func XeapiDecrypt(body []byte) ([]byte, error) {
	plaintext, err := aesECBDecrypt([]byte(eApiKey), body)
	if err != nil {
		return nil, fmt.Errorf("aesECBDecrypt: %w", err)
	}

	return utils.GzipReader(plaintext)
}

// XeapiDecryptRequest decrypts R with the known static key and optionally B
// with a caller-supplied dynamic or session key. A present S is validated as a
// strict Base64 X25519/GCM frame, but cannot be decrypted without either side's
// X25519 private key. If R succeeds but B or S fails, the returned result keeps
// every independently recovered field alongside the non-nil error.
func XeapiDecryptRequest(req XeapiEncryptedRequest, dynamicKey []byte) (XeapiDecryptedRequest, error) {
	version, sessionID, err := decryptXeapiR(req.R)
	if err != nil {
		return XeapiDecryptedRequest{}, fmt.Errorf("decrypt R: %w", err)
	}

	result := XeapiDecryptedRequest{
		PublicKeyVersion: version,
		SessionID:        sessionID,
	}

	var decryptErrs []error

	if len(dynamicKey) > 0 {
		plaintext, decryptErr := decryptXeapiB(req.B, dynamicKey)
		if decryptErr != nil {
			decryptErrs = append(decryptErrs, fmt.Errorf("decrypt B: %w", decryptErr))
		} else {
			result.Plaintext = plaintext
		}
	} else if req.B != "" {
		if _, validateErr := decodeXeapiBFrame(req.B); validateErr != nil {
			decryptErrs = append(decryptErrs, fmt.Errorf("validate B: %w", validateErr))
		}
	}

	if req.S != "" {
		if validateErr := validateXeapiS(req.S); validateErr != nil {
			decryptErrs = append(decryptErrs, fmt.Errorf("validate S: %w", validateErr))
		} else {
			result.SFrameValid = true
		}
	}
	return result, errors.Join(decryptErrs...)
}

// XeapiEncrypt 将原始 API 请求加密并封装为 xeapi 的 B/S/R 表单参数。
func XeapiEncrypt(req *XeapiEncryptRequest, publicKey XeapiPublicKeyState, session XeapiSession) (map[string]string, error) {
	if req == nil {
		return nil, ErrEncryptRequestMissing
	}

	if err := publicKey.Validate(); err != nil {
		return nil, err
	}

	osName := req.OS
	if osName == "" {
		osName = "android"
	}

	plaintext, err := buildPlaintextEnvelope(req)
	if err != nil {
		return nil, fmt.Errorf("buildPlaintextEnvelope: %w", err)
	}

	inner, err := aesECBEncrypt(xeapiStaticKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt plaintext envelope: %w", err)
	}

	mid, err := midTransform(inner)
	if err != nil {
		return nil, fmt.Errorf("midTransform: %w", err)
	}

	dynamicKey, activeSessionID, err := dynamicKey(session)
	if err != nil {
		return nil, fmt.Errorf("dynamicKey: %w", err)
	}

	b, err := aesECBEncrypt(dynamicKey, mid)
	if err != nil {
		return nil, fmt.Errorf("aesECBEncrypt B: %w", err)
	}

	s, err := encryptS(dynamicKey, publicKey, osName)
	if err != nil {
		return nil, fmt.Errorf("encryptS: %w", err)
	}

	r, err := aesECBEncrypt(xeapiStaticKey, []byte(publicKey.Version+"|"+activeSessionID))
	if err != nil {
		return nil, fmt.Errorf("aesECBEncrypt R: %w", err)
	}
	return map[string]string{
		"B": base64.StdEncoding.EncodeToString(b),
		"S": base64.StdEncoding.EncodeToString(s),
		"R": base64.StdEncoding.EncodeToString(r),
	}, nil
}

// XeapiDecryptPublicKey 解密公钥刷新响应里的 encryptedData 字段。
func XeapiDecryptPublicKey(encryptedData string) (*XeapiPublicKeyState, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("base64.DecodeString public key: %w", err)
	}

	plaintext, err := aesECBDecrypt(xeapiStaticKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt public key: %w", err)
	}

	var state XeapiPublicKeyState
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return nil, fmt.Errorf("json.Unmarshal public key: %w", err)
	}

	// The refresh caller may inherit SK from the previous state, but the new
	// public key, version, and expiration must be independently trustworthy.
	if err := state.validate(false); err != nil {
		return nil, err
	}
	return &state, nil
}

// buildPlaintextEnvelope 构建加冕请求体.
func buildPlaintextEnvelope(req *XeapiEncryptRequest) ([]byte, error) {
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = http.MethodPost
	}

	if method == http.MethodPost {
		method = ""
	}

	contentType := req.ContentType
	if isDefaultXeapiContentType(contentType) {
		contentType = ""
	}

	queryString, err := xeapiQueryString(req.URI)
	if err != nil {
		return nil, fmt.Errorf("xeapiQueryString: %w", err)
	}

	body, hasBody, err := xeapiBody(req)
	if err != nil {
		return nil, fmt.Errorf("xeapiBody: %w", err)
	}

	envelope := xeapiPlaintextEnvelope{
		Method:      method,
		ContentType: contentType,
		QueryString: queryString,
	}

	if hasBody {
		encodedBody := base64.StdEncoding.EncodeToString(body)
		envelope.Body = &encodedBody
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal xeapi plaintext envelope: %w", err)
	}
	return data, nil
}

func xeapiQueryString(rawURI string) (string, error) {
	if strings.TrimSpace(rawURI) == "" {
		return "", errors.New("xeapi uri is empty")
	}

	uri, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("url.Parse xeapi uri: %w", err)
	}

	// AegisSDK 只把原始 URL 的 query 放入明文信封，且总是追加 e_r=true 触发加密响应。
	rawQuery := uri.RawQuery
	if rawQuery == "" {
		return "e_r=true", nil
	}
	return rawQuery + "&e_r=true", nil
}

func xeapiBody(req *XeapiEncryptRequest) ([]byte, bool, error) {
	if req.Body != nil {
		// Body 用于已经有完整原始请求体的场景，避免重新编码导致字节序或转义方式变化。
		return append([]byte(nil), req.Body...), true, nil
	}

	if req.Data == nil {
		return nil, false, nil
	}

	if !isDefaultXeapiContentType(req.ContentType) {
		body, err := rawRequestBody(req.Data)
		if err != nil {
			return nil, false, fmt.Errorf("rawRequestBody: %w", err)
		}
		return body, true, nil
	}

	values, err := formValues(req.Data)
	if err != nil {
		return nil, false, err
	}
	// 高层 Data 输入代表表单字段；xeapi 的 e_r 应落在 queryString，避免表单里重复携带。
	values.Del("e_r")
	return []byte(values.Encode()), true, nil
}

func isDefaultXeapiContentType(contentType string) bool {
	if contentType == "" {
		return true
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType, _, _ = strings.Cut(strings.TrimSpace(contentType), ";")
		mediaType = strings.TrimSpace(mediaType)
	}

	defaultMediaType, _, _ := mime.ParseMediaType(xeapiDefaultContentType)
	return strings.EqualFold(mediaType, defaultMediaType)
}

func rawRequestBody(data any) ([]byte, error) {
	switch v := data.(type) {
	case []byte:
		return append([]byte(nil), v...), nil
	case string:
		return []byte(v), nil
	default:
		body, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("json.Marshal xeapi raw body: %w", err)
		}
		return body, nil
	}
}

func formValues(data any) (url.Values, error) {
	switch v := data.(type) {
	case nil:
		return url.Values{}, nil
	case url.Values:
		return maps.Clone(v), nil
	case map[string][]string:
		return maps.Clone(url.Values(v)), nil
	case map[string]string:
		values := make(url.Values, len(v))
		for key, value := range v {
			values.Set(key, value)
		}
		return values, nil
	case string:
		return url.ParseQuery(v)
	case []byte:
		return url.ParseQuery(string(v))
	default:
		return jsonFormValues(v)
	}
}

func jsonFormValues(data any) (url.Values, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal xeapi data: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, fmt.Errorf("json.Unmarshal xeapi data: %w", err)
	}

	values := make(url.Values, len(fields))
	for key, raw := range fields {
		text, err := rawFormValue(raw)
		if err != nil {
			return nil, fmt.Errorf("format xeapi form value %q: %w", key, err)
		}

		values.Set(key, text)
	}
	return values, nil
}

func rawFormValue(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}

	if !json.Valid(raw) {
		return "", errors.New("invalid json value")
	}
	return string(raw), nil
}

func dynamicKey(session XeapiSession) ([]byte, string, error) {
	if err := session.Validate(); err != nil {
		return nil, "", err
	}

	if strings.TrimSpace(session.ID) != "" {
		// x-encr-sskey 是服务端下发的 ASCII 字符串，形似 hex 时也不能做 hex.DecodeString。
		return []byte(session.Key), strings.TrimSpace(session.ID), nil
	}

	key, err := xeapiRandomBytes(16)
	if err != nil {
		return nil, "", fmt.Errorf("crypto.Read dynamic key: %w", err)
	}
	return key, "", nil
}

func midTransform(ciphertext []byte) ([]byte, error) {
	random, err := xeapiRandomBytes(16)
	if err != nil {
		return nil, fmt.Errorf("crypto.Read mid random: %w", err)
	}

	xored := make([]byte, len(ciphertext))
	for i := range ciphertext {
		xored[i] = ciphertext[i] ^ random[i&0x0f]
	}

	var (
		b64 = []byte(base64.StdEncoding.EncodeToString(xored))
		rot = 0
	)
	if len(b64) > 0 {
		rot = int(random[0]&0x0f) % len(b64)
	}

	out := make([]byte, 0, len(random)+len(b64))
	out = append(out, random...)
	out = append(out, b64[rot:]...)
	out = append(out, b64[:rot]...)
	return out, nil
}

func decryptXeapiB(encryptedB string, dynamicKey []byte) ([]byte, error) {
	ciphertext, err := decodeXeapiBBase64(encryptedB)
	if err != nil {
		return nil, err
	}

	mid, err := aesECBDecrypt(dynamicKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt B outer layer: %w", err)
	}

	if len(mid) <= aes.BlockSize {
		return nil, fmt.Errorf("mid payload too short: got %d bytes", len(mid))
	}

	var (
		random  = mid[:aes.BlockSize]
		rotated = mid[aes.BlockSize:]
		rot     = int(random[0]&0x0f) % len(rotated)
	)

	b64 := make([]byte, 0, len(rotated))
	b64 = append(b64, rotated[len(rotated)-rot:]...)
	b64 = append(b64, rotated[:len(rotated)-rot]...)

	xored, err := base64.StdEncoding.Strict().DecodeString(string(b64))
	if err != nil {
		return nil, fmt.Errorf("base64.DecodeString reversed mid payload: %w", err)
	}

	inner := make([]byte, len(xored))
	for i := range xored {
		inner[i] = xored[i] ^ random[i%len(random)]
	}

	plaintext, err := aesECBDecrypt(xeapiStaticKey, inner)
	if err != nil {
		return nil, fmt.Errorf("decrypt B inner layer: %w", err)
	}
	return plaintext, nil
}

func decodeXeapiBFrame(encryptedB string) ([]byte, error) {
	ciphertext, err := decodeXeapiBBase64(encryptedB)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("b ciphertext length %d is not a positive AES block multiple", len(ciphertext))
	}
	return ciphertext, nil
}

func decodeXeapiBBase64(encryptedB string) ([]byte, error) {
	if encryptedB == "" {
		return nil, errors.New("xeapi B is empty")
	}

	ciphertext, err := base64.StdEncoding.Strict().DecodeString(encryptedB)
	if err != nil {
		return nil, fmt.Errorf("base64.DecodeString B: %w", err)
	}
	return ciphertext, nil
}

func decryptXeapiR(encryptedR string) (string, string, error) {
	if encryptedR == "" {
		return "", "", errors.New("xeapi R is empty")
	}

	ciphertext, err := base64.StdEncoding.Strict().DecodeString(encryptedR)
	if err != nil {
		return "", "", fmt.Errorf("base64.DecodeString R: %w", err)
	}

	plaintext, err := aesECBDecrypt(xeapiStaticKey, ciphertext)
	if err != nil {
		return "", "", fmt.Errorf("decrypt R: %w", err)
	}

	version, sessionID, ok := strings.Cut(string(plaintext), "|")
	if !ok || strings.TrimSpace(version) == "" || strings.Contains(sessionID, "|") {
		return "", "", errors.New("invalid R plaintext format")
	}
	return version, sessionID, nil
}

func validateXeapiS(encryptedS string) error {
	frame, err := base64.StdEncoding.Strict().DecodeString(encryptedS)
	if err != nil {
		return fmt.Errorf("base64.DecodeString S: %w", err)
	}

	const fixedFrameSize = 32 + 12 + 16 // X25519 public key, GCM nonce, and tag.
	if len(frame) <= fixedFrameSize {
		return fmt.Errorf("frame is too short: got %d bytes, want more than %d", len(frame), fixedFrameSize)
	}
	return nil
}

func encryptS(dynamicKey []byte, publicKey XeapiPublicKeyState, os string) ([]byte, error) {
	peerRaw, err := base64.StdEncoding.DecodeString(publicKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("base64.DecodeString peer public key: %w", err)
	}

	curve := ecdh.X25519()

	peer, err := curve.NewPublicKey(peerRaw)
	if err != nil {
		return nil, fmt.Errorf("x25519 peer public key: %w", err)
	}

	privateKey, err := xeapiGenerateX25519Key(curve)
	if err != nil {
		return nil, fmt.Errorf("x25519 generate key: %w", err)
	}

	ephemeralRaw := privateKey.PublicKey().Bytes()

	sharedSecret, err := privateKey.ECDH(peer)
	if err != nil {
		return nil, fmt.Errorf("x25519 ECDH: %w", err)
	}

	iv, err := xeapiRandomBytes(12)
	if err != nil {
		return nil, fmt.Errorf("cryptorand gcm iv: %w", err)
	}

	block, err := aes.NewCipher(deriveX25519AESKey(sharedSecret, ephemeralRaw))
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher S: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	var (
		plaintext = []byte(base64.StdEncoding.EncodeToString(dynamicKey) + "|" + os + "|" + publicKey.SK)
		encrypted = gcm.Seal(nil, iv, plaintext, nil)
		out       = make([]byte, 0, len(ephemeralRaw)+len(iv)+len(encrypted))
	)

	out = append(out, ephemeralRaw...)
	out = append(out, iv...)
	out = append(out, encrypted...)
	return out, nil
}

func deriveX25519AESKey(sharedSecret, ephemeralPublicKey []byte) []byte {
	if len(sharedSecret) == 0 {
		sharedSecret = make([]byte, 32)
	}

	prkMAC := hmac.New(sha256.New, make([]byte, 32))
	prkMAC.Write(sharedSecret)
	prk := prkMAC.Sum(nil)

	hash := hmac.New(sha256.New, prk)
	hash.Write(ephemeralPublicKey)
	hash.Write([]byte{1})
	return hash.Sum(nil)[:16]
}
