// MIT License
//
// Copyright (c) 2025 chaunsin
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

package cookiecloud

import (
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

const (
	pkcs5SaltLen = 8
	aes256KeyLen = 32
	keyLen       = 32
	blockLen     = 16
)

// Decrypt a CryptoJS.AES.encrypt(msg, password) encrypted msg.
// ciphertext is the result of CryptoJS.AES.encrypt(), which is the base64 string of
// "Salted__" + [8 bytes random salt] + [actual ciphertext].
// actual ciphertext is padded (make it's length align with block length) using Pkcs7.
// CryptoJS use a OpenSSL-compatible EVP_BytesToKey to derive (key,iv) from (password,salt),
// using md5 as hash type and 32 / 16 as length of key / block.
// See: https://stackoverflow.com/questions/35472396/how-does-cryptojs-get-an-iv-when-none-is-specified ,
// https://stackoverflow.com/questions/64797987/what-is-the-default-aes-config-in-crypto-js
func Decrypt(password string, ciphertext string) ([]byte, error) {
	if len(password) < 16 {
		return nil, fmt.Errorf("password length must be greater than 16")
	}
	if ciphertext == "" {
		return nil, fmt.Errorf("ciphertext is empty")
	}

	rawEncrypted, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode Encrypted: %v", err)
	}
	if len(rawEncrypted) < 17 ||
		len(rawEncrypted)%blockLen != 0 ||
		string(rawEncrypted[:8]) != "Salted__" {
		return nil, fmt.Errorf("invalid ciphertext")
	}

	var (
		salt      = rawEncrypted[8:16]
		encrypted = rawEncrypted[16:]
	)

	key, iv, err := BytesToKey(salt, []byte(password), md5.New(), keyLen, blockLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key and iv: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create aes cipher: %v", err)
	}
	decrypted, err := crypto.AesDecryptCBC(block, encrypted, iv)
	if err != nil {
		return nil, fmt.Errorf("AesDecryptCBC: %v", err)
	}
	return crypto.Pkcs7UnPadding(decrypted)
}

// Encrypt encrypts the plaintext using the password.
func Encrypt(password string, plaintext string) (string, error) {
	if len(password) < 16 {
		return "", fmt.Errorf("password length must be greater than 16")
	}
	if plaintext == "" {
		return "", fmt.Errorf("plaintext is empty")
	}

	// 1. 生成随机 salt
	salt := make([]byte, pkcs5SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %v", err)
	}

	// 2. 通过 EVP_BytesToKey 派生密钥和 IV
	key, iv, err := BytesToKey(salt, []byte(password), md5.New(), keyLen, blockLen)
	if err != nil {
		return "", fmt.Errorf("failed to derive key and iv: %v", err)
	}

	// 3. 创建 AES 加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create aes cipher: %v", err)
	}

	// 4. 使用 Pkcs7 填充明文
	padding, err := crypto.Pkcs7Padding([]byte(plaintext), block.BlockSize())
	if err != nil {
		return "", fmt.Errorf("Pkcs7Padding: %v", err)
	}

	// 5. 使用 AES-CBC 模式加密
	encrypted := crypto.AesEncryptCBC(block, padding, iv)

	// 6. 组合 OpenSSL 格式输出 ("Salted__" + salt + encrypted)
	finalOutput := append([]byte("Salted__"), salt...)
	finalOutput = append(finalOutput, encrypted...)

	// 7. 返回 Base64 编码的加密结果
	return base64.StdEncoding.EncodeToString(finalOutput), nil
}

// BytesToKey
// From https://github.com/walkert/go-evp .
// BytesToKey implements the Openssl EVP_BytesToKey logic.
// It takes the salt, data, a hash type and the key/block length used by that type.
// As such it differs considerably from the openssl method in C.
func BytesToKey(salt, data []byte, h hash.Hash, keyLen, blockLen int) (key, iv []byte, err error) {
	saltLen := len(salt)
	if saltLen > 0 && saltLen != pkcs5SaltLen {
		return nil, nil, fmt.Errorf("salt length is %d, expected %d", saltLen, pkcs5SaltLen)
	}
	var (
		concat   []byte
		lastHash []byte
		totalLen = keyLen + blockLen
	)
	for ; len(concat) < totalLen; h.Reset() {
		// concatenate lastHash, data and salt and write them to the hash
		h.Write(append(lastHash, append(data, salt...)...))
		// passing nil to Sum() will return the current hash value
		lastHash = h.Sum(nil)
		// append lastHash to the running total bytes
		concat = append(concat, lastHash...)
	}
	return concat[:keyLen], concat[keyLen:totalLen], nil
}

// BytesToKeyAES256CBC implements the SHA256 version of EVP_BytesToKey using AES CBC
func BytesToKeyAES256CBC(salt, data []byte) (key []byte, iv []byte, err error) {
	return BytesToKey(salt, data, sha256.New(), aes256KeyLen, aes.BlockSize)
}

// BytesToKeyAES256CBCMD5 implements the MD5 version of EVP_BytesToKey using AES CBC
func BytesToKeyAES256CBCMD5(salt, data []byte) (key []byte, iv []byte, err error) {
	return BytesToKey(salt, data, md5.New(), aes256KeyLen, aes.BlockSize)
}

// Md5String return the MD5 hex hash string (lower-case) of input string(s)
func Md5String(inputs ...string) string {
	keyHash := md5.New()
	for _, str := range inputs {
		io.WriteString(keyHash, str)
	}
	return hex.EncodeToString(keyHash.Sum(nil))
}
