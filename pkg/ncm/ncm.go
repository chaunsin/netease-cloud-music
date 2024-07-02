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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dhowden/tag"
)

type CoverType string

const (
	CoverTypeUnknown CoverType = "unknown"
	CoverTypePng     CoverType = "png"
	CoverTypeJpeg    CoverType = "jpeg"
)

func (c CoverType) FileType() string {
	return string(c)
}

func (c CoverType) MIME() string {
	switch c {
	case CoverTypeJpeg:
		return "image/jpeg"
	case CoverTypePng:
		return "image/png"
	case CoverTypeUnknown:
		fallthrough
	default:
		return "unknown"
	}
}

var (
	pngPrefix  = []byte("\x89PNG\x0D\x0A\x1A\x0A")
	jpegPrefix = []byte("\xFF\xD8\xFF")
)

func DetectCoverType(data []byte) CoverType {
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

	// metaLen <=0 that means no metadata
	if metaLen <= 0 {
		data, err := DecodeMusic(rs)
		if err != nil {
			return nil, fmt.Errorf("DecodeMusic: %w", err)
		}

		m, err := tag.ReadFrom(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("tag.ReadFrom: %w", err)
		}

		// no metadata see as a music type
		meta.mt = "music"
		meta.music.Format = strings.ToLower(string(m.FileType()))
		// usually empty, try your best to get information
		meta.music.Name = m.Title()
		meta.music.Album = m.Album()
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

	sep := bytes.IndexByte(data, ':')
	if sep == -1 {
		return nil, errors.New("invalid ncm meta file")
	}

	meta.mt = MetadataType(data[:sep])
	switch meta.mt {
	case "music":
		if err := json.Unmarshal(data[sep+1:], &meta.music); err != nil {
			return nil, fmt.Errorf("json.Unmarshal.music: %w", err)
		}
	case "dj":
		if err := json.Unmarshal(data[sep+1:], &meta.dj); err != nil {
			return nil, fmt.Errorf("json.Unmarshal.dj: %w", err)
		}
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
	return imgData, DetectCoverType(imgData), nil
}

func DecodeMusic(rs io.ReadSeeker) ([]byte, error) {
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
			bj := box[j]
			tb[i] ^= box[(bj+box[(bj+j)&0xff])&0xff]
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

	// check file is ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, fmt.Errorf("IsNCMFile: %w", err)
	}

	// jump over the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, err
	}

	// decode music key
	box, err := decodeKey(rs)
	if err != nil {
		return nil, fmt.Errorf("decodeKey: %w", err)
	}

	// start decode metadata
	{
		// get metadata length
		var metaBuf = make([]byte, 4)
		metaLen, err := readUint32(metaBuf, rs)
		if err != nil {
			return nil, fmt.Errorf("readUint32.metaBuf: %w", err)
		}

		// read metadata
		var metadata = make([]byte, metaLen)
		if _, err = rs.Read(metadata); err != nil {
			return nil, fmt.Errorf("metadata: %w", err)
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
			return nil, fmt.Errorf("decryptAes128Ecb: %w", err)
		}

		sep := bytes.IndexByte(meta, ':')
		if sep == -1 {
			return nil, errors.New("invalid ncm meta file")
		}

		var md = Metadata{mt: MetadataType(meta[:sep])}
		switch md.mt {
		case "music":
			if err := json.Unmarshal(meta[sep+1:], &md.music); err != nil {
				return nil, fmt.Errorf("json.Unmarshal.music: %w", err)
			}
		case "dj":
			if err := json.Unmarshal(meta[sep+1:], &md.dj); err != nil {
				return nil, fmt.Errorf("json.Unmarshal.dj: %w", err)
			}
		}
		ncm.metadata = &md
		// fmt.Printf("metadata: %+v\n", ncm.metadata)
	}

	// start decode cover
	{
		// 5 bytes gap + 4 bytes image crc
		if _, err := rs.Seek(9, io.SeekCurrent); err != nil {
			return nil, err
		}

		// get cover image length
		var imgBuf = make([]byte, 4)
		imgLen, err := readUint32(imgBuf, rs)
		if err != nil {
			return nil, fmt.Errorf("readUint32.imgBuf: %w", err)
		}

		// get cover image data
		var imgData = make([]byte, imgLen)
		if _, err = rs.Read(imgData); err != nil {
			return nil, fmt.Errorf("imgData: %w", err)
		}
		ncm.cover = imgData
		ncm.coverType = DetectCoverType(imgData)
	}

	// decode music data
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
