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
