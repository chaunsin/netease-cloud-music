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

package ncm

import (
	"bufio"
	"crypto/aes"
	"errors"
	"fmt"
	"io"
)

var (
	// aesCoreKey 用于解密音乐部分数据使用
	aesCoreKey = []byte{0x68, 0x7A, 0x48, 0x52, 0x41, 0x6D, 0x73, 0x6F, 0x35, 0x6B, 0x49, 0x6E, 0x62, 0x61, 0x78, 0x57}
	// aesModifyKey 用于解密metadata使用
	aesModifyKey = []byte{0x23, 0x31, 0x34, 0x6C, 0x6A, 0x6B, 0x5F, 0x21, 0x5C, 0x5D, 0x26, 0x30, 0x55, 0x3C, 0x27, 0x28}
)

func buildKeyBox(key []byte) []byte {
	var (
		box                    = make([]byte, 256)
		keyLen                 = byte(len(key))
		c, lastByte, keyOffset byte
	)
	for i := 0; i < 256; i++ {
		box[i] = byte(i)
	}

	for i := 0; i < 256; i++ {
		c = (box[i] + lastByte + key[keyOffset]) & 0xff
		keyOffset++
		if keyOffset >= keyLen {
			keyOffset = 0
		}
		box[i], box[c] = box[c], box[i]
		lastByte = c
	}
	return box
}

func fixBlockSize(src []byte) []byte {
	return src[:len(src)/aes.BlockSize*aes.BlockSize]
}

func pkcs7UnPadding(src []byte) ([]byte, error) {
	var (
		length    = len(src)
		unPadding = int(src[length-1])
	)
	if length == 0 {
		return nil, errors.New("pkcs7: invalid length")
	}
	if unPadding > length || unPadding == 0 {
		return nil, errors.New("pkcs7: invalid unPadding")
	}
	for i := 0; i < unPadding; i++ {
		if src[length-1-i] != byte(unPadding) {
			return nil, errors.New("pkcs7: invalid padding full")
		}
	}
	return src[:(length - unPadding)], nil
}

func decryptAes128Ecb(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	var (
		dataLen   = len(data)
		decrypted = make([]byte, dataLen)
		bs        = block.BlockSize()
	)
	for i := 0; i <= dataLen-bs; i += bs {
		block.Decrypt(decrypted[i:i+bs], data[i:i+bs])
	}
	return pkcs7UnPadding(decrypted)
}

func decryptAes128EcbStream(key []byte, input io.Reader, output io.Writer) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	var (
		bs  = block.BlockSize()
		buf = make([]byte, bs)
	)

	for {
		n, err := input.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		decrypted := make([]byte, bs)
		block.Decrypt(decrypted, buf[:n])

		if n < bs {
			decrypted, err = pkcs7UnPadding(decrypted)
			if err != nil {
				return fmt.Errorf("pkcs7UnPadding: %w", err)
			}
		}

		_, err = output.Write(decrypted)
		if err != nil {
			return err
		}
	}
	return nil
}

func decryptMusic(box []byte, rs io.ReadSeeker, w io.Writer) ([]byte, error) {
	var (
		size   = 4096
		isRead = false
		header = make([]byte, 11)
		data   = make([]byte, size)
		bw     = bufio.NewWriter(w)
	)

	for {
		if _, err := rs.Read(data); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		for i := 0; i < size; i++ {
			j := byte((i + 1) & 0xff)
			bj := box[j]
			data[i] ^= box[(bj+box[(bj+j)&0xff])&0xff]
		}
		if !isRead {
			copy(header, data[:11])
			isRead = true
		}
		if _, err := bw.Write(data); err != nil {
			return nil, err
		}
	}
	return header, bw.Flush()
}
