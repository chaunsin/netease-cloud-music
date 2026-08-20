// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncm

import (
	"bytes"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type shortReadSeeker struct {
	io.ReadSeeker

	max int
}

func (r *shortReadSeeker) Read(p []byte) (int, error) {
	return r.ReadSeeker.Read(p[:min(len(p), r.max)])
}

func applyMusicCipher(box, input []byte) []byte {
	output := append([]byte(nil), input...)
	for i := range output {
		j := byte((i + 1) & 0xff)
		bj := box[j]
		output[i] ^= box[(bj+box[(bj+j)&0xff])&0xff]
	}
	return output
}

func TestDecryptMusicHandlesShortReads(t *testing.T) {
	t.Parallel()

	box := buildKeyBox([]byte("short-read-regression"))

	plaintext := make([]byte, 10_321)
	for i := range plaintext {
		plaintext[i] = byte((i*31 + 17) & 0xff)
	}

	ciphertext := applyMusicCipher(box, plaintext)

	for _, maxRead := range []int{1, 255, 256, 1000, 4096} {
		t.Run(strconv.Itoa(maxRead), func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer

			reader := &shortReadSeeker{
				ReadSeeker: bytes.NewReader(ciphertext),
				max:        maxRead,
			}

			require.NoError(t, decryptMusic(box, reader, &output))
			require.Equal(t, plaintext, output.Bytes())
		})
	}
}
