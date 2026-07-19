// Copyright (c) 2025-2026 chaunsin
// SPDX-License-Identifier: MIT

package cookiecloud

import (
	"crypto/aes"
	"crypto/md5" //nolint:gosec // CookieCloud's OpenSSL-compatible format requires MD5 key derivation.
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

const (
	pkcs5SaltLen = 8
	aes256KeyLen = 32
	keyLen       = 32
	blockLen     = 16
)

func newLegacyMD5() hash.Hash {
	return md5.New() //nolint:gosec // Required for CookieCloud/OpenSSL EVP_BytesToKey compatibility.
}

// Decrypt a CryptoJS.AES.encrypt(msg, password) encrypted msg.
// Ciphertext is the result of CryptoJS.AES.encrypt(), which is the base64 string of
// "Salted__" + [8 bytes random salt] + [actual ciphertext].
// Actual ciphertext is padded (make it's length align with block length) using Pkcs7.
// CryptoJS use a OpenSSL-compatible EVP_BytesToKey to derive (key,iv) from (password,salt),
// using md5 as hash type and 32 / 16 as length of key / block.
// See: https://stackoverflow.com/questions/35472396/how-does-cryptojs-get-an-iv-when-none-is-specified ,
// https://stackoverflow.com/questions/64797987/what-is-the-default-aes-config-in-crypto-js
func Decrypt(password, ciphertext string) ([]byte, error) {
	if len(password) < 16 {
		return nil, errors.New("password length must be greater than 16")
	}

	if ciphertext == "" {
		return nil, errors.New("ciphertext is empty")
	}

	rawEncrypted, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode Encrypted: %w", err)
	}

	if len(rawEncrypted) < 17 ||
		len(rawEncrypted)%blockLen != 0 ||
		string(rawEncrypted[:8]) != "Salted__" {
		return nil, errors.New("invalid ciphertext")
	}

	var (
		salt      = rawEncrypted[8:16]
		encrypted = rawEncrypted[16:]
	)

	key, iv, err := BytesToKey(salt, []byte(password), newLegacyMD5(), keyLen, blockLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key and iv: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create aes cipher: %w", err)
	}

	decrypted, err := crypto.AesDecryptCBC(block, encrypted, iv)
	if err != nil {
		return nil, fmt.Errorf("AesDecryptCBC: %w", err)
	}
	return crypto.Pkcs7UnPadding(decrypted)
}

// Encrypt encrypts the plaintext using the password.
func Encrypt(password, plaintext string) (string, error) {
	if len(password) < 16 {
		return "", errors.New("password length must be greater than 16")
	}

	if plaintext == "" {
		return "", errors.New("plaintext is empty")
	}

	// 1. 生成随机 salt
	salt := make([]byte, pkcs5SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// 2. 通过 EVP_BytesToKey 派生密钥和 IV
	key, iv, err := BytesToKey(salt, []byte(password), newLegacyMD5(), keyLen, blockLen)
	if err != nil {
		return "", fmt.Errorf("failed to derive key and iv: %w", err)
	}

	// 3. 创建 AES 加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create aes cipher: %w", err)
	}

	// 4. 使用 Pkcs7 填充明文
	padding, err := crypto.Pkcs7Padding([]byte(plaintext), block.BlockSize())
	if err != nil {
		return "", fmt.Errorf("Pkcs7Padding: %w", err)
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
func BytesToKey(salt, data []byte, h hash.Hash, keyLen, blockLen int) ([]byte, []byte, error) {
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

// BytesToKeyAES256CBC implements the SHA256 version of EVP_BytesToKey using AES CBC.
func BytesToKeyAES256CBC(salt, data []byte) ([]byte, []byte, error) {
	return BytesToKey(salt, data, sha256.New(), aes256KeyLen, aes.BlockSize)
}

// BytesToKeyAES256CBCMD5 implements the MD5 version of EVP_BytesToKey using AES CBC.
func BytesToKeyAES256CBCMD5(salt, data []byte) ([]byte, []byte, error) {
	return BytesToKey(salt, data, newLegacyMD5(), aes256KeyLen, aes.BlockSize)
}

// Md5String return the MD5 hex hash string (lower-case) of input string(s).
func Md5String(inputs ...string) string {
	digest := md5.Sum([]byte(strings.Join(inputs, ""))) //nolint:gosec // CookieCloud identifiers use the legacy MD5 format.
	return hex.EncodeToString(digest[:])
}
