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

// IsNCMFile check whether the file is ncm file
func IsNCMFile(rs io.ReadSeeker) error {
	if rs == nil {
		return errors.New("io.ReadSeeker is nil")
	}
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
	if rs == nil {
		return nil, errors.New("io.ReadSeeker is nil")
	}

	// detect whether ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, fmt.Errorf("IsNCMFile: %w", err)
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

	// deKeyData length 130
	deKeyData, err := decryptAes128Ecb(aesCoreKey, fixBlockSize(keyData))
	if err != nil {
		return nil, err
	}
	// deKeyData[:17] = len("neteasecloudmusic") = 17
	return buildKeyBox(deKeyData[17:]), nil
}

// DecodeMeta decode meta info
func DecodeMeta(rs io.ReadSeeker) (*Metadata, error) {
	if rs == nil {
		return nil, errors.New("io.ReadSeeker is nil")
	}

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

	var meta Metadata
	// metaLen <= 0 that means no metadata, use detect music header magic.
	if metaLen <= 0 {
		// whether a decoded key is successful
		box, err := DecodeKey(rs)
		if err != nil {
			return nil, fmt.Errorf("DecodeKey: %w", err)
		}

		// skip get cover image data
		if err := DecodeCover(rs, io.Discard); err != nil {
			return nil, fmt.Errorf("DecodeCover: %w", err)
		}

		// get music header magic
		var data = make([]byte, 11)
		if _, err := rs.Read(data); err != nil {
			if err == io.EOF {
			} else {
				return nil, err
			}
		}
		for i := 0; i < len(data); i++ {
			j := byte((i + 1) & 0xff)
			bj := box[j]
			data[i] ^= box[(bj+box[(bj+j)&0xff])&0xff]
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
	// fmt.Println("modifyData length:", len(modifyData))

	data, err := decryptAes128Ecb(aesModifyKey, fixBlockSize(modifyData))
	if err != nil {
		return nil, fmt.Errorf("decryptAes128Ecb: %w", err)
	}

	var sep = bytes.IndexByte(data, ':')
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

func decodeCover(rs io.ReadSeeker) ([]byte, int64, error) {
	// detect whether ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, 0, fmt.Errorf("IsNCMFile: %w", err)
	}

	// jump the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, 0, err
	}

	// whether a decoded key is successful
	var keyBuf = make([]byte, 4)
	keyLen, err := readUint32(keyBuf, rs)
	if err != nil {
		return nil, 0, fmt.Errorf("readUint32.keyBuf: %w", err)
	}

	if _, err := rs.Seek(int64(keyLen), io.SeekCurrent); err != nil {
		return nil, 0, err
	}

	// get metadata length
	var metaBuf = make([]byte, 4)
	metaLen, err := readUint32(metaBuf, rs)
	if err != nil {
		return nil, 0, fmt.Errorf("readUint32.metaBuf: %w", err)
	}
	if metaLen > 0 {
		if _, err := rs.Seek(int64(metaLen), io.SeekCurrent); err != nil {
			return nil, 0, err
		}
	}

	// 5 bytes gap + 4 bytes image crc
	if _, err := rs.Seek(9, io.SeekCurrent); err != nil {
		return nil, 0, err
	}

	// get cover image length
	var imgBuf = make([]byte, 4)
	imgLen, err := readUint32(imgBuf, rs)
	if err != nil {
		return nil, 0, fmt.Errorf("readUint32.imgBuf: %w", err)
	}

	// detect a cover image type
	var imgType = make([]byte, 8)
	if _, err = rs.Read(imgType); err != nil {
		return nil, 0, fmt.Errorf("readUint32.imgType: %w", err)
	}
	return imgType, int64(imgLen) - 8, nil
}

func DecodeCoverType(rs io.ReadSeeker) (CoverType, error) {
	if rs == nil {
		return CoverTypeUnknown, errors.New("io.ReadSeeker is nil")
	}

	data, _, err := decodeCover(rs)
	if err != nil {
		return CoverTypeUnknown, fmt.Errorf("decodeCover: %w", err)
	}
	return DetectCoverType(data), nil
}

// DecodeCover decode cover image
func DecodeCover(rs io.ReadSeeker, w io.Writer) error {
	if rs == nil || w == nil {
		return errors.New("io.ReadSeeker or io.Writer is nil")
	}

	imgType, imgLen, err := decodeCover(rs)
	if err != nil {
		return fmt.Errorf("decodeCover: %w", err)
	}
	if _, err := w.Write(imgType); err != nil {
		return fmt.Errorf("write imgType: %w", err)
	}

	// copy image data to w
	if _, err := io.Copy(w, io.LimitReader(rs, imgLen)); err != nil {
		return err
	}
	return nil
}

func DecodeMusic(rs io.ReadSeeker, w io.Writer) error {
	if rs == nil || w == nil {
		return errors.New("io.ReadSeeker or io.Writer is nil")
	}

	// whether a decoded key is successful
	box, err := DecodeKey(rs)
	if err != nil {
		return fmt.Errorf("DecodeKey: %w", err)
	}

	// get cover image data
	if err := DecodeCover(rs, io.Discard); err != nil {
		return fmt.Errorf("DecodeCover: %w", err)
	}
	_, err = decryptMusic(box, rs, w)
	return err
}

type NCM struct {
	rs io.ReadSeeker

	box []byte

	metadataOffset int64
	metadata       *Metadata

	coverOffset int64
	coverType   CoverType

	musicOffset int64
}

type File struct {
	f *os.File
	*NCM
}

func (f *File) Close() error {
	return f.f.Close()
}

func Open(filename string) (*File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	n, err := FromReadSeeker(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("FromReadSeeker: %w", err)
	}
	return &File{f: file, NCM: n}, nil
}

func FromReadSeeker(rs io.ReadSeeker) (*NCM, error) {
	if rs == nil {
		return nil, errors.New("io.ReadSeeker is nil")
	}

	var ncm = NCM{rs: rs}

	// check file is ncm file
	if err := IsNCMFile(rs); err != nil {
		return nil, fmt.Errorf("IsNCMFile: %w", err)
	}

	// jump over the gap(2).
	if _, err := rs.Seek(2, io.SeekCurrent); err != nil {
		return nil, err
	}

	// decode music key, result length 256
	box, err := decodeKey(rs)
	if err != nil {
		return nil, fmt.Errorf("decodeKey: %w", err)
	}
	ncm.box = box

	// decode metadata
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

		var sep = bytes.IndexByte(meta, ':')
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
	}

	// decode cover
	{
		// 5 bytes gap + 4 bytes image crc
		offset, err := rs.Seek(9, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		ncm.coverOffset = offset

		// get cover image length
		var imgBuf = make([]byte, 4)
		imgLen, err := readUint32(imgBuf, rs)
		if err != nil {
			return nil, fmt.Errorf("readUint32.imgBuf: %w", err)
		}

		// skip the image data to the music data offset
		offset, err = rs.Seek(int64(imgLen), io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		ncm.musicOffset = offset
	}
	return &ncm, nil
}

func (n *NCM) Metadata() *Metadata {
	if n.metadata != nil {
		return n.metadata
	}
	return nil
}

func (n *NCM) DecodeCoverType() (CoverType, error) {
	offset, err := n.rs.Seek(n.coverOffset, io.SeekStart)
	if err != nil {
		return CoverTypeUnknown, fmt.Errorf("seek(%v): %w", n.coverOffset, err)
	}
	_ = offset

	// get cover image length
	var imgBuf = make([]byte, 4)
	imgLen, err := readUint32(imgBuf, n.rs)
	if err != nil {
		return CoverTypeUnknown, fmt.Errorf("readUint32.imgBuf: %w", err)
	}
	if imgLen <= 0 {
		return CoverTypeUnknown, errors.New("invalid cover image length")
	}

	// detect a cover image type
	var imgType = make([]byte, 8)
	if _, err = n.rs.Read(imgType); err != nil {
		return CoverTypeUnknown, fmt.Errorf("readUint32.imgType: %w", err)
	}
	return DetectCoverType(imgType), nil
}

func (n *NCM) DecodeCover(w io.Writer) error {
	if w == nil {
		return errors.New("io.Writer is nil")
	}
	offset, err := n.rs.Seek(n.coverOffset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek(%v): %w", n.coverOffset, err)
	}
	_ = offset

	// get cover image length
	var imgBuf = make([]byte, 4)
	imgLen, err := readUint32(imgBuf, n.rs)
	if err != nil {
		return fmt.Errorf("readUint32.imgBuf: %w", err)
	}

	// copy image data to w
	if _, err := io.Copy(w, io.LimitReader(n.rs, int64(imgLen))); err != nil {
		return err
	}
	return nil
}

func (n *NCM) DecodeMusic(w io.Writer) error {
	if w == nil {
		return errors.New("io.Writer is nil")
	}
	offset, err := n.rs.Seek(n.musicOffset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek(%v): %w", n.musicOffset, err)
	}
	_ = offset
	_, err = decryptMusic(n.box, n.rs, w)
	return err
}
