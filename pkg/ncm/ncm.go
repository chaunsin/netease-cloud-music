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
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type CoverType string

const (
	CoverTypeUnknown CoverType = "unknown"
	CoverTypePng     CoverType = "png"
	CoverTypeJpeg    CoverType = "jpeg"
)

var (
	pngPrefix  = []byte("\x89PNG\x0D\x0A\x1A\x0A")
	jpegPrefix = []byte("\xFF\xD8\xFF")
)

func coverType(data []byte) CoverType {
	if bytes.HasPrefix(data, jpegPrefix) {
		return CoverTypeJpeg
	}
	if bytes.HasPrefix(data, pngPrefix) {
		return CoverTypePng
	}
	return CoverTypeUnknown
}

func readUint32(rBuf []byte, rs io.ReadSeeker) (uint32, error) {
	if n, err := rs.Read(rBuf); err != nil {
		return uint32(n), fmt.Errorf("read: %w", err)
	}
	return binary.LittleEndian.Uint32(rBuf), nil
}

// IsNCMFile check whether the file is ncm file
func IsNCMFile(rs io.ReadSeeker) error {
	// Jump to begin of file
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// var rBuf = make([]byte, 4)
	// uLen, err := readUint32(rBuf, rs)
	// if err != nil {
	// 	return fmt.Errorf("readUint32.rBuf: %w", err)
	// }
	// if uLen != 0x4e455443 {
	// 	return fmt.Errorf("isn't netease cloud music copyright file")
	// }
	//
	// uLen, err = readUint32(rBuf, rs)
	// if err != nil {
	// 	return fmt.Errorf("readUint32.uLen: %w", err)
	// }
	// if uLen != 0x4d414446 {
	// 	return fmt.Errorf("isn't netease cloud music copyright file")
	// }

	var header = make([]byte, 8)
	if err := binary.Read(rs, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("binary.Read: %w", err)
	}
	if string(header) != "CTENFDAM" {
		return fmt.Errorf("isn't netease cloud music copyright file detail: %s", string(header))
	}
	return nil
}

// DecodeKey decode key
func DecodeKey(rs io.ReadSeeker) ([]byte, error) {
	// detect whether ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, err
	}

	// jump over and the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, err
	}
	return decodeKey(rs)
}

func decodeKey(rs io.ReadSeeker) ([]byte, error) {
	var keyBuf = make([]byte, 4)
	keyLen, err := readUint32(keyBuf, rs)
	if err != nil {
		return nil, err
	}

	var keyData = make([]byte, keyLen)
	if _, err := rs.Read(keyData); err != nil {
		return nil, err
	}
	for i := range keyData {
		keyData[i] ^= 0x64
	}

	deKeyData, err := decryptAes128Ecb(aesCoreKey, fixBlockSize(keyData))
	if err != nil {
		return nil, err
	}
	// deKeyData[:17] = len("neteasecloudmusic") = 17
	return buildKeyBox(deKeyData[17:]), nil
}

// DecodeMeta decode meta info
func DecodeMeta(rs io.ReadSeeker) (*Metadata, error) {
	var meta Metadata
	// detect whether ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, fmt.Errorf("IsNCMFile: %w", err)
	}

	// jump over and the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, err
	}

	// whether a decoded key is successful
	var keyBuf = make([]byte, 4)
	keyLen, err := readUint32(keyBuf, rs)
	if err != nil {
		return nil, fmt.Errorf("readUint32.keyBuf: %w", err)
	}

	if _, err := rs.Seek(int64(keyLen), io.SeekCurrent); err != nil {
		return nil, err
	}

	// get metadata length
	var metaBuf = make([]byte, 4)
	metaLen, err := readUint32(metaBuf, rs)
	if err != nil {
		return nil, fmt.Errorf("readUint32.metaBuf: %w", err)
	}

	// metaLen <=0 说明没有音乐相关metadata数据
	if metaLen <= 0 {
		fmt.Println("metadata len <= 0")
		// TODO(chaunsin): 此处是直接比较文件大小来判定文件格式待优化？
		meta.Format = "flac"
		// if info, err := rs.Stat(); err != nil && info.Size() < int64(math.Pow(1024, 2)*16) {
		// 	meta.Format = "mp3"
		// }
		return &meta, nil
	}

	var metadata = make([]byte, metaLen)
	if _, err = rs.Read(metadata); err != nil {
		return nil, fmt.Errorf("read.metadata: %w", err)
	}
	for i := range metadata {
		metadata[i] ^= 0x63
	}

	// 22 = len(`163 key(Don't modify):`)
	var modifyData = make([]byte, base64.StdEncoding.DecodedLen(len(metadata)-22))
	if _, err = base64.StdEncoding.Decode(modifyData, metadata[22:]); err != nil {
		return nil, fmt.Errorf("base64.Decode: %w", err)
	}

	data, err := decryptAes128Ecb(aesModifyKey, fixBlockSize(modifyData))
	if err != nil {
		return nil, fmt.Errorf("decryptAes128Ecb: %w", err)
	}

	// 6 = len("music:")
	data = data[6:]
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &meta, nil
}

// DecodeCover decode cover image
func DecodeCover(rs io.ReadSeeker) ([]byte, CoverType, error) {
	// detect whether ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, CoverTypeUnknown, fmt.Errorf("IsNCMFile: %w", err)
	}

	// jump the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, CoverTypeUnknown, err
	}

	// whether a decoded key is successful
	var keyBuf = make([]byte, 4)
	keyLen, err := readUint32(keyBuf, rs)
	if err != nil {
		return nil, CoverTypeUnknown, fmt.Errorf("readUint32.keyBuf: %w", err)
	}

	if _, err := rs.Seek(int64(keyLen), io.SeekCurrent); err != nil {
		return nil, CoverTypeUnknown, err
	}

	// get metadata length
	var metaBuf = make([]byte, 4)
	metaLen, err := readUint32(metaBuf, rs)
	if err != nil {
		return nil, CoverTypeUnknown, fmt.Errorf("readUint32.metaBuf: %w", err)
	}
	if metaLen > 0 {
		if _, err := rs.Seek(int64(metaLen), io.SeekCurrent); err != nil {
			return nil, CoverTypeUnknown, err
		}
	}

	// 5 bytes gap + 4 bytes image crc
	if _, err := rs.Seek(9, io.SeekCurrent); err != nil {
		return nil, CoverTypeUnknown, err
	}

	// get cover image length
	var imgBuf = make([]byte, 4)
	imgLen, err := readUint32(imgBuf, rs)
	if err != nil {
		return nil, CoverTypeUnknown, fmt.Errorf("readUint32.imgBuf: %w", err)
	}

	// get cover image data
	var imgData = make([]byte, imgLen)
	if _, err = rs.Read(imgData); err != nil {
		return nil, CoverTypeUnknown, fmt.Errorf("readUint32.imgData: %w", err)
	}
	return imgData, coverType(imgData), nil
}

func Decode(rs io.ReadSeeker) ([]byte, error) {
	// detect whether ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, err
	}

	// whether a decoded key is successful
	box, err := DecodeKey(rs)
	if err != nil {
		return nil, err
	}

	// get cover image data
	if _, _, err := DecodeCover(rs); err != nil {
		return nil, err
	}

	return decodeMusic(box, rs)
}

func decodeMusic(box []byte, rs io.ReadSeeker) ([]byte, error) {
	var (
		n   = 0x8000
		buf bytes.Buffer
		tb  = make([]byte, n)
	)

	for {
		if _, err := rs.Read(tb); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		for i := 0; i < n; i++ {
			j := byte((i + 1) & 0xff)
			tb[i] ^= box[(box[j]+box[(box[j]+j)&0xff])&0xff]
		}
		if _, err := buf.Write(tb); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

type NCM struct {
	metadata  *Metadata
	cover     []byte
	coverType CoverType
	music     []byte
	valid     bool
}

func Open(filename string) (*NCM, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return NewReadSeeker(file)
}

func NewReadSeeker(rs io.ReadSeeker) (*NCM, error) {
	var ncm NCM

	// 校验
	if err := IsNCMFile(rs); err != nil {
		return nil, err
	}

	// jump over the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, err
	}

	// 解析出key
	box, err := decodeKey(rs)
	if err != nil {
		return nil, fmt.Errorf("decodeKey: %w", err)
	}

	// === 解析metadata开始 ===
	// get metadata length
	var metaBuf = make([]byte, 4)
	metaLen, err := readUint32(metaBuf, rs)
	if err != nil {
		return nil, err
	}

	// read metadata
	var metadata = make([]byte, metaLen)
	if _, err = rs.Read(metadata); err != nil {
		return nil, err
	}
	for i := range metadata {
		metadata[i] ^= 0x63
	}

	// 22 = len(`163 key(Don't modify):`)
	var modifyData = make([]byte, base64.StdEncoding.DecodedLen(len(metadata)-22))
	if _, err = base64.StdEncoding.Decode(modifyData, metadata[22:]); err != nil {
		return nil, err
	}

	meta, err := decryptAes128Ecb(aesModifyKey, fixBlockSize(modifyData))
	if err != nil {
		return nil, err
	}
	meta = meta[6:] // 6 = len("music:")

	if err := json.Unmarshal(meta, &ncm.metadata); err != nil {
		return nil, err
	}
	// === 解析metadata结束 ===

	// === 解析cover开始 ===
	// 5 bytes gap + 4 bytes image crc
	if _, err := rs.Seek(9, io.SeekCurrent); err != nil {
		return nil, err
	}

	// get cover image length
	var imgBuf = make([]byte, 4)
	imgLen, err := readUint32(imgBuf, rs)
	if err != nil {
		return nil, err
	}

	// get cover image data
	var imgData = make([]byte, imgLen)
	if _, err = rs.Read(imgData); err != nil {
		return nil, err
	}
	ncm.cover = imgData
	ncm.coverType = coverType(imgData)
	// === 解析cover结束 ===

	// 解析出歌曲
	ncm.music, err = decodeMusic(box, rs)
	if err != nil {
		return nil, fmt.Errorf("decodeMusic: %w", err)
	}
	return &ncm, nil
}

func (n *NCM) Metadata() *Metadata {
	if n.metadata != nil {
		return n.metadata
	}
	return nil
}

func (n *NCM) Cover() ([]byte, CoverType) {
	if len(n.cover) > 0 {
		return n.cover, n.coverType
	}
	return nil, CoverTypeUnknown
}

func (n *NCM) Music() []byte {
	if len(n.music) > 0 {
		return n.music
	}
	return nil
}
