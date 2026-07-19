// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCookieData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   string
		format string
	}{
		{
			name:   "auto-detect JSON string",
			data:   `[{"domain":".music.163.com","name":"MUSIC_U","path":"/","value":"token"}]`,
			format: "",
		},
		{
			name:   "explicit header data",
			data:   "MUSIC_U=token; __csrf=csrf",
			format: "header",
		},
		{
			name: "auto-detect Netscape data",
			data: "# Netscape HTTP Cookie File\n" +
				".music.163.com\tTRUE\t/\tTRUE\t1893456000\tMUSIC_U\ttoken\n",
			format: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cookies, err := parseCookieData([]byte(tt.data), tt.format)
			require.NoError(t, err)
			require.NotEmpty(t, cookies)
			assert.Equal(t, "MUSIC_U", cookies[0].Name)
			assert.Equal(t, "token", cookies[0].Value)
		})
	}
}

func TestParseCookieDataRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	_, err := parseCookieData([]byte("MUSIC_U=token"), "unknown")
	require.ErrorContains(t, err, `unsupported cookie format "unknown"`)
}
