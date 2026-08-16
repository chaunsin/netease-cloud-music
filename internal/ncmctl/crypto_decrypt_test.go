// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	har "github.com/chaunsin/go-har"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
	projectlog "github.com/chaunsin/netease-cloud-music/pkg/log"
)

const (
	xeapiGoldenB = "J5+3SnVyE16Pm4720e7gA3mgIZ1L4axkB6jte8X079wgjs3SU+IK7AANKKdewVLtBIJw5y5LtyhCcJ3FZm4u2LOfXnKdOC0VKIfVgX/lWloAZX6hQGVaRHgnR3BdQi+t"
	xeapiGoldenS = "B6N8vBQgk8i3VdwbEOhstCY3StFqqFPtC9/AsrhtHHwAAQIDBAUGBwgJCguNFV1OAc3Z5noM7bYwvLwNFBK0H8NY/JVdIRN2dRDdG1JrMTLDI/ArlqMSIXdq9rfulgMKqRO7imtYLn8PrI4cIbwOdSkz"
	xeapiGoldenR = "3LCoCTuHo/mDfZ1x3PtHsQ=="

	xeapiGoldenDynamicKeyHex = "00112233445566778899aabbccddeeff"
	xeapiGoldenResponseHex   = "BCC6C3A838364F78C6613EF403862326D0CB333FB97328516FB0C72CD7DB1B8E6AA3B102FBE7296AB0DB9EA5C46AD12B"
	xeapiGoldenEnvelope      = `{"body":"","queryString":"e_r=true"}`
)

func TestPayloadMarshalOmitsZeroResponse(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Payload{
		Request: Request{Ciphertext: "request-ciphertext"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"request":{"ciphertext":"request-ciphertext"}}`, string(data))
}

func TestPayloadMarshalOmitsZeroRequest(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Payload{
		Response: Response{Ciphertext: "response-ciphertext"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"response":{"ciphertext":"response-ciphertext"}}`, string(data))
}

func TestPayloadMarshalIncludesRequestAndResponse(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Payload{
		Request:  Request{Ciphertext: "request-ciphertext"},
		Response: Response{Ciphertext: "response-ciphertext"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"request": {"ciphertext": "request-ciphertext"},
		"response": {"ciphertext": "response-ciphertext"}
	}`, string(data))
}

func TestParseEAPIEnvelopeUsesOuterDelimiters(t *testing.T) {
	payload := `{"value":"before` + eapiEnvelopeDelimiter + `after"}`
	raw := "/api/test" + eapiEnvelopeDelimiter + payload + eapiEnvelopeDelimiter + "digest"

	plaintext, requestURL, digest := parseEAPIEnvelope(raw)
	assert.Equal(t, payload, plaintext)
	assert.Equal(t, "/api/test", requestURL)
	assert.Equal(t, "digest", digest)

	rawJSON := `{"value":"` + eapiEnvelopeDelimiter + eapiEnvelopeDelimiter + `"}`
	plaintext, requestURL, digest = parseEAPIEnvelope(rawJSON)
	assert.JSONEq(t, rawJSON, plaintext)
	assert.Empty(t, requestURL)
	assert.Empty(t, digest)
}

func TestCryptoDecryptXeapiDirectResponse(t *testing.T) {
	rawCiphertext, err := hex.DecodeString(xeapiGoldenResponseHex)
	require.NoError(t, err)

	ciphertextFile := filepath.Join(t.TempDir(), "response.txt")
	require.NoError(t, os.WriteFile(ciphertextFile, []byte(xeapiGoldenResponseHex+"\n"), 0o600))

	tests := []struct {
		name           string
		encoding       string
		input          string
		outputEncoding string
	}{
		{name: "hex defaults to response", encoding: "hex", input: xeapiGoldenResponseHex, outputEncoding: "hex"},
		{name: "base64", encoding: "base64", input: base64.StdEncoding.EncodeToString(rawCiphertext), outputEncoding: "base64"},
		{name: "raw string", encoding: "string", input: string(rawCiphertext), outputEncoding: "base64"},
		{name: "ciphertext file with trailing newline", encoding: "hex", input: ciphertextFile, outputEncoding: "hex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeCryptoCommand(t,
				"--kind", "xeapi",
				"decrypt", "--encode", tt.encoding, tt.input,
			)
			require.NoError(t, err)

			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(output, &fields))
			assert.NotContains(t, fields, "request")

			var result Payload
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, "xeapi", result.Kind)
			assert.Equal(t, "ok", result.Status)
			assert.Equal(t, tt.outputEncoding, result.Response.CiphertextEncoding)
			storedCiphertext, decodeErr := decodeCryptoBytes(result.Response.Ciphertext, result.Response.CiphertextEncoding)
			require.NoError(t, decodeErr)
			assert.Equal(t, rawCiphertext, storedCiphertext)
			assert.JSONEq(t, `{"code":200}`, string(result.Response.Plaintext))
		})
	}
}

func TestCryptoDecryptXeapiDirectRequestKeyEncodings(t *testing.T) {
	goldenKey, err := hex.DecodeString(xeapiGoldenDynamicKeyHex)
	require.NoError(t, err)

	goldenParams := xeapiGoldenParams()
	goldenForm := encodeXeapiParams(goldenParams)
	sessionForm, sessionParams := xeapiSessionFixture(t)

	tests := []struct {
		name           string
		keyEncoding    string
		key            string
		form           string
		params         map[string]string
		wantEnvelope   string
		wantSessionID  string
		useDefaultMode bool
	}{
		{
			name:          "hex golden vector",
			keyEncoding:   "hex",
			key:           xeapiGoldenDynamicKeyHex,
			form:          goldenForm,
			params:        goldenParams,
			wantEnvelope:  xeapiGoldenEnvelope,
			wantSessionID: "",
		},
		{
			name:          "base64 golden vector",
			keyEncoding:   "base64",
			key:           base64.StdEncoding.EncodeToString(goldenKey),
			form:          goldenForm,
			params:        goldenParams,
			wantEnvelope:  xeapiGoldenEnvelope,
			wantSessionID: "",
		},
		{
			name:           "raw session key uses string by default",
			key:            "0123456789abcdef",
			form:           sessionForm,
			params:         sessionParams,
			wantEnvelope:   `{"body":"","queryString":"id=1\u0026e_r=true"}`,
			wantSessionID:  "session-id",
			useDefaultMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{
				"--kind", "xeapi",
				"decrypt", "--target", "request", "--dynamic-key", tt.key,
			}
			if !tt.useDefaultMode {
				args = append(args, "--dynamic-key-encode", tt.keyEncoding)
			}

			args = append(args, tt.form)

			output, err := executeCryptoCommand(t, args...)
			require.NoError(t, err)

			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(output, &fields))
			assert.NotContains(t, fields, "response")

			var result Payload
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, "ok", result.Status)
			assert.Equal(t, tt.params, result.Request.Params)
			assert.Equal(t, "1000000000000", result.Request.KeyVersion)
			require.NotNil(t, result.Request.SessionID)
			assert.Equal(t, tt.wantSessionID, *result.Request.SessionID)
			assertExactEnvelope(t, tt.wantEnvelope, result.Request.RawPlaintext)
			assert.JSONEq(t, tt.wantEnvelope, string(result.Request.Plaintext))
		})
	}
}

func TestCryptoDecryptXeapiDirectRequestWritesPartialResult(t *testing.T) {
	form := encodeXeapiParams(xeapiGoldenParams())

	t.Run("missing key keeps R metadata", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "request", form,
		)
		require.ErrorContains(t, err, "only partially decrypted")

		var result Payload
		require.NoError(t, json.Unmarshal(output, &result))
		assert.Equal(t, "partial", result.Status)
		assert.Equal(t, "1000000000000", result.Request.KeyVersion)
		require.NotNil(t, result.Request.SessionID)
		assert.Empty(t, *result.Request.SessionID)
		assert.Contains(t, result.Request.Error, "dynamic key is required")
		assert.Empty(t, result.Request.Plaintext)
	})

	t.Run("wrong key keeps R metadata", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "request",
			"--dynamic-key-encode", "hex",
			"--dynamic-key", strings.Repeat("ff", 16),
			form,
		)
		require.ErrorContains(t, err, "only partially decrypted")

		var result Payload
		require.NoError(t, json.Unmarshal(output, &result))
		assert.Equal(t, "partial", result.Status)
		assert.Equal(t, "1000000000000", result.Request.KeyVersion)
		assert.Contains(t, result.Request.Error, "decrypt B")
	})

	t.Run("missing S keeps R metadata and decrypted B", func(t *testing.T) {
		params := xeapiGoldenParams()
		delete(params, "S")

		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "request",
			"--dynamic-key-encode", "hex",
			"--dynamic-key", xeapiGoldenDynamicKeyHex,
			encodeXeapiParams(params),
		)
		require.ErrorContains(t, err, "only partially decrypted")

		var result Payload
		require.NoError(t, json.Unmarshal(output, &result))
		assert.Equal(t, "partial", result.Status)
		assert.Equal(t, "1000000000000", result.Request.KeyVersion)
		assert.Contains(t, result.Request.Error, "parameter S is missing")
		assertExactEnvelope(t, xeapiGoldenEnvelope, result.Request.RawPlaintext)
		assert.JSONEq(t, xeapiGoldenEnvelope, string(result.Request.Plaintext))
	})

	t.Run("R-only form keeps metadata", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "request",
			url.Values{"R": {xeapiGoldenR}}.Encode(),
		)
		require.ErrorContains(t, err, "only partially decrypted")

		var result Payload
		require.NoError(t, json.Unmarshal(output, &result))
		assert.Equal(t, "partial", result.Status)
		assert.Equal(t, "1000000000000", result.Request.KeyVersion)
		require.NotNil(t, result.Request.SessionID)
		assert.Empty(t, *result.Request.SessionID)
		assert.Contains(t, result.Request.Error, "parameter B is missing")
		assert.Contains(t, result.Request.Error, "parameter S is missing")
	})
}

func TestCryptoDecryptXeapiDirectRequestRejectsMalformedS(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "invalid base64", value: "%%%", wantErr: "base64.DecodeString S"},
		{
			name:    "truncated frame",
			value:   base64.StdEncoding.EncodeToString(make([]byte, 32+12+16)),
			wantErr: "frame is too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := xeapiGoldenParams()
			params["S"] = tt.value

			output, err := executeCryptoCommand(t,
				"--kind", "xeapi",
				"decrypt", "--target", "request",
				"--dynamic-key-encode", "hex",
				"--dynamic-key", xeapiGoldenDynamicKeyHex,
				encodeXeapiParams(params),
			)
			require.ErrorContains(t, err, "only partially decrypted")

			var result Payload
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, "partial", result.Status)
			assert.Contains(t, result.Request.Error, tt.wantErr)
			assert.Equal(t, "1000000000000", result.Request.KeyVersion)
			assert.JSONEq(t, xeapiGoldenEnvelope, string(result.Request.Plaintext))
		})
	}
}

func TestCryptoDecryptXeapiRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown target",
			args:    []string{"--kind", "xeapi", "decrypt", "--target", "sideways", xeapiGoldenResponseHex},
			wantErr: `unknown decrypt target "sideways"`,
		},
		{
			name:    "both with direct input",
			args:    []string{"--kind", "xeapi", "decrypt", "--target", "both", xeapiGoldenResponseHex},
			wantErr: "target both is only supported for HAR input",
		},
		{
			name: "invalid dynamic key length",
			args: []string{
				"--kind", "xeapi", "decrypt", "--target", "request",
				"--dynamic-key-encode", "hex", "--dynamic-key", "0011",
				encodeXeapiParams(xeapiGoldenParams()),
			},
			wantErr: "dynamic key must decode to 16, 24, or 32 bytes; got 2",
		},
		{
			name: "invalid dynamic key encoding",
			args: []string{
				"--kind", "xeapi", "decrypt", "--target", "request",
				"--dynamic-key-encode", "rot13", "--dynamic-key", "value",
				encodeXeapiParams(xeapiGoldenParams()),
			},
			wantErr: `dynamic-key-encode: unknown encoding "rot13"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeCryptoCommand(t, tt.args...)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Empty(t, output)
		})
	}
}

func TestCryptoDecryptXeapiHAR(t *testing.T) {
	rawResponse, err := hex.DecodeString(xeapiGoldenResponseHex)
	require.NoError(t, err)
	harFile := writeXeapiHAR(t, xeapiGoldenParams(), rawResponse)

	t.Run("both decrypts params and form text", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt",
			"--dynamic-key-encode", "hex",
			"--dynamic-key", xeapiGoldenDynamicKeyHex,
			harFile,
		)
		require.NoError(t, err)

		var results []Payload
		require.NoError(t, json.Unmarshal(output, &results))
		require.Len(t, results, 2)

		for _, result := range results {
			assert.Equal(t, "xeapi", result.Kind)
			assert.Equal(t, "ok", result.Status)
			assert.Equal(t, xeapiGoldenParams(), result.Request.Params)
			assertExactEnvelope(t, xeapiGoldenEnvelope, result.Request.RawPlaintext)
			assert.JSONEq(t, xeapiGoldenEnvelope, string(result.Request.Plaintext))
			assert.Equal(t, "base64", result.Response.CiphertextEncoding)
			storedResponse, decodeErr := decodeCryptoBytes(
				result.Response.Ciphertext,
				result.Response.CiphertextEncoding,
			)
			require.NoError(t, decodeErr)
			assert.Equal(t, rawResponse, storedResponse)
			assert.JSONEq(t, `{"code":200}`, string(result.Response.Plaintext))
		}
	})

	t.Run("missing key writes all partial results before returning", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", harFile,
		)
		require.ErrorContains(t, err, "2 XEAPI request(s) were only partially decrypted")

		var results []Payload
		require.NoError(t, json.Unmarshal(output, &results))
		require.Len(t, results, 2)

		for _, result := range results {
			assert.Equal(t, "partial", result.Status)
			assert.Equal(t, "1000000000000", result.Request.KeyVersion)
			require.NotNil(t, result.Request.SessionID)
			assert.Empty(t, *result.Request.SessionID)
			assert.Contains(t, result.Request.Error, "dynamic key is required")
			assert.Empty(t, result.Request.Plaintext)
			assert.JSONEq(t, `{"code":200}`, string(result.Response.Plaintext))
		}
	})
}

func TestCryptoDecryptXeapiHARTargetIsolation(t *testing.T) {
	rawResponse, err := hex.DecodeString(xeapiGoldenResponseHex)
	require.NoError(t, err)

	malformedForm := url.Values{
		"R": {xeapiGoldenR},
		"S": {xeapiGoldenS},
	}.Encode() + "&B=%ZZ"
	malformedRequestFile := writeHAR(t, []*har.Entry{
		newXeapiHAREntry("/xeapi/malformed", &har.PostData{
			MimeType: "Application/X-WWW-Form-Urlencoded; charset=utf-8",
			Text:     malformedForm,
		}, rawResponse),
	})

	t.Run("response target ignores malformed request form", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "response", malformedRequestFile,
		)
		require.NoError(t, err)

		var results []Payload
		require.NoError(t, json.Unmarshal(output, &results))
		require.Len(t, results, 1)
		assert.Empty(t, results[0].Request)
		assert.JSONEq(t, `{"code":200}`, string(results[0].Response.Plaintext))
	})

	t.Run("both records malformed request and decrypts response", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", malformedRequestFile,
		)
		require.ErrorContains(t, err, "1 XEAPI request(s) were only partially decrypted")

		var results []Payload
		require.NoError(t, json.Unmarshal(output, &results))
		require.Len(t, results, 1)
		assert.Equal(t, "partial", results[0].Status)
		assert.Equal(t, "1000000000000", results[0].Request.KeyVersion)
		assert.Contains(t, results[0].Request.Error, "invalid URL escape")
		assert.Contains(t, results[0].Request.Error, "parameter B is missing")
		assert.JSONEq(t, `{"code":200}`, string(results[0].Response.Plaintext))
	})

	missingResponse := newXeapiHAREntry("/xeapi/request-only", &har.PostData{
		MimeType: "application/x-www-form-urlencoded",
		Params:   xeapiPostParams(xeapiGoldenParams()),
	}, rawResponse)
	missingResponse.Response = nil
	missingResponseFile := writeHAR(t, []*har.Entry{missingResponse})

	t.Run("request target accepts missing response without polluting JSON", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "request",
			"--dynamic-key-encode", "hex",
			"--dynamic-key", xeapiGoldenDynamicKeyHex,
			missingResponseFile,
		)
		require.NoError(t, err)

		var results []Payload
		require.NoError(t, json.Unmarshal(output, &results))
		require.Len(t, results, 1)
		assert.Equal(t, "ok", results[0].Status)
		assert.Empty(t, results[0].Response)
		assert.JSONEq(t, xeapiGoldenEnvelope, string(results[0].Request.Plaintext))
	})

	t.Run("response target rejects missing response as structure error", func(t *testing.T) {
		output, err := executeCryptoCommand(t,
			"--kind", "xeapi",
			"decrypt", "--target", "response", missingResponseFile,
		)
		require.ErrorContains(t, err, "entry 0 response is missing")
		assert.Empty(t, output)
	})
}

func TestCryptoDecryptPropagatesHARReaderError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.har")
	require.NoError(t, os.WriteFile(path, []byte(`{`), 0o600))

	output, err := executeCryptoCommand(t,
		"--kind", "xeapi",
		"decrypt", "--target", "request", path,
	)
	require.ErrorContains(t, err, "har.NewReader")
	assert.Empty(t, output)
}

func TestCryptoDecryptRejectsHARWithoutRequest(t *testing.T) {
	harFile := writeHAR(t, []*har.Entry{{
		Response: &har.Response{Content: &har.Content{}},
	}})

	output, err := executeCryptoCommand(t,
		"--kind", "xeapi",
		"decrypt", "--target", "response", harFile,
	)
	require.ErrorContains(t, err, "entry 0 request is missing")
	assert.Empty(t, output)
}

func TestCryptoEncryptRejectsXeapiBeforeParsingInput(t *testing.T) {
	output, err := executeCryptoCommand(t,
		"--kind", "xeapi",
		"encrypt", "not-json",
	)
	require.ErrorContains(t, err, "XEAPI encryption is not implemented by ncmctl crypto encrypt")
	assert.NotContains(t, err.Error(), "unmarshal")
	assert.Empty(t, output)
}

func TestCryptoDecryptEapiStringEncoding(t *testing.T) {
	encrypted, err := ncmcrypto.EApiEncrypt("/eapi/test", map[string]int{"id": 1})
	require.NoError(t, err)
	rawCiphertext, err := hex.DecodeString(encrypted["params"])
	require.NoError(t, err)

	output, err := executeCryptoCommand(t,
		"--kind", "eapi",
		"decrypt", "--encode", "string", string(rawCiphertext),
	)
	require.NoError(t, err)

	var result Payload
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, "base64", result.Request.CiphertextEncoding)
	storedCiphertext, err := decodeCryptoBytes(result.Request.Ciphertext, result.Request.CiphertextEncoding)
	require.NoError(t, err)
	assert.Equal(t, rawCiphertext, storedCiphertext)
	assert.Equal(t, "/api/test", result.Request.Url)
	assert.JSONEq(t, `{"id":1}`, string(result.Request.Plaintext))
}

func TestCryptoDecryptEapiExplicitResponseTarget(t *testing.T) {
	const responseCiphertext = "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08"

	output, err := executeCryptoCommand(t,
		"--kind", "eapi",
		"decrypt", "--target", "response", responseCiphertext,
	)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output, &fields))
	assert.NotContains(t, fields, "request")

	var result Payload
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, "hex", result.Response.CiphertextEncoding)
	assert.JSONEq(t, `{"code":200,"data":true}`, string(result.Response.Plaintext))
}

func TestCryptoDecryptEapiHARUsesRequestResponseMode(t *testing.T) {
	const encryptedResponseHex = "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08"

	encryptedResponse, err := hex.DecodeString(encryptedResponseHex)
	require.NoError(t, err)

	tests := []struct {
		name         string
		request      map[string]any
		response     []byte
		wantResponse string
	}{
		{
			name:         "explicit plaintext response",
			request:      map[string]any{"e_r": false, "id": 1},
			response:     []byte(`{"code":200,"mode":"plaintext"}`),
			wantResponse: `{"code":200,"mode":"plaintext"}`,
		},
		{
			name:         "explicit encrypted response",
			request:      map[string]any{"e_r": true, "id": 1},
			response:     encryptedResponse,
			wantResponse: `{"code":200,"data":true}`,
		},
		{
			name:         "missing response mode defaults to encrypted",
			request:      map[string]any{"id": 1},
			response:     encryptedResponse,
			wantResponse: `{"code":200,"data":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encryptedRequest, encryptErr := ncmcrypto.EApiEncrypt("/eapi/test", tt.request)
			require.NoError(t, encryptErr)

			harFile := writeHAR(t, []*har.Entry{
				newEapiHAREntry("/eapi/test", encryptedRequest["params"], tt.response),
			})

			output, executeErr := executeCryptoCommand(t,
				"--kind", "eapi",
				"decrypt", harFile,
			)
			require.NoError(t, executeErr)

			var results []Payload
			require.NoError(t, json.Unmarshal(output, &results))
			require.Len(t, results, 1)
			assert.Equal(t, "ok", results[0].Status)
			assert.JSONEq(t, tt.wantResponse, string(results[0].Response.Plaintext))
		})
	}
}

func TestCryptoDecryptEapiEnvelopeAllowsDelimiterInPayload(t *testing.T) {
	payload := map[string]string{"value": "before" + eapiEnvelopeDelimiter + "after"}
	encrypted, err := ncmcrypto.EApiEncrypt("/eapi/test", payload)
	require.NoError(t, err)

	output, err := executeCryptoCommand(t,
		"--kind", "eapi",
		"decrypt", encrypted["params"],
	)
	require.NoError(t, err)

	var result Payload
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, "/api/test", result.Request.Url)
	assert.JSONEq(t, `{"value":"before-36cd479b6b5-after"}`, string(result.Request.Plaintext))
}

func TestCryptoDecryptUsesConfiguredRootLogger(t *testing.T) {
	root := New()
	logger := &root.l
	previousLogger := projectlog.GetDefault()
	closed := false

	t.Cleanup(func() {
		if !closed {
			_ = root.l.Close()
		}

		projectlog.SetDefault(previousLogger)
	})

	home := t.TempDir()

	var output bytes.Buffer
	root.cmd.SetOut(&output)
	root.cmd.SetErr(&output)
	root.cmd.SetArgs([]string{
		"--config", filepath.Join("..", "..", "config", "config.yaml"),
		"--home", home,
		"--debug",
		"crypto", "--kind", "xeapi",
		"decrypt", "--encode", "hex", xeapiGoldenResponseHex,
	})

	require.NoError(t, root.cmd.Execute())
	require.Same(t, logger, projectlog.GetDefault())

	var result Payload
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(output.Bytes()), &result))
	assert.JSONEq(t, `{"code":200}`, string(result.Response.Plaintext))

	require.NoError(t, root.l.Close())

	closed = true
	logData, err := os.ReadFile(filepath.Join(home, ".ncmctl", "log", "ncm.log"))
	require.NoError(t, err)
	assert.Contains(t, string(logData), "[decryptRes] ciphertext_bytes=")
	assert.Contains(t, string(logData), "[decryptRes] decrypted_bytes=")
}

func executeCryptoCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()

	var output bytes.Buffer
	// Root registers commands before PersistentPreRunE creates its logger.
	command := NewCrypto(&Root{}, nil).Command()
	command.SetOut(&output)
	command.SetErr(&output)
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetArgs(args)

	err := command.Execute()
	return bytes.TrimSpace(output.Bytes()), err
}

func assertExactEnvelope(t *testing.T, want, got string) {
	t.Helper()

	if got != want {
		t.Errorf("raw XEAPI envelope = %q, want %q", got, want)
	}
}

func xeapiGoldenParams() map[string]string {
	return map[string]string{
		"B": xeapiGoldenB,
		"S": xeapiGoldenS,
		"R": xeapiGoldenR,
	}
}

func xeapiSessionFixture(t *testing.T) (string, map[string]string) {
	t.Helper()

	params, err := ncmcrypto.XeapiEncrypt(&ncmcrypto.XeapiEncryptRequest{
		URI:  "/api/test?id=1",
		Body: []byte{},
	}, ncmcrypto.XeapiPublicKeyState{
		PublicKey: "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:   "1000000000000",
		SK:        "server-key",
	}, ncmcrypto.XeapiSession{
		ID:  "session-id",
		Key: "0123456789abcdef",
	})
	require.NoError(t, err)
	return encodeXeapiParams(params), params
}

func encodeXeapiParams(params map[string]string) string {
	values := make(url.Values, len(params))
	for name, value := range params {
		values.Set(name, value)
	}
	return values.Encode()
}

func writeXeapiHAR(t *testing.T, params map[string]string, response []byte) string {
	t.Helper()

	return writeHAR(t, []*har.Entry{
		newXeapiHAREntry("/xeapi/params", &har.PostData{
			MimeType: "application/x-www-form-urlencoded",
			Params:   xeapiPostParams(params),
		}, response),
		newXeapiHAREntry("/xeapi/text", &har.PostData{
			MimeType: "application/x-www-form-urlencoded; charset=utf-8",
			Text:     encodeXeapiParams(params),
		}, response),
	})
}

func xeapiPostParams(params map[string]string) []*har.PostParam {
	postParams := make([]*har.PostParam, 0, len(params))
	for _, name := range []string{"B", "S", "R"} {
		postParams = append(postParams, &har.PostParam{Name: name, Value: params[name]})
	}
	return postParams
}

func newXeapiHAREntry(path string, postData *har.PostData, response []byte) *har.Entry {
	return &har.Entry{
		Request: &har.Request{
			Method:   "POST",
			URL:      "https://interface.music.163.com" + path,
			PostData: postData,
		},
		Response: &har.Response{
			Content: &har.Content{
				Size:     int64(len(response)),
				MimeType: "application/octet-stream",
				Text:     response,
				Encoding: "base64",
			},
		},
	}
}

func newEapiHAREntry(path, params string, response []byte) *har.Entry {
	return &har.Entry{
		Request: &har.Request{
			Method: "POST",
			URL:    "https://interface.music.163.com" + path,
			PostData: &har.PostData{
				MimeType: "application/x-www-form-urlencoded",
				Params: []*har.PostParam{
					{Name: "params", Value: params},
				},
			},
		},
		Response: &har.Response{
			Content: &har.Content{
				Size:     int64(len(response)),
				MimeType: "application/octet-stream",
				Text:     response,
				Encoding: "base64",
			},
		},
	}
}

func writeHAR(t *testing.T, entries []*har.Entry) string {
	t.Helper()

	document := har.Har{Log: &har.Log{
		Version: "1.2",
		Creator: &har.Creator{Name: "ncmctl-test", Version: "1"},
		Entries: entries,
	}}

	data, err := json.Marshal(document)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "capture.har")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}
