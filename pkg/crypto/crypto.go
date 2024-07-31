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
)

const (
	base62      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
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

	var cipherText []byte
	switch mode {
	case "cbc":
		cipherText = encryptCBC(block, []byte(text), []byte(iv))
	case "ecb":
		cipherText = encryptECB(block, []byte(text))
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
		text, err = decryptCBC(block, data, []byte(iv))
	case "ecb":
		text, err = decryptECB(block, data)
	default:
		return nil, fmt.Errorf("%s unknown mode", mode)
	}
	if err != nil {
		return nil, fmt.Errorf("mode: %w", err)
	}
	return text, nil
}

// encryptCBC 加密
func encryptCBC(block cipher.Block, plaintext, iv []byte) []byte {
	plaintext = pkcs7Padding(plaintext, block.BlockSize())
	ciphertext := make([]byte, len(plaintext))
	encrypt := cipher.NewCBCEncrypter(block, iv)
	encrypt.CryptBlocks(ciphertext, plaintext)
	return ciphertext
}

// decryptCBC 解密
func decryptCBC(block cipher.Block, cipherText, iv []byte) ([]byte, error) {
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("IV length must be %d bytes", block.BlockSize())
	}

	data := make([]byte, len(cipherText))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, cipherText)
	return data, nil
}

// encryptECB 加密
func encryptECB(block cipher.Block, plaintext []byte) []byte {
	plaintext = pkcs7Padding(plaintext, block.BlockSize())
	ciphertext := make([]byte, len(plaintext))
	blockSize := block.BlockSize()
	for i := 0; i < len(plaintext); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], plaintext[i:i+blockSize])
	}
	return ciphertext
}

// decryptECB 解密
func decryptECB(block cipher.Block, cipherBytes []byte) ([]byte, error) {
	if len(cipherBytes)%block.BlockSize() != 0 {
		return nil, errors.New("cipherBytes length is not a multiple of block size")
	}
	var decrypted = make([]byte, len(cipherBytes))
	for i := 0; i < len(cipherBytes); i += block.BlockSize() {
		block.Decrypt(decrypted[i:i+block.BlockSize()], cipherBytes[i:i+block.BlockSize()])
	}
	return pkcs7UnPadding(decrypted), nil
}

// rsaEncrypt 公钥加密无填充方式
func rsaEncrypt(ciphertext, key string) (string, error) {
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

// pkcs7Padding 补码
func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, pad...)
}

// pkcs7UnPadding 去码
func pkcs7UnPadding(origData []byte) []byte {
	length := len(origData)
	pad := int(origData[length-1])
	return origData[:(length - pad)]
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
	encSecKey, err := rsaEncrypt(reverseString(secretKey), publicKey)
	if err != nil {
		return nil, fmt.Errorf("rsaEncrypt: %w", err)
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
// - eparams
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

// EApiDecrypt 解密,当解析请求参数时encode使用hex,当解析请求响应参数时则为空相当于二进制
// - params
func EApiDecrypt(ciphertext, encode string) ([]byte, error) {
	plaintext, err := aesDecrypt(ciphertext, eApiKey, "", "ecb", encode)
	if err != nil {
		return nil, fmt.Errorf("aesDecrypt: %w", err)
	}
	return plaintext, nil
}
