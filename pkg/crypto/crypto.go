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

package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"time"
)

const (
	base62      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	idXORKey1   = "3go8&$8*3*3h0k(2)2"
	cacheKey    = ")(13daqP@ssw0rd~"
	iv          = "0102030405060708"
	presetKey   = "0CoJUm6Qyw8W8jud"
	linuxApiKey = "rFgB&h#%2?^eDg:Q"
	eApiKey     = "e82ckenh8dichen8"
	eApiFormat  = "%s-36cd479b6b5-%s-36cd479b6b5-%s"
	eApiSlat    = "nobody%suse%smd5forencrypt"
	publicKey   = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDgtQn2JZ34ZC28NWYpAUd98iZ37BUrX/aKzmFbt7clFSs6sXqHauqKWqdtLkF2KexO40H1YTX8z2lSgBBOAxLsvaklV8k4cBFK9snQXE9/DDaFt6Rr7iVZMldczhC0JNgTz+SHXT6CBHuX3e9SdB1Ua44oncaTWz7OBGLbCiK45wIDAQAB
-----END PUBLIC KEY-----`
)

func randomKey() string {
	var buffer bytes.Buffer
	for i := 0; i < 16; i++ {
		buffer.WriteByte(base62[rand.Int63n(62)])
	}
	return buffer.String()
}

func reverseString(str string) string {
	var runes = []rune(str)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func digest(url, data string) string {
	var message = fmt.Sprintf(eApiSlat, url, data)
	return fmt.Sprintf("%x", md5.Sum([]byte(message)))
}

// aesEncrypt 加密
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

// aesDecrypt 解密
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

// AesEncryptCBC 加密
func AesEncryptCBC(block cipher.Block, plaintext, iv []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	encrypt := cipher.NewCBCEncrypter(block, iv)
	encrypt.CryptBlocks(ciphertext, plaintext)
	return ciphertext
}

// AesDecryptCBC 解密
func AesDecryptCBC(block cipher.Block, cipherText, iv []byte) ([]byte, error) {
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("IV length must be %d bytes", block.BlockSize())
	}

	data := make([]byte, len(cipherText))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, cipherText)
	return data, nil
}

// AesEncryptECB 加密
func AesEncryptECB(block cipher.Block, plaintext []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	blockSize := block.BlockSize()
	for i := 0; i < len(plaintext); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], plaintext[i:i+blockSize])
	}
	return ciphertext
}

// AesDecryptECB 解密
func AesDecryptECB(block cipher.Block, cipherBytes []byte) ([]byte, error) {
	if len(cipherBytes)%block.BlockSize() != 0 {
		return nil, errors.New("cipherBytes length is not a multiple of block size")
	}
	var decrypted = make([]byte, len(cipherBytes))
	for i := 0; i < len(cipherBytes); i += block.BlockSize() {
		block.Decrypt(decrypted[i:i+block.BlockSize()], cipherBytes[i:i+block.BlockSize()])
	}
	return decrypted, nil
}

// RsaEncrypt 公钥加密无填充方式
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

	// 使用noPadding方式填充
	c := new(big.Int).SetBytes([]byte(ciphertext))
	encryptedBytes := c.Exp(c, big.NewInt(int64(pubKey.E)), pubKey.N).Bytes()
	return hex.EncodeToString(encryptedBytes), nil
}

// Pkcs7Padding 补码,严格遵循 RFC 5652 规范
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

// Pkcs7UnPadding 去码
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

// WeApiEncrypt 加密
func WeApiEncrypt(object interface{}) (map[string]string, error) {
	var secretKey = randomKey()
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
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

// WeApiDecrypt 解密 TODO: 由于拿不到私钥则不能解密
func WeApiDecrypt(params, encSecKey string) (map[string]string, error) {
	panic("unrealized")
	return nil, nil
}

// LinuxApiEncrypt 加密
func LinuxApiEncrypt(object interface{}) (map[string]string, error) {
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

// LinuxApiDecrypt 解密
func LinuxApiDecrypt(cipherText string) ([]byte, error) {
	plaintext, err := aesDecrypt(cipherText, linuxApiKey, "", "ecb", "hex")
	if err != nil {
		return nil, fmt.Errorf("aesDecrypt: %w", err)
	}
	return plaintext, nil
}

// EApiEncrypt 加密
// 通常在MAC、windows、android、ios中使用
// todo: 貌似当url为空时存在问题,网易接口加密返回中有不带url的情况，
// 例如: DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08
func EApiEncrypt(url string, object interface{}) (map[string]string, error) {
	// 需要替换路由地址,不然会出现接口未找到错误
	url = strings.Replace(url, "eapi", "api", 1)
	data, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf(eApiFormat, url, string(data), digest(url, string(data)))
	// fmt.Println("payload:", text)

	ciphertext, err := aesEncrypt(text, eApiKey, "", "ecb", "HEX")
	if err != nil {
		return nil, fmt.Errorf("aesEncrypt: %w", err)
	}
	return map[string]string{"params": ciphertext}, nil
}

// EApiDecrypt 解密,当解析请求参数是encode使用hex,当解析请求响应参数为空相当于二进制
func EApiDecrypt(ciphertext, encode string) ([]byte, error) {
	plaintext, err := aesDecrypt(ciphertext, eApiKey, "", "ecb", encode)
	if err != nil {
		return nil, fmt.Errorf("aesDecrypt: %w", err)
	}
	return plaintext, nil
}

// CacheKeyEncrypt 生成缓存 key
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

// CacheKeyDecrypt 解密缓存 key
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

func DLLEncodeID(someID string) (string, error) {
	inputBytes := []byte(someID)
	xor := make([]byte, len(inputBytes))
	keyLength := len(idXORKey1)

	// 执行异或操作
	for i, c := range inputBytes {
		xor[i] = c ^ idXORKey1[i%keyLength]
	}

	// 计算MD5哈希+Base64编码
	hasher := md5.New()
	hasher.Write(xor)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil)), nil
}

// Anonymous 匿名用户生成
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

// GenerateWNMCID 生成WNMCID
// 生成规则: 6位随机小写字母 + 当前时间戳（毫秒） + 默认抓取版本号 + 0
// 例如: "abcdef.1633557080686.01.0"
// 作用: 貌似是网易云音乐的抓取标识,或者用于爬虫标识等作用
func GenerateWNMCID() string {
	const (
		crawlerVersion = "01" // 默认抓取版本号
		charset        = "abcdefghijklmnopqrstuvwxyz"
	)
	// 1. 生成6位随机小写字母
	b := make([]byte, 6)
	for i := range b {
		// 从字符集中随机选取字符（0-25）
		b[i] = charset[rand.Intn(len(charset))]
	}

	// 2. 获取当前时间戳（毫秒）
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	// 3. 拼接最终字符串
	return fmt.Sprintf("%s.%d.%s.0", string(b), timestamp, crawlerVersion)
}

func HexDigest(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
