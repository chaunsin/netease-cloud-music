// Copyright (c) 2024-2026 chaunsin
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRandomKey(t *testing.T) {
	text, err := randomKey()
	require.NoError(t, err)
	assert.NotEmpty(t, text)
	t.Logf("GenerateRandomKey: %s\n", text) // ONRhfKsUOHoF8iVd
}

func TestReverseString(t *testing.T) {
	type args struct {
		str string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "正常",
			args: args{
				str: "123456789",
			},
			want: "987654321",
		},
		{
			name: "回文",
			args: args{
				str: "123321",
			},
			want: "123321",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, reverseString(tt.args.str), "ReverseString(%v)", tt.args.str)
		})
	}
}

func TestRsaEncryptReturnsFixedWidthHex(t *testing.T) {
	got, err := RsaEncrypt("0123456789abcdef", publicKey)
	require.NoError(t, err)
	assert.Len(t, got, 256)
}

func TestWeApiEncrypt(t *testing.T) {
	type args struct {
		object any
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "正常",
			args: args{
				object: map[string]string{"phone": "188********", "ctcode": "86"},
			},
			want:    map[string]string{},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WeApiEncrypt(tt.args.object)
			require.NoError(t, err)
			t.Logf("data:%+v\n", got)
			// assert.Equalf(t, tt.want, got, "weapi(%v)", tt.args.object)
		})
	}
}

func TestEApiDecrypt(t *testing.T) {
	type args struct {
		encode string
		data   string
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test request",
			args: args{
				encode: "hex",
				data:   "1BDAA66BB859333CCCE0A53AE6D1E6E61F5C1663DE05CFFB8C87BCE2FDC6F9ECAB1F5341B2FBCB5CBBACDA665D6F1A10B007189F44A13DB2463BB3EBF2639CF10A3E14D47E97975942FF626F17CE4A658E17F19C52EDACCB199F262EA09723E644C46E3880B4754AE1A2A1F4712268C52AEA6F5D0158780D82BDC30C930756181972480BE18A2ECD68A276C68E5214491F2323B3C87ECA2AF9532A4F483D55B8C5187D558AF5699D2C2437C1D98CB5AD7B90402CCDB12DF950521A86D854646BF8422708A649C1B8B752AF70AD5B3868F939FD0E9BEAA8BAE0D05BB0D4D88BE1A6BFAA8F5BBECD6F92368480E657D2200F8ACE7740ACAAA5634297D6661704EE7F74779E833DF2241939FC60C5D92569E31285E4F4A4F737CC8E89316DE7BBC8FB99E94B87DC05C190EA228637B2C0D182152BFAC603EF671A9A0B2F907D98F30E8A4614F236B3ED78392F039EDAD3C3CE5A856EE51BCDE2173F428CD1BB0239",
			},
			want: "/api/music/partner/work/evaluate-36cd479b6b5-{\"taskId\":\"185640294\",\"workId\":\"1312207\",\"score\":\"3\",\"tags\":\"3-C-1\",\"customTags\":\"[]\",\"comment\":\"\",\"extraResource\":\"true\",\"syncYunCircle\":\"false\",\"syncComment\":\"true\",\"extraScore\":\"{\\\"1\\\":3,\\\"2\\\":2,\\\"3\\\":4}\",\"source\":\"mp-music-partner\",\"header\":\"{}\",\"e_r\":true}-36cd479b6b5-f891fb9aa53a9b84280a53c43ff84de8",
			wantErr: func(t assert.TestingT, err error, msg ...any) bool {
				if err != nil {
					t.Errorf("%s", msg)
					return false
				}
				return true
			},
		},
		{
			name: "test response",
			args: args{
				encode: "hex",
				data:   "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08",
			},
			want: `{"code":200,"data":true}`,
			wantErr: func(t assert.TestingT, err error, msg ...any) bool {
				if err != nil {
					t.Errorf("%s", msg)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EApiDecrypt(tt.args.data, "hex")
			if !tt.wantErr(t, err, fmt.Sprintf("EApiDecrypt(%v)", tt.args.data)) {
				return
			}

			t.Logf("data: %+v\n", string(got))
			t.Logf("abcd: %+v\n", tt.want)
			assert.Equalf(t, tt.want, string(got), "EApiDecrypt(%v)", tt.args.data)
		})
	}
}

func TestEApiEncrypt(t *testing.T) {
	type args struct {
		url    string
		object any
	}

	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: args{"/test/url", "test value"},
			want: map[string]string{
				"params": "E556EA4892989E4A1B98043B56CD3C77C6DBE3D0261A0FA8ACF45E2882DBABFD13F52E05D9EF39C101A7A46DD0E0CD0979A2DD9CE30975861F6F4E86855FE00AD841C36BA90177218D0D8D32A54A0DC4",
			},
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
		// {
		// 	name: "无url参数情况",
		// 	args: args{"", `{"code":200,"data":true}`},
		// 	want: map[string]string{"params": "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08"},
		// 	wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
		// 		if err != nil {
		// 			t.Errorf("err: %v args: %s", err, i)
		// 			return false
		// 		}
		// 		return true
		// 	},
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EApiEncrypt(tt.args.url, tt.args.object)
			if !tt.wantErr(t, err, fmt.Sprintf("EApiEncrypt(%v, %v)", tt.args.url, tt.args.object)) {
				return
			}

			assert.Equalf(t, tt.want, got, "EApiEncrypt(%v, %v)", tt.args.url, tt.args.object)
		})
	}
}

func TestHex(t *testing.T) {
	var (
		apiPath = "/api/sms/captcha/sent"
		data    = `{"deviceId":"4cdb39bf34a848781b89663e1e546b8b","os":"OSX","cellphone":"188********","header":"{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\"IuRPVVmc3WWul9fT\\\":{\\\"version\\\":143360,\\\"appver\\\":\\\"2.3.17\\\"}}\",\"MG-Product-Name\":\"music\"}","ctcode":"86","verifyId":1,"e_r":true}`
		target  = `/api/sms/captcha/sent-36cd479b6b5-{"deviceId":"4cdb39bf34a848781b89663e1e546b8b","os":"OSX","cellphone":"188********","header":"{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\"IuRPVVmc3WWul9fT\\\":{\\\"version\\\":143360,\\\"appver\\\":\\\"2.3.17\\\"}}\",\"MG-Product-Name\":\"music\"}","ctcode":"86","verifyId":1,"e_r":true}-36cd479b6b5-6712bc8cd675b8e4059289b0b56abcbe`
	)

	message := fmt.Sprintf("nobody%suse%smd5forencrypt", apiPath, data)
	digest := fmt.Sprintf("%x", legacyMD5([]byte(message)))
	text := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%s", apiPath, data, digest)
	t.Logf("text: %s", text)
	t.Logf("equal: %v", text == target)
}

// func Test_digest(t *testing.T) {
// 	var (
// 		sendSMSReq = api.SendSMSReq{
// 			CtCode:    "86",
// 			Cellphone: "188********",
// 			DeviceId:  "4cdb39bf34a848781b89663e1e546b8b",
// 			Os:        "OSX",
// 			VerifyId:  1,
// 			ER:        true,
// 		}
// 		header = api.SendSMSReqHeader{
// 			Os:            "osx",
// 			AppVer:        "2.3.17",
// 			DeviceId:      "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B",
// 			RequestId:     "93487028",
// 			ClientSign:    "",
// 			OsVer:         "%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89",
// 			NmGCoreStatus: "1",
// 			MConfigInfo:   `{"IuRPVVmc3WWul9fT":{"version":143360,"appver":"2.3.17"}}`,
// 			MGProductName: "music",
// 		}
// 		url = "/api/sms/captcha/sent"
// 	)
// 	headerByte, err := json.Marshal(header)
// 	if err != nil {
// 		t.Fatalf("json: %s", err)
// 	}
// 	sendSMSReq.Header = string(headerByte)
// 	payload, err := json.Marshal(sendSMSReq)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("json: %s", string(payload))
// 	t.Logf("digest: %s\n", digest(url, string(payload)))
// }

func TestGetCacheKey(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: "args123",
			want: "RFKLrid1HPwKv4hPWldxJA==",
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CacheKeyEncrypt(tt.args)
			if !tt.wantErr(t, err, fmt.Sprintf("CacheKeyEncrypt(%v)", tt.args)) {
				return
			}

			t.Logf("data: %+v\n", got)
			assert.Equalf(t, tt.want, got, "CacheKeyEncrypt(%v)", tt.args)
		})
	}
}

func TestCacheKeyDecrypt(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: "RFKLrid1HPwKv4hPWldxJA==",
			want: "args123",
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return true
				}
				return false
			},
		},
		{
			name: "真实参数",
			args: "0cjs/PeKn8i8GZDV84eJ5IqRq/RX1Hok5Oyt1+2iwcgHfZVdOn+GbulSnnhB4gmf",
			want: "e_r=false&id=10171989900&n=3&s=0",
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CacheKeyDecrypt(tt.args)
			if !tt.wantErr(t, err, fmt.Sprintf("CacheKeyDecrypt(%v)", tt.args)) {
				return
			}

			t.Logf("got: %+v\n", got)
			assert.Equalf(t, tt.want, got, "CacheKeyDecrypt(%v)", tt.args)
		})
	}
}

func TestDLLEncodeID(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: `7F3DB13E-3B90-5859-ACCC-E5BD694285A8%7CB5961B82-C81E-40E9-9164-5BE49896353A`,
			want: "7ciIN1SujXn_nJUbCEIOBA==",
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DLLEncodeID(tt.args)
			if !tt.wantErr(t, err, fmt.Sprintf("DLLEncodeID(%v)", tt.args)) {
				return
			}

			assert.Equalf(t, tt.want, got, "DLLEncodeID(%v)", tt.args)
		})
	}
}

func TestAnonymous(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: "7F3DB13E-3B90-5859-ACCC-E5BD694285A8%7CB5961B82-C81E-40E9-9164-5BE49896353A",
			want: "N0YzREIxM0UtM0I5MC01ODU5LUFDQ0MtRTVCRDY5NDI4NUE4JTdDQjU5NjFCODItQzgxRS00MEU5LTkxNjQtNUJFNDk4OTYzNTNBIDdjaUlOMVN1alhuX25KVWJDRUlPQkE9PQ==",
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Anonymous(tt.args)
			if !tt.wantErr(t, err, fmt.Sprintf("Anonymous(%v)", tt.args)) {
				return
			}

			assert.Equalf(t, tt.want, got, "Anonymous(%v)", tt.args)
		})
	}
}

func TestHexDigest(t *testing.T) {
	tests := []struct {
		input  []byte
		expect string
	}{
		{[]byte{0x00}, "93b885adfe0da089cdf634904fd59f71"},
		{[]byte{0xAB, 0xCD}, "7838496fd0586421bbb500bb6f472f13"},
		{[]byte{0x0A, 0x1F}, "22094d34279aa1ba36fa7094cfc90eeb"},
		{[]byte{}, "d41d8cd98f00b204e9800998ecf8427e"},
	}

	for _, tt := range tests {
		got := HexDigest(string(tt.input))
		if got != tt.expect {
			t.Errorf("HexDigest(%x) = %s, want %s", tt.input, got, tt.expect)
		}
	}
}

func TestBuildPlaintextEnvelope(t *testing.T) {
	t.Run("default request omits method and content type", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
			URI: "/api/song/detail?id=101",
		})

		assert.Equal(t, "id=101&e_r=true", envelope["queryString"])
		assert.NotContains(t, envelope, "body")
		assert.NotContains(t, envelope, "url")
		assert.NotContains(t, envelope, "method")
		assert.NotContains(t, envelope, "contentType")
	})

	t.Run("non default request keeps method and content type", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
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
		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
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
		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
			URI: "/api/test?e_r=false&id=1",
		})

		assert.Equal(t, "e_r=false&id=1&e_r=true", envelope["queryString"])
	})

	t.Run("raw body preserves exact bytes", func(t *testing.T) {
		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
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
		plaintext, err := buildPlaintextEnvelope(&EncryptRequest{
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

		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
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
		envelope := decodeXeapiEnvelope(t, &EncryptRequest{
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

	want := PublicKeyState{
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

func TestIssue174CapturedXeapiVectors(t *testing.T) {
	staticKeyFromIssue, err := base64.StdEncoding.DecodeString("qx1aQw9rsEo/Aegd3XK9kW1c5ZEkisEocUgG1/j7G4Q=")
	require.NoError(t, err)
	assert.Equal(t, xeapiStaticKey, staticKeyFromIssue)

	capturedR, err := base64.StdEncoding.DecodeString("6uMm/2V2SqT96D2FtoKGgFHzKX+TP+dChrWGTsVtcjBpuNxqLTfwHTEO8RThwA7e")
	require.NoError(t, err)
	capturedRPlaintext, err := aesECBDecrypt(xeapiStaticKey, capturedR)
	require.NoError(t, err)
	assert.Equal(t, "1000000000000|01c3a3532a884dd2a583228d6f335211", string(capturedRPlaintext))

	noSessionR, err := base64.StdEncoding.DecodeString("3LCoCTuHo/mDfZ1x3PtHsQ==")
	require.NoError(t, err)
	noSessionRPlaintext, err := aesECBDecrypt(xeapiStaticKey, noSessionR)
	require.NoError(t, err)
	assert.Equal(t, "1000000000000|", string(noSessionRPlaintext))

	responseBody, err := hex.DecodeString("BCC6C3A838364F78C6613EF403862326D0CB333FB97328516FB0C72CD7DB1B8E6AA3B102FBE7296AB0DB9EA5C46AD12B")
	require.NoError(t, err)
	plaintext, err := XeapiDecryptResponse(responseBody)
	require.NoError(t, err)
	assert.Equal(t, `{"code":200}`, string(plaintext))
}

func TestXeapiEncryptIssue174GoldenBody(t *testing.T) {
	dynamicKey, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	require.NoError(t, err)

	transformRandom := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	gcmIV := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	privateKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	stubXeapiRandomness(t, [][]byte{transformRandom, dynamicKey, gcmIV}, privateKey)

	req := EncryptRequest{
		URI:  "/api/song/enhance/location/info",
		Body: []byte{},
		OS:   "android",
	}
	plaintext, err := buildPlaintextEnvelope(&req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"body":"","queryString":"e_r=true"}`, string(plaintext))

	got, err := XeapiEncrypt(&req, PublicKeyState{
		PublicKey:      "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:        "1000000000000",
		NextUpdateTime: 1803882269000,
		SK:             "8PZfbIFA1779944463972",
	}, Session{})
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

	publicKey := PublicKeyState{
		PublicKey: base64.StdEncoding.EncodeToString(peer.PublicKey().Bytes()),
		Version:   "v1",
		SK:        "server-key",
	}
	_, err = XeapiEncrypt(nil, publicKey, Session{})
	require.ErrorIs(t, err, ErrEncryptRequestMissing)

	req := EncryptRequest{
		URI:  "/api/song/detail?id=1",
		Data: map[string]any{"id": 1, "e_r": true},
	}

	withSession, err := XeapiEncrypt(&req, publicKey, Session{ID: "session-id", Key: "0123456789abcdef"})
	require.NoError(t, err)
	withoutSession, err := XeapiEncrypt(&req, publicKey, Session{})
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

	_, err = XeapiEncrypt(&req, PublicKeyState{
		PublicKey: publicKey.PublicKey,
		Version:   "v1",
	}, Session{})
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

func decodeXeapiEnvelope(t *testing.T, req *EncryptRequest) map[string]string {
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
