// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package cookie

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync(t *testing.T) {
	cookiePath := os.TempDir() + "cookie.json"

	t.Cleanup(func() {
		_ = os.Remove(cookiePath)
	})

	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(cookiePath))
	require.NoError(t, err)

	u := &url.URL{Scheme: "https", Host: "example.com"}
	ck := []*http.Cookie{{Name: "token", Value: "pwd123"}, {Name: "email", Value: "test@example.com"}}
	jar.SetCookies(u, ck)

	data, err := os.ReadFile(cookiePath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	t.Logf("data:%s\n", string(data))
	// assert.JSONEq(t, string(data), target)
}
