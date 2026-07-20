// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/andybalholm/brotli"
)

func decodeHTTPContent(raw []byte, contentEncoding string, limit int64) ([]byte, bool, error) {
	encodings := strings.Split(strings.ToLower(contentEncoding), ",")
	data := raw

	for _, v := range slices.Backward(encodings) {
		encoding := strings.TrimSpace(v)
		if encoding == "" || encoding == "identity" {
			continue
		}

		var (
			err       error
			truncated bool
		)

		data, truncated, err = decodeOneContentEncoding(data, encoding, limit)
		if err != nil {
			return raw, false, err
		}

		if truncated {
			return data, true, nil
		}
	}
	return data, false, nil
}

func decodeOneContentEncoding(raw []byte, encoding string, limit int64) ([]byte, bool, error) {
	var (
		reader io.Reader
		closer io.Closer
	)

	switch encoding {
	case "gzip", "x-gzip":
		gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, false, fmt.Errorf("gzip reader: %w", err)
		}

		reader, closer = gzipReader, gzipReader
	case "deflate":
		zlibReader, err := zlib.NewReader(bytes.NewReader(raw))
		if err == nil {
			reader, closer = zlibReader, zlibReader
		} else {
			flateReader := flate.NewReader(bytes.NewReader(raw))
			reader, closer = flateReader, flateReader
		}
	case "br":
		reader = brotli.NewReader(bytes.NewReader(raw))
	default:
		return nil, false, fmt.Errorf("unsupported content encoding %q", encoding)
	}

	decoded, truncated, readErr := readLimited(reader, limit)

	var closeErr error
	if closer != nil {
		closeErr = closer.Close()
	}

	if readErr != nil {
		return nil, false, fmt.Errorf("decode %s: %w", encoding, readErr)
	}

	if closeErr != nil {
		return nil, false, fmt.Errorf("close %s decoder: %w", encoding, closeErr)
	}
	return decoded, truncated, nil
}

// readLimited avoids the common limit+1 overflow trap while still probing for
// a byte beyond the display budget. The returned data is never larger than
// limit, even when callers pass math.MaxInt64.
func readLimited(reader io.Reader, limit int64) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, errors.New("decoded body limit must be greater than zero")
	}

	data, err := io.ReadAll(&io.LimitedReader{R: reader, N: limit})
	if err != nil {
		return nil, false, err
	}

	if int64(len(data)) < limit {
		return data, false, nil
	}

	var probe [1]byte
	for {
		n, readErr := reader.Read(probe[:])
		if n > 0 {
			return data, true, nil
		}

		if readErr == io.EOF {
			return data, false, nil
		}

		if readErr != nil {
			return nil, false, readErr
		}
	}
}
