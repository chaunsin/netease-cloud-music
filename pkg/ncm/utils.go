// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type CoverType string

const (
	CoverTypeUnknown CoverType = "unknown"
	CoverTypePng     CoverType = "png"
	CoverTypeJpeg    CoverType = "jpeg"
	CoverTypeBmp     CoverType = "bmp"
	CoverTypeWebp    CoverType = "webp"
	CoverTypeGif     CoverType = "gif"
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
	case CoverTypeBmp:
		return "image/bmp"
	case CoverTypeWebp:
		return "image/webp"
	case CoverTypeGif:
		return "image/gif"
	case CoverTypeUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

var (
	pngPrefix  = []byte("\x89PNG\x0D\x0A\x1A\x0A")
	jpegPrefix = []byte("\xFF\xD8\xFF")
	bmpPrefix  = []byte("BM")
	webpPrefix = []byte("RIFF")
	gifPrefix  = []byte("GIF8")
)

func DetectCoverType(data []byte) CoverType {
	if len(data) < 2 {
		return CoverTypeUnknown
	}

	if bytes.HasPrefix(data, jpegPrefix) {
		return CoverTypeJpeg
	}

	if bytes.HasPrefix(data, pngPrefix) {
		return CoverTypePng
	}

	if bytes.HasPrefix(data, bmpPrefix) {
		return CoverTypeBmp
	}

	if bytes.HasPrefix(data, webpPrefix) {
		return CoverTypeWebp
	}

	if bytes.HasPrefix(data, gifPrefix) {
		return CoverTypeGif
	}

	return CoverTypeUnknown
}

func readUint32(rBuf []byte, rs io.ReadSeeker) (uint32, error) {
	if n, err := rs.Read(rBuf); err != nil {
		return uint32(n), fmt.Errorf("read: %w", err)
	}
	return binary.LittleEndian.Uint32(rBuf), nil
}
