// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package crypto

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXeapiPublicKeyStateIsValid(t *testing.T) {
	valid := XeapiPublicKeyState{
		PublicKey: "public-key",
		Version:   "v1",
		SK:        "server-key",
	}

	tests := []struct {
		name  string
		state XeapiPublicKeyState
		want  bool
	}{
		{name: "complete without expiration", state: valid, want: true},
		{name: "complete with future expiration", state: func() XeapiPublicKeyState {
			state := valid
			state.NextUpdateTime = time.Now().Add(time.Hour).UnixMilli()
			return state
		}(), want: true},
		{name: "expired", state: func() XeapiPublicKeyState {
			state := valid
			state.NextUpdateTime = time.Now().Add(-time.Hour).UnixMilli()
			return state
		}(), want: false},
		{name: "missing public key", state: XeapiPublicKeyState{Version: "v1", SK: "server-key"}},
		{name: "missing version", state: XeapiPublicKeyState{PublicKey: "public-key", SK: "server-key"}},
		{name: "missing server key", state: XeapiPublicKeyState{PublicKey: "public-key", Version: "v1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.IsValid())
		})
	}
}

func TestBuildPlaintextEnvelope(t *testing.T) {
	t.Run("default request omits method and content type", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI: "/api/song/detail?id=101",
		})

		assert.Equal(t, "id=101&e_r=true", envelope["queryString"])
		assert.NotContains(t, envelope, "body")
		assert.NotContains(t, envelope, "url")
		assert.NotContains(t, envelope, "method")
		assert.NotContains(t, envelope, "contentType")
	})

	t.Run("non default request keeps method and content type", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI:         "https://interface.music.163.com/api/song/detail?ids=1",
			Method:      http.MethodPut,
			ContentType: "application/json",
			Data: map[string]any{
				"id":   123,
				"name": "hello world",
				"e_r":  true,
			},
		})

		assert.Equal(t, "ids=1&e_r=true", envelope["queryString"])
		assert.Equal(t, http.MethodPut, envelope["method"])
		assert.Equal(t, "application/json", envelope["contentType"])

		body, err := base64.StdEncoding.DecodeString(envelope["body"])
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":123,"name":"hello world","e_r":true}`, string(body))
	})

	t.Run("form data removes e_r", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI: "/api/test",
			Data: url.Values{
				"id":  []string{"1"},
				"e_r": []string{"false"},
			},
		})

		assert.Equal(t, "e_r=true", envelope["queryString"])
		body, err := base64.StdEncoding.DecodeString(envelope["body"])
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "1", values.Get("id"))
		assert.False(t, values.Has("e_r"))
	})

	t.Run("existing query e_r still appends encrypted response flag", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI: "/api/test?e_r=false&id=1",
		})

		assert.Equal(t, "e_r=false&id=1&e_r=true", envelope["queryString"])
	})

	t.Run("raw body preserves exact bytes", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI:         "/api/test",
			ContentType: "application/json",
			Body:        []byte(`{"id":1,"e_r":false}`),
		})

		assert.Equal(t, "e_r=true", envelope["queryString"])
		body, err := base64.StdEncoding.DecodeString(envelope["body"])
		require.NoError(t, err)
		assert.Equal(t, `{"id":1,"e_r":false}`, string(body))
	})

	t.Run("empty raw body is still present", func(t *testing.T) {
		plaintext, err := buildPlaintextEnvelope(&XeapiEncryptRequest{
			URI:  "/api/test",
			Body: []byte{},
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{"body":"","queryString":"e_r=true"}`, string(plaintext))
	})

	t.Run("struct data follows json tags", func(t *testing.T) {
		type request struct {
			ID      int            `json:"id"`
			ER      bool           `json:"e_r"`
			Empty   string         `json:"empty,omitempty"`
			Ignore  string         `json:"-"`
			Options map[string]int `json:"options"`
		}

		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI: "/api/test",
			Data: request{
				ID:      123,
				ER:      true,
				Ignore:  "ignored",
				Options: map[string]int{"level": 1},
			},
		})

		body, err := base64.StdEncoding.DecodeString(envelope["body"])
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "123", values.Get("id"))
		assert.Equal(t, `{"level":1}`, values.Get("options"))
		assert.False(t, values.Has("e_r"))
		assert.False(t, values.Has("empty"))
		assert.False(t, values.Has("Ignore"))
	})

	t.Run("default content type tolerates spaces around parameters", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &XeapiEncryptRequest{
			URI:         "/api/test",
			ContentType: "application/x-www-form-urlencoded ; charset=utf-8",
			Data:        map[string]string{"id": "1"},
		})

		assert.NotContains(t, envelope, "contentType")
		body, err := base64.StdEncoding.DecodeString(envelope["body"])
		require.NoError(t, err)
		assert.Equal(t, "id=1", string(body))
	})
}

func TestXeapiStaticKey(t *testing.T) {
	assert.Len(t, xeapiStaticKey, 32)

	want := XeapiPublicKeyState{
		PublicKey:      "test-public-key",
		Version:        "v1",
		NextUpdateTime: 123456789,
		SK:             "server-key",
		DeviceID:       "device-id",
	}
	plain, err := json.Marshal(want)
	require.NoError(t, err)
	ciphertext, err := aesECBEncrypt(xeapiStaticKey, plain)
	require.NoError(t, err)

	got, err := XeapiDecryptPublicKey(base64.StdEncoding.EncodeToString(ciphertext))
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, want, *got)

	_, err = aesECBEncrypt(xeapiStaticKey, []byte(want.Version+"|session-id"))
	require.NoError(t, err)

	missingPublicKey := want
	missingPublicKey.PublicKey = ""
	plain, err = json.Marshal(missingPublicKey)
	require.NoError(t, err)
	ciphertext, err = aesECBEncrypt(xeapiStaticKey, plain)
	require.NoError(t, err)
	_, err = XeapiDecryptPublicKey(base64.StdEncoding.EncodeToString(ciphertext))
	assert.ErrorIs(t, err, ErrPublicKeyMissing)
}

func TestXeapiSign(t *testing.T) {
	got := XeapiSign("1710000000000", "nonce-123")
	assert.Equal(t, "bKpwwK7JsV1jXJO21nxNpGX0w9Np8HCktvJcQJNcm8E=", got)

	// Issue #174 的公钥刷新样例可证明 signKey 应按原始 ASCII 字节参与 HMAC。
	got = XeapiSign("1779955010033", "4477405878624231")
	assert.Equal(t, "d6ouZ8bOiQrsH6kfslwG9zhJMvF6sJ4DCOlsGUkk7fw=", got)
}

func TestXeapiEncryptIssue174GoldenBody(t *testing.T) {
	dynamicKey, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	require.NoError(t, err)

	transformRandom := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	gcmIV := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	privateKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	stubXeapiRandomness(t, [][]byte{transformRandom, dynamicKey, gcmIV}, privateKey)

	req := XeapiEncryptRequest{
		URI:  "/api/song/enhance/location/info",
		Body: []byte{},
		OS:   "android",
	}
	plaintext, err := buildPlaintextEnvelope(&req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"body":"","queryString":"e_r=true"}`, string(plaintext))

	got, err := XeapiEncrypt(&req, XeapiPublicKeyState{
		PublicKey:      "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:        "1000000000000",
		NextUpdateTime: 1803882269000,
		SK:             "8PZfbIFA1779944463972",
	}, XeapiSession{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"B": "J5+3SnVyE16Pm4720e7gA3mgIZ1L4axkB6jte8X079wgjs3SU+IK7AANKKdewVLtBIJw5y5LtyhCcJ3FZm4u2LOfXnKdOC0VKIfVgX/lWloAZX6hQGVaRHgnR3BdQi+t",
		"S": "B6N8vBQgk8i3VdwbEOhstCY3StFqqFPtC9/AsrhtHHwAAQIDBAUGBwgJCguNFV1OAc3Z5noM7bYwvLwNFBK0H8NY/JVdIRN2dRDdG1JrMTLDI/ArlqMSIXdq9rfulgMKqRO7imtYLn8PrI4cIbwOdSkz",
		"R": "3LCoCTuHo/mDfZ1x3PtHsQ==",
	}, got)
}

func TestXeapiEncrypt(t *testing.T) {
	curve := ecdh.X25519()
	peer, err := curve.GenerateKey(cryptorand.Reader)
	require.NoError(t, err)

	publicKey := XeapiPublicKeyState{
		PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey().Bytes()),
		Version:   "v1",
		SK:        "server-key",
	}
	_, err = XeapiEncrypt(nil, publicKey, XeapiSession{})
	require.ErrorIs(t, err, ErrEncryptRequestMissing)

	req := XeapiEncryptRequest{
		URI:  "/api/song/detail?id=1",
		Data: map[string]any{"id": 1, "e_r": true},
	}

	withSession, err := XeapiEncrypt(&req, publicKey, XeapiSession{ID: "session-id", Key: "0123456789abcdef"})
	require.NoError(t, err)
	withoutSession, err := XeapiEncrypt(&req, publicKey, XeapiSession{})
	require.NoError(t, err)

	for _, item := range []map[string]string{withSession, withoutSession} {
		for _, key := range []string{"B", "S", "R"} {
			assert.NotEmpty(t, item[key])
			_, decodeErr := base64.StdEncoding.DecodeString(item[key])
			require.NoError(t, decodeErr)
		}
	}

	assert.NotEqual(t, withSession["R"], withoutSession["R"])

	plaintext := decryptXeapiB(t, withSession["B"], []byte("0123456789abcdef"))

	var envelope map[string]string
	require.NoError(t, json.Unmarshal(plaintext, &envelope))
	assert.Equal(t, "id=1&e_r=true", envelope["queryString"])
	body, err := base64.StdEncoding.DecodeString(envelope["body"])
	require.NoError(t, err)
	assert.Equal(t, "id=1", string(body))

	rPlaintext := decryptXeapiR(t, withSession["R"])
	assert.Equal(t, "v1|session-id", string(rPlaintext))

	sPlaintext := decryptXeapiS(t, withSession["S"], peer)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))+"|android|server-key", string(sPlaintext))

	_, err = XeapiEncrypt(&req, XeapiPublicKeyState{
		PublicKey: publicKey.PublicKey,
		Version:   "v1",
	}, XeapiSession{})
	assert.ErrorIs(t, err, ErrServerKeyMissing)
}

func TestXeapiDecryptResponse(t *testing.T) {
	t.Run("plain json", func(t *testing.T) {
		ciphertext, err := aesECBEncrypt([]byte(eApiKey), []byte(`{"code":200}`))
		require.NoError(t, err)

		got, err := XeapiDecryptResponse(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, `{"code":200}`, string(got))
	})

	t.Run("gzip json", func(t *testing.T) {
		var buf bytes.Buffer

		zw := gzip.NewWriter(&buf)
		_, err := zw.Write([]byte(`{"code":201}`))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		ciphertext, err := aesECBEncrypt([]byte(eApiKey), buf.Bytes())
		require.NoError(t, err)

		got, err := XeapiDecryptResponse(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, `{"code":201}`, string(got))
	})
}

func decodeXeapiEnvelope(t *testing.T, req *XeapiEncryptRequest) map[string]string {
	t.Helper()

	plain, err := buildPlaintextEnvelope(req)
	require.NoError(t, err)

	var envelope map[string]string
	require.NoError(t, json.Unmarshal(plain, &envelope))
	return envelope
}

func decryptXeapiB(t *testing.T, encryptedB string, dynamicKey []byte) []byte {
	t.Helper()

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedB)
	require.NoError(t, err)
	mid, err := aesECBDecrypt(dynamicKey, ciphertext)
	require.NoError(t, err)

	if len(mid) < 16 {
		t.Fatalf("midTransform payload too short: %d", len(mid))
	}

	random := mid[:16]
	rotated := mid[16:]

	rot := 0
	if len(rotated) > 0 {
		rot = int(random[0]&0x0f) % len(rotated)
	}

	b64 := make([]byte, 0, len(rotated))
	b64 = append(b64, rotated[len(rotated)-rot:]...)
	b64 = append(b64, rotated[:len(rotated)-rot]...)

	xored, err := base64.StdEncoding.DecodeString(string(b64))
	require.NoError(t, err)

	inner := make([]byte, len(xored))
	for i := range xored {
		inner[i] = xored[i] ^ random[i&0x0f]
	}

	plaintext, err := aesECBDecrypt(xeapiStaticKey, inner)
	require.NoError(t, err)
	return plaintext
}

func decryptXeapiR(t *testing.T, encryptedR string) []byte {
	t.Helper()

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedR)
	require.NoError(t, err)
	plaintext, err := aesECBDecrypt(xeapiStaticKey, ciphertext)
	require.NoError(t, err)
	return plaintext
}

func decryptXeapiS(t *testing.T, encryptedS string, peer *ecdh.PrivateKey) []byte {
	t.Helper()

	payload, err := base64.StdEncoding.DecodeString(encryptedS)
	require.NoError(t, err)

	if len(payload) < 32+12 {
		t.Fatalf("S payload too short: %d", len(payload))
	}

	ephemeralRaw := payload[:32]
	iv := payload[32:44]
	ciphertext := payload[44:]

	curve := ecdh.X25519()
	ephemeral, err := curve.NewPublicKey(ephemeralRaw)
	require.NoError(t, err)
	sharedSecret, err := peer.ECDH(ephemeral)
	require.NoError(t, err)

	key := deriveX25519AESKey(sharedSecret, ephemeralRaw)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	require.NoError(t, err)
	return plaintext
}

func stubXeapiRandomness(t *testing.T, randoms [][]byte, privateKey []byte) {
	t.Helper()

	oldRandomBytes := xeapiRandomBytes
	oldGenerateKey := xeapiGenerateX25519Key

	var nextRandom int

	xeapiRandomBytes = func(length int) ([]byte, error) {
		if nextRandom >= len(randoms) {
			return nil, fmt.Errorf("unexpected xeapi random request for %d bytes", length)
		}

		data := randoms[nextRandom]
		nextRandom++

		if len(data) != length {
			return nil, fmt.Errorf("xeapi random length: got %d want %d", len(data), length)
		}
		return append([]byte(nil), data...), nil
	}
	xeapiGenerateX25519Key = func(curve ecdh.Curve) (*ecdh.PrivateKey, error) {
		return curve.NewPrivateKey(privateKey)
	}

	t.Cleanup(func() {
		xeapiRandomBytes = oldRandomBytes
		xeapiGenerateX25519Key = oldGenerateKey

		assert.Equal(t, len(randoms), nextRandom)
	})
}
