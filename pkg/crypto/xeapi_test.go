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
			assert.Equal(t, tt.want, tt.state.Validate() == nil)
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

func TestFormValuesDoesNotDeleteCallerKeys(t *testing.T) {
	t.Run("url values", func(t *testing.T) {
		source := url.Values{"e_r": []string{"false"}, "id": []string{"1"}}

		values, err := formValues(source)
		require.NoError(t, err)
		values.Del("e_r")

		assert.Equal(t, []string{"false"}, source["e_r"])
	})

	t.Run("string slice map", func(t *testing.T) {
		source := map[string][]string{"e_r": {"false"}, "id": {"1"}}

		values, err := formValues(source)
		require.NoError(t, err)
		values.Del("e_r")

		assert.Equal(t, []string{"false"}, source["e_r"])
	})
}

func TestXeapiStaticKey(t *testing.T) {
	assert.Len(t, xeapiStaticKey, 32)

	want := XeapiPublicKeyState{
		PublicKey:      "test-public-key",
		Version:        "v1",
		NextUpdateTime: 4102444800000,
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
	require.ErrorIs(t, err, ErrPublicKeyMissing)

	missingVersion := want
	missingVersion.Version = ""
	plain, err = json.Marshal(missingVersion)
	require.NoError(t, err)
	ciphertext, err = aesECBEncrypt(xeapiStaticKey, plain)
	require.NoError(t, err)
	_, err = XeapiDecryptPublicKey(base64.StdEncoding.EncodeToString(ciphertext))
	require.ErrorIs(t, err, ErrPublicKeyVersionMissing)

	expired := want
	expired.NextUpdateTime = time.Now().Add(-time.Minute).UnixMilli()
	plain, err = json.Marshal(expired)
	require.NoError(t, err)
	ciphertext, err = aesECBEncrypt(xeapiStaticKey, plain)
	require.NoError(t, err)
	_, err = XeapiDecryptPublicKey(base64.StdEncoding.EncodeToString(ciphertext))
	require.ErrorIs(t, err, ErrPublicKeyExpired)
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
		NextUpdateTime: 0,
		SK:             "8PZfbIFA1779944463972",
	}, XeapiSession{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"B": "J5+3SnVyE16Pm4720e7gA3mgIZ1L4axkB6jte8X079wgjs3SU+IK7AANKKdewVLtBIJw5y5LtyhCcJ3FZm4u2LOfXnKdOC0VKIfVgX/lWloAZX6hQGVaRHgnR3BdQi+t",
		"S": "B6N8vBQgk8i3VdwbEOhstCY3StFqqFPtC9/AsrhtHHwAAQIDBAUGBwgJCguNFV1OAc3Z5noM7bYwvLwNFBK0H8NY/JVdIRN2dRDdG1JrMTLDI/ArlqMSIXdq9rfulgMKqRO7imtYLn8PrI4cIbwOdSkz",
		"R": "3LCoCTuHo/mDfZ1x3PtHsQ==",
	}, got)

	decrypted, err := XeapiDecryptRequest(XeapiEncryptedRequest{B: got["B"], S: got["S"], R: got["R"]}, dynamicKey)
	require.NoError(t, err)
	assert.Equal(t, "1000000000000", decrypted.PublicKeyVersion)
	assert.Empty(t, decrypted.SessionID)
	assert.JSONEq(t, `{"body":"","queryString":"e_r=true"}`, string(decrypted.Plaintext))
	assert.True(t, decrypted.SFrameValid)
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

	decrypted, err := XeapiDecryptRequest(
		XeapiEncryptedRequest{B: withSession["B"], S: withSession["S"], R: withSession["R"]},
		[]byte("0123456789abcdef"),
	)
	require.NoError(t, err)
	assert.Equal(t, "v1", decrypted.PublicKeyVersion)
	assert.Equal(t, "session-id", decrypted.SessionID)

	var envelope map[string]string
	require.NoError(t, json.Unmarshal(decrypted.Plaintext, &envelope))
	assert.Equal(t, "id=1&e_r=true", envelope["queryString"])
	body, err := base64.StdEncoding.DecodeString(envelope["body"])
	require.NoError(t, err)
	assert.Equal(t, "id=1", string(body))

	sPlaintext := decryptXeapiS(t, withSession["S"], peer)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))+"|android|server-key", string(sPlaintext))

	_, err = XeapiEncrypt(&req, XeapiPublicKeyState{
		PublicKey: publicKey.PublicKey,
		Version:   "v1",
	}, XeapiSession{})
	require.ErrorIs(t, err, ErrServerKeyMissing)

	_, err = XeapiEncrypt(&req, XeapiPublicKeyState{
		PublicKey: publicKey.PublicKey,
		SK:        publicKey.SK,
	}, XeapiSession{})
	require.ErrorIs(t, err, ErrPublicKeyVersionMissing)

	expired := publicKey
	expired.NextUpdateTime = time.Now().Add(-time.Minute).UnixMilli()
	_, err = XeapiEncrypt(&req, expired, XeapiSession{})
	require.ErrorIs(t, err, ErrPublicKeyExpired)
}

func TestDynamicKeyValidatesSessionPairAndRawKeyLength(t *testing.T) {
	for _, key := range []string{
		"0123456789abcdef",
		"0123456789abcdefghijklmn",
		"0123456789abcdefghijklmnopqrstuv",
		" 123456789abcde ",
	} {
		gotKey, gotID, err := dynamicKey(XeapiSession{ID: "session-id", Key: key})
		require.NoError(t, err)
		assert.Equal(t, key, string(gotKey))
		assert.Equal(t, "session-id", gotID)
	}

	_, _, err := dynamicKey(XeapiSession{ID: "session-id"})
	require.ErrorIs(t, err, ErrSessionIncomplete)

	_, _, err = dynamicKey(XeapiSession{Key: "0123456789abcdef"})
	require.ErrorIs(t, err, ErrSessionIncomplete)

	_, _, err = dynamicKey(XeapiSession{ID: "session-id", Key: "too-short"})
	require.ErrorIs(t, err, ErrSessionKeyLength)
}

func TestXeapiDecryptRequestRejectsMalformedInput(t *testing.T) {
	dynamicKey := []byte("0123456789abcdef")
	validR := encryptXeapiTestValue(t, xeapiStaticKey, []byte("v1|session-id"))
	invalidR := encryptXeapiTestValue(t, xeapiStaticKey, []byte("missing-separator"))
	emptyVersionR := encryptXeapiTestValue(t, xeapiStaticKey, []byte("|session-id"))
	extraSeparatorR := encryptXeapiTestValue(t, xeapiStaticKey, []byte("v1|session|extra"))
	oversizedPaddingR := encryptXeapiTestBlocks(t, xeapiStaticKey, append(
		[]byte("v1|session-idxx"),
		bytes.Repeat([]byte{17}, 17)...,
	))
	shortMid := encryptXeapiTestValue(t, dynamicKey, make([]byte, aes.BlockSize))
	invalidMidBase64 := encryptXeapiTestValue(t, dynamicKey, append(make([]byte, aes.BlockSize), '!'))
	invalidInnerMid := append(
		make([]byte, aes.BlockSize),
		[]byte(base64.StdEncoding.EncodeToString(make([]byte, aes.BlockSize)))...,
	)
	invalidInner := encryptXeapiTestValue(t, dynamicKey, invalidInnerMid)

	tests := []struct {
		name    string
		req     XeapiEncryptedRequest
		key     []byte
		wantErr string
	}{
		{name: "missing R", req: XeapiEncryptedRequest{}, wantErr: "xeapi R is empty"},
		{name: "invalid R base64", req: XeapiEncryptedRequest{R: "%%%"}, wantErr: "base64.DecodeString R"},
		{name: "invalid R plaintext", req: XeapiEncryptedRequest{R: invalidR}, wantErr: "invalid R plaintext"},
		{name: "empty R version", req: XeapiEncryptedRequest{R: emptyVersionR}, wantErr: "invalid R plaintext"},
		{name: "extra R separator", req: XeapiEncryptedRequest{R: extraSeparatorR}, wantErr: "invalid R plaintext"},
		{name: "oversized R padding", req: XeapiEncryptedRequest{R: oversizedPaddingR}, wantErr: "invalid padding size"},
		{name: "missing B", req: XeapiEncryptedRequest{R: validR}, key: dynamicKey, wantErr: "xeapi B is empty"},
		{name: "invalid dynamic key", req: XeapiEncryptedRequest{B: "AA==", R: validR}, key: []byte("short"), wantErr: "invalid key size"},
		{name: "invalid B base64", req: XeapiEncryptedRequest{B: "%%%", R: validR}, key: dynamicKey, wantErr: "base64.DecodeString B"},
		{name: "short mid payload", req: XeapiEncryptedRequest{B: shortMid, R: validR}, key: dynamicKey, wantErr: "mid payload too short"},
		{name: "invalid mid base64", req: XeapiEncryptedRequest{B: invalidMidBase64, R: validR}, key: dynamicKey, wantErr: "reversed mid payload"},
		{name: "invalid inner padding", req: XeapiEncryptedRequest{B: invalidInner, R: validR}, key: dynamicKey, wantErr: "decrypt B inner layer"},
		{name: "invalid B without key", req: XeapiEncryptedRequest{B: "AA==", R: validR}, wantErr: "validate B"},
		{name: "invalid S base64", req: XeapiEncryptedRequest{S: "%%%", R: validR}, wantErr: "base64.DecodeString S"},
		{name: "short S frame", req: XeapiEncryptedRequest{S: base64.StdEncoding.EncodeToString(make([]byte, 60)), R: validR}, wantErr: "frame is too short"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := XeapiDecryptRequest(tt.req, tt.key)
			require.ErrorContains(t, err, tt.wantErr)

			if tt.req.R == validR {
				assert.Equal(t, "v1", got.PublicKeyVersion)
				assert.Equal(t, "session-id", got.SessionID)
			}
		})
	}
}

func TestXeapiDecryptRequestRetainsBWhenSIsInvalid(t *testing.T) {
	dynamicKey, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	require.NoError(t, err)

	result, err := XeapiDecryptRequest(XeapiEncryptedRequest{
		B: "J5+3SnVyE16Pm4720e7gA3mgIZ1L4axkB6jte8X079wgjs3SU+IK7AANKKdewVLtBIJw5y5LtyhCcJ3FZm4u2LOfXnKdOC0VKIfVgX/lWloAZX6hQGVaRHgnR3BdQi+t",
		S: "%%%",
		R: "3LCoCTuHo/mDfZ1x3PtHsQ==",
	}, dynamicKey)
	require.ErrorContains(t, err, "validate S")
	assert.JSONEq(t, `{"body":"","queryString":"e_r=true"}`, string(result.Plaintext))
	assert.False(t, result.SFrameValid)
}

func TestXeapiDecryptRequestKeepsMissingSAsCallerPolicy(t *testing.T) {
	dynamicKey, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	require.NoError(t, err)

	result, err := XeapiDecryptRequest(XeapiEncryptedRequest{
		B: "J5+3SnVyE16Pm4720e7gA3mgIZ1L4axkB6jte8X079wgjs3SU+IK7AANKKdewVLtBIJw5y5LtyhCcJ3FZm4u2LOfXnKdOC0VKIfVgX/lWloAZX6hQGVaRHgnR3BdQi+t",
		R: "3LCoCTuHo/mDfZ1x3PtHsQ==",
	}, dynamicKey)
	require.NoError(t, err)
	assert.JSONEq(t, `{"body":"","queryString":"e_r=true"}`, string(result.Plaintext))
	assert.False(t, result.SFrameValid)
}

func TestXeapiDecrypt(t *testing.T) {
	t.Run("plain json", func(t *testing.T) {
		ciphertext, err := aesECBEncrypt([]byte(eApiKey), []byte(`{"code":200}`))
		require.NoError(t, err)

		got, err := XeapiDecrypt(ciphertext)
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

		got, err := XeapiDecrypt(ciphertext)
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

func encryptXeapiTestValue(t *testing.T, key, plaintext []byte) string {
	t.Helper()

	ciphertext, err := aesECBEncrypt(key, plaintext)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func encryptXeapiTestBlocks(t *testing.T, key, plaintext []byte) string {
	t.Helper()

	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	require.Zero(t, len(plaintext)%block.BlockSize())
	return base64.StdEncoding.EncodeToString(AesEncryptECB(block, plaintext))
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
