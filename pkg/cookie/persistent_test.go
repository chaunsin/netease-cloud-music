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

package cookie

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSync(t *testing.T) {
	filepath := os.TempDir() + "cookie.json"
	t.Cleanup(func() {
		_ = os.Remove(filepath)
	})

	jar, err := NewPersistentJar(WithSyncInterval(0), WithFilePath(filepath))
	assert.NoError(t, err)

	u := &url.URL{Scheme: "https", Host: "example.com"}
	ck := []*http.Cookie{{Name: "token", Value: "pwd123"}, {Name: "email", Value: "test@example.com"}}
	jar.SetCookies(u, ck)

	data, err := os.ReadFile(filepath)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	t.Logf("data:%s\n", string(data))
	// assert.JSONEq(t, string(data), target)
}
