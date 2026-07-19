// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package crypto

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // NetEase EAPI and cache formats require MD5 for protocol compatibility.
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

const (
	base62                  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	idXORKey1               = "3go8&$8*3*3h0k(2)2"
	cacheKey                = ")(13daqP@ssw0rd~"
	iv                      = "0102030405060708"
	presetKey               = "0CoJUm6Qyw8W8jud"
	linuxApiKey             = "rFgB&h#%2?^eDg:Q"
	eApiKey                 = "e82ckenh8dichen8"
	eApiFormat              = "%s-36cd479b6b5-%s-36cd479b6b5-%s"
	eApiSlat                = "nobody%suse%smd5forencrypt"
	xeapiSignKey            = "mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g==" // xeapi 的 signKey 在 AegisSDK 中以这段 Base64 文本原样参与 HMAC，不需要先做 Base64 解码。
	xeapiStaticKeyHex       = "ab1d5a430f6bb04a3f01e81ddd72bd916d5ce591248ac128714806d7f8fb1b84"                         // xeapi 静态密钥为 32 字节 AES-256-ECB key，用于公钥缓存、B 内层和 R 参数加解密。
	xeapiDefaultContentType = "application/x-www-form-urlencoded;charset=utf-8"
	publicKey               = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDgtQn2JZ34ZC28NWYpAUd98iZ37BUrX/aKzmFbt7clFSs6sXqHauqKWqdtLkF2KexO40H1YTX8z2lSgBBOAxLsvaklV8k4cBFK9snQXE9/DDaFt6Rr7iVZMldczhC0JNgTz+SHXT6CBHuX3e9SdB1Ua44oncaTWz7OBGLbCiK45wIDAQAB
-----END PUBLIC KEY-----`
)

var xeapiStaticKey = mustDecodeHex(xeapiStaticKeyHex)

func mustDecodeHex(s string) []byte {
	data, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return data
}

func randomKey() (string, error) {
	var (
		alphabetSize = big.NewInt(int64(len(base62)))
		key          = make([]byte, aes.BlockSize)
	)

	for i := range key {
		index, err := cryptorand.Int(cryptorand.Reader, alphabetSize)
		if err != nil {
			return "", fmt.Errorf("rand.Int: %w", err)
		}

		key[i] = base62[index.Int64()]
	}
	return string(key), nil
}

func reverseString(str string) string {
	runes := []rune(str)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func digest(requestURL, data string) string {
	message := fmt.Sprintf(eApiSlat, requestURL, data)
	return fmt.Sprintf("%x", legacyMD5([]byte(message)))
}

func legacyMD5(data []byte) [md5.Size]byte {
	return md5.Sum(data) //nolint:gosec // This helper is used only by legacy NetEase wire formats.
}

// aesEncrypt 加密.
func aesEncrypt(text, key, iv, mode, format string) (string, error) {
	// fmt.Printf("[aesEncrypt] request mode=%s format=%s\n", mode, format)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("NewCipher: %w", err)
	}

	padding, err := Pkcs7Padding([]byte(text), block.BlockSize())
	if err != nil {
		return "", fmt.Errorf("Pkcs7Padding: %w", err)
	}

	var cipherText []byte

	switch mode {
	case "cbc":
		cipherText = AesEncryptCBC(block, padding, []byte(iv))
	case "ecb":
		cipherText = AesEncryptECB(block, padding)
	default:
		return "", fmt.Errorf("%s unknown mode", mode)
	}

	switch format {
	case "base64":
		return base64.StdEncoding.EncodeToString(cipherText), nil
	case "hex":
		return hex.EncodeToString(cipherText), nil
	case "HEX":
		return strings.ToUpper(hex.EncodeToString(cipherText)), nil
	default:
		return "", fmt.Errorf("%s unknown format", format)
	}
}

// aesDecrypt 解密.
func aesDecrypt(cipherText, key, iv, mode, format string) ([]byte, error) {
	// fmt.Printf("[aesDecrypt] request mode=%s format=%s\n", mode, format)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("NewCipher: %w", err)
	}

	var data []byte

	switch format {
	case "base64":
		data, err = base64.StdEncoding.DecodeString(cipherText)
	case "hex", "HEX":
		data, err = hex.DecodeString(cipherText)
	case "":
		data = []byte(cipherText)
	default:
		return nil, fmt.Errorf("%s unknown format", format)
	}

	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}

	var text []byte

	switch mode {
	case "cbc":
		// 这里不需要Pkcs7UnPadding?
		text, err = AesDecryptCBC(block, data, []byte(iv))
	case "ecb":
		text, err = AesDecryptECB(block, data)
	default:
		return nil, fmt.Errorf("%s unknown mode", mode)
	}

	if err != nil {
		return nil, fmt.Errorf("mode: %w", err)
	}

	text, err = Pkcs7UnPadding(text)
	if err != nil {
		return nil, fmt.Errorf("Pkcs7UnPadding: %w", err)
	}
	return text, nil
}

// AesEncryptCBC 加密.
func AesEncryptCBC(block cipher.Block, plaintext, iv []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	encrypt := cipher.NewCBCEncrypter(block, iv)
	encrypt.CryptBlocks(ciphertext, plaintext)
	return ciphertext
}

// AesDecryptCBC 解密.
func AesDecryptCBC(block cipher.Block, cipherText, iv []byte) ([]byte, error) {
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("IV length must be %d bytes", block.BlockSize())
	}

	if len(cipherText)%block.BlockSize() != 0 {
		return nil, errors.New("cipherText length is not a multiple of block size")
	}

	data := make([]byte, len(cipherText))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, cipherText)
	return data, nil
}

// AesEncryptECB 加密.
func AesEncryptECB(block cipher.Block, plaintext []byte) []byte {
	ciphertext := make([]byte, len(plaintext))

	blockSize := block.BlockSize()
	for i := 0; i < len(plaintext); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], plaintext[i:i+blockSize])
	}
	return ciphertext
}

// AesDecryptECB 解密.
func AesDecryptECB(block cipher.Block, cipherBytes []byte) ([]byte, error) {
	if len(cipherBytes)%block.BlockSize() != 0 {
		return nil, errors.New("cipherBytes length is not a multiple of block size")
	}

	decrypted := make([]byte, len(cipherBytes))
	for i := 0; i < len(cipherBytes); i += block.BlockSize() {
		block.Decrypt(decrypted[i:i+block.BlockSize()], cipherBytes[i:i+block.BlockSize()])
	}
	return decrypted, nil
}

// RsaEncrypt 公钥加密无填充方式.
func RsaEncrypt(ciphertext, key string) (string, error) {
	block, _ := pem.Decode([]byte(key))
	if block == nil {
		return "", errors.New("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("ParsePKIXPublicKey: %w", err)
	}

	pubKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("failed to parse DER encoded public key")
	}

	// weapi 的 encSecKey 使用 RSA no-padding，输出需要补齐到模数字节数对应的 hex 长度。
	c := new(big.Int).SetBytes([]byte(ciphertext))
	// encryptedBytes := c.Exp(c, big.NewInt(int64(pubKey.E)), pubKey.N).Bytes()
	encryptedBytes := c.Exp(c, big.NewInt(int64(pubKey.E)), pubKey.N).FillBytes(make([]byte, pubKey.Size()))
	return hex.EncodeToString(encryptedBytes), nil
}

// Pkcs7Padding 补码,严格遵循 RFC 5652 规范.
func Pkcs7Padding(data []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 || blockSize > 255 {
		return nil, errors.New("pkcs7: invalid block size")
	}

	padding := blockSize - (len(data) % blockSize)
	if padding == 0 {
		padding = blockSize // 必须添加完整填充块
	}

	// 验证填充值有效性
	if padding < 1 || padding > blockSize {
		return nil, errors.New("pkcs7: invalid padding size")
	}
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...), nil
}

// Pkcs7UnPadding 去码.
func Pkcs7UnPadding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("pkcs7: empty input data")
	}

	padding := int(data[len(data)-1])
	if padding < 1 || padding > len(data) {
		return nil, errors.New("pkcs7: invalid padding size")
	}

	// 验证所有填充字节一致
	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, errors.New("pkcs7: invalid padding content")
		}
	}
	return data[:len(data)-padding], nil
}

// WeApiEncrypt 加密.
func WeApiEncrypt(object any) (map[string]string, error) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	secretKey, err := randomKey()
	if err != nil {
		return nil, fmt.Errorf("randomKey: %w", err)
	}

	encryptText, err := aesEncrypt(string(data), presetKey, iv, "cbc", "base64")
	if err != nil {
		return nil, fmt.Errorf("aesEncrypt: %w", err)
	}

	params, err := aesEncrypt(encryptText, secretKey, iv, "cbc", "base64")
	if err != nil {
		return nil, fmt.Errorf("aesEncrypt: %w", err)
	}

	encSecKey, err := RsaEncrypt(reverseString(secretKey), publicKey)
	if err != nil {
		return nil, fmt.Errorf("RsaEncrypt: %w", err)
	}
	return map[string]string{
		"params":    params,
		"encSecKey": encSecKey,
	}, nil
}

// WeApiDecrypt 解密 TODO: 由于拿不到私钥则不能解密.
func WeApiDecrypt(params, encSecKey string) (map[string]string, error) {
	panic("unrealized")
}

// LinuxApiEncrypt 加密.
func LinuxApiEncrypt(object any) (map[string]string, error) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	ciphertext, err := aesEncrypt(string(data), linuxApiKey, "", "ecb", "hex")
	if err != nil {
		return nil, fmt.Errorf("aesEncrypt: %w", err)
	}
	return map[string]string{"eparams": ciphertext}, nil
}

// LinuxApiDecrypt 解密.
func LinuxApiDecrypt(cipherText string) ([]byte, error) {
	plaintext, err := aesDecrypt(cipherText, linuxApiKey, "", "ecb", "hex")
	if err != nil {
		return nil, fmt.Errorf("aesDecrypt: %w", err)
	}
	return plaintext, nil
}

// EApiEncrypt 加密
// 通常在MAC、windows、android、ios中使用
// Pending: 貌似当url为空时存在问题,网易接口加密返回中有不带url的情况，
// 例如: DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08 .
func EApiEncrypt(requestURL string, object any) (map[string]string, error) {
	// 需要替换路由地址,不然会出现接口未找到错误
	requestURL = strings.Replace(requestURL, "eapi", "api", 1)

	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf(eApiFormat, requestURL, string(data), digest(requestURL, string(data)))
	// fmt.Println("payload:", text)

	ciphertext, err := aesEncrypt(text, eApiKey, "", "ecb", "HEX")
	if err != nil {
		return nil, fmt.Errorf("aesEncrypt: %w", err)
	}
	return map[string]string{"params": ciphertext}, nil
}

// EApiDecrypt 解密,当解析请求参数是encode使用hex,当解析请求响应参数为空相当于二进制.
func EApiDecrypt(ciphertext, encode string) ([]byte, error) {
	plaintext, err := aesDecrypt(ciphertext, eApiKey, "", "ecb", encode)
	if err != nil {
		return nil, fmt.Errorf("aesDecrypt: %w", err)
	}
	return plaintext, nil
}

// CacheKeyEncrypt 生成缓存 key.
func CacheKeyEncrypt(data string) (string, error) {
	block, err := aes.NewCipher([]byte(cacheKey))
	if err != nil {
		return "", fmt.Errorf("NewCipher: %w", err)
	}

	padding, err := Pkcs7Padding([]byte(data), block.BlockSize())
	if err != nil {
		return "", fmt.Errorf("Pkcs7Padding: %w", err)
	}

	encrypted := AesEncryptECB(block, padding)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// CacheKeyDecrypt 解密缓存 key.
func CacheKeyDecrypt(data string) (string, error) {
	encrypted, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(cacheKey))
	if err != nil {
		return "", fmt.Errorf("NewCipher: %w", err)
	}

	decrypted, err := AesDecryptECB(block, encrypted)
	if err != nil {
		return "", fmt.Errorf("AesDecryptECB: %w", err)
	}

	plaintext, err := Pkcs7UnPadding(decrypted)
	if err != nil {
		return "", fmt.Errorf("Pkcs7UnPadding: %w", err)
	}
	return string(plaintext), nil
}

// HexDigest returns the lowercase MD5 digest of text.
func HexDigest(text string) string {
	digest := legacyMD5([]byte(text))
	return hex.EncodeToString(digest[:])
}

// DLLEncodeID
// region cloudmusic.dll (Windows) security.
// XORs bytes then returns its base64 MD5 hash. Used in encodeAnonymousId
// Searching for ID_XOR_KEY_1 in cloudmusic.dll will get you to their implementation.
func DLLEncodeID(someID string) (string, error) {
	inputBytes := []byte(someID)
	xor := make([]byte, len(inputBytes))
	keyLength := len(idXORKey1)

	// 执行异或操作
	for i, c := range inputBytes {
		xor[i] = c ^ idXORKey1[i%keyLength]
	}

	// 计算MD5哈希+Base64编码
	digest := legacyMD5(xor)
	return base64.URLEncoding.EncodeToString(digest[:]), nil
}

// Anonymous 匿名用户生成.
func Anonymous(deviceId string) (string, error) {
	encodedID, err := DLLEncodeID(deviceId)
	if err != nil {
		return "", err
	}
	// fmt.Println("encodedID:", encodedID)
	// 构建username内容
	content := fmt.Sprintf("%s %s", deviceId, encodedID)
	username := base64.URLEncoding.EncodeToString([]byte(content))
	return username, nil
}

// xeapi implements the algorithm documented in
// https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced/issues/174.
var (
	ErrEncryptRequestMissing = errors.New("xeapi encrypt request is missing")
	ErrPublicKeyMissing      = errors.New("xeapi public key is missing")
	ErrServerKeyMissing      = errors.New("xeapi server key is missing")
	ErrSessionKeyLength      = errors.New("xeapi session key length is invalid")
)

const (
	defaultOS = "android"
)

// PublicKeyState 是 xeapi 公钥刷新接口返回并缓存的服务端密钥状态。
type PublicKeyState struct {
	PublicKey      string `json:"publicKey"`
	Version        string `json:"version"`
	NextUpdateTime int64  `json:"nextUpdateTime"`
	SK             string `json:"sk"`
	DeviceID       string `json:"deviceId,omitempty"`
}

// Session 保存 xeapi 响应头下发的会话信息。
type Session struct {
	ID  string
	Key string
}

// EncryptRequest 描述待封装的原始 API 请求。
type EncryptRequest struct {
	URI         string
	Data        any
	Body        []byte
	Method      string
	ContentType string
	OS          string
	AppVersion  string
	DeviceID    string
	UserAgent   string
}

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

// XeapiSign 生成公钥刷新请求/响应校验用的 HMAC-SHA256 签名。
func XeapiSign(timestamp, nonce string) string {
	mac := hmac.New(sha256.New, []byte(xeapiSignKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(nonce))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type xeapiPlaintextEnvelope struct {
	Body        *string `json:"body,omitempty"`
	Method      string  `json:"method,omitempty"`
	ContentType string  `json:"contentType,omitempty"`
	QueryString string  `json:"queryString,omitempty"`
}

func buildPlaintextEnvelope(req *EncryptRequest) ([]byte, error) {
	queryString, err := xeapiQueryString(req.URI)
	if err != nil {
		return nil, fmt.Errorf("xeapiQueryString: %w", err)
	}

	body, hasBody, err := xeapiBody(req)
	if err != nil {
		return nil, fmt.Errorf("xeapiBody: %w", err)
	}

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

func xeapiBody(req *EncryptRequest) ([]byte, bool, error) {
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
		return cloneFormValues(v), nil
	case map[string][]string:
		return cloneFormValues(url.Values(v)), nil
	case map[string]string:
		return stringMapFormValues(v), nil
	case string:
		return url.ParseQuery(v)
	case []byte:
		return url.ParseQuery(string(v))
	default:
		return jsonFormValues(v)
	}
}

func cloneFormValues(src url.Values) url.Values {
	values := make(url.Values, len(src))
	for key, list := range src {
		values[key] = append([]string(nil), list...)
	}
	return values
}

func stringMapFormValues(src map[string]string) url.Values {
	values := make(url.Values, len(src))
	for key, value := range src {
		values.Set(key, value)
	}
	return values
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

// XeapiEncrypt 将原始 API 请求封装为 xeapi 的 B/S/R 表单参数。
func XeapiEncrypt(req *EncryptRequest, publicKey PublicKeyState, session Session) (map[string]string, error) {
	if req == nil {
		return nil, ErrEncryptRequestMissing
	}

	if strings.TrimSpace(publicKey.PublicKey) == "" {
		return nil, ErrPublicKeyMissing
	}

	if strings.TrimSpace(publicKey.SK) == "" {
		return nil, ErrServerKeyMissing
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

	osName := req.OS
	if osName == "" {
		osName = defaultOS
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
func XeapiDecryptPublicKey(encryptedData string) (*PublicKeyState, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("base64.DecodeString public key: %w", err)
	}

	plaintext, err := aesECBDecrypt(xeapiStaticKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt public key: %w", err)
	}

	var state PublicKeyState
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return nil, fmt.Errorf("json.Unmarshal public key: %w", err)
	}

	if strings.TrimSpace(state.PublicKey) == "" {
		return nil, ErrPublicKeyMissing
	}
	return &state, nil
}

// XeapiDecryptResponse 解密 xeapi 业务响应，明文为 gzip 时会继续解压。
func XeapiDecryptResponse(body []byte) ([]byte, error) {
	plaintext, err := aesECBDecrypt([]byte(eApiKey), body)
	if err != nil {
		return nil, fmt.Errorf("aesECBDecrypt: %w", err)
	}

	if len(plaintext) >= 2 && plaintext[0] == 0x1f && plaintext[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(plaintext))
		if err != nil {
			return nil, fmt.Errorf("gzip.NewReader: %w", err)
		}

		data, readErr := io.ReadAll(r)

		closeErr := r.Close()
		if readErr != nil {
			return nil, fmt.Errorf("gzip.ReadAll: %w", errors.Join(readErr, closeErr))
		}

		if closeErr != nil {
			return nil, fmt.Errorf("gzip.Close: %w", closeErr)
		}
		return data, nil
	}
	return plaintext, nil
}

func aesECBEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	padded, err := Pkcs7Padding(plaintext, block.BlockSize())
	if err != nil {
		return nil, fmt.Errorf("Pkcs7Padding: %w", err)
	}
	return AesEncryptECB(block, padded), nil
}

func aesECBDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	decrypted, err := AesDecryptECB(block, ciphertext)
	if err != nil {
		return nil, err
	}
	return Pkcs7UnPadding(decrypted)
}

func dynamicKey(session Session) ([]byte, string, error) {
	if strings.TrimSpace(session.Key) != "" {
		// x-encr-sskey 是服务端下发的 ASCII 字符串，形似 hex 时也不能做 hex.DecodeString。
		key := []byte(session.Key)
		switch len(key) {
		case 16, 24, 32:
			return key, session.ID, nil
		default:
			return nil, "", fmt.Errorf("%w: got %d bytes", ErrSessionKeyLength, len(key))
		}
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

func encryptS(dynamicKey []byte, publicKey PublicKeyState, os string) ([]byte, error) {
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

	plaintext := []byte(base64.StdEncoding.EncodeToString(dynamicKey) + "|" + os + "|" + publicKey.SK)

	iv, err := xeapiRandomBytes(12)
	if err != nil {
		return nil, fmt.Errorf("cryptorand gcm iv: %w", err)
	}

	key := deriveX25519AESKey(sharedSecret, ephemeralRaw)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher S: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	encrypted := gcm.Seal(nil, iv, plaintext, nil)

	out := make([]byte, 0, len(ephemeralRaw)+len(iv)+len(encrypted))
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
