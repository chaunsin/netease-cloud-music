// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type testRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type trackedDownloadBody struct {
	reader     io.Reader
	closeCalls int
}

func (b *trackedDownloadBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *trackedDownloadBody) Close() error {
	b.closeCalls++
	return nil
}

type failingDownloadWriter struct {
	err error
}

func (w failingDownloadWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestDownloadClosesResponseBodyBeforeReturning(t *testing.T) {
	t.Parallel()

	const content = "downloaded music"

	body := &trackedDownloadBody{reader: bytes.NewBufferString(content)}
	client := newDownloadTestClient(body, http.StatusOK, int64(len(content)))

	var output bytes.Buffer

	response, err := client.Download(context.Background(), "https://example.com/song", nil, nil, &output, nil) //nolint:bodyclose // Download closes the response body before returning.
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, content, output.String())
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, int64(len(content)), response.ContentLength)
	assert.Equal(t, 1, body.closeCalls)
}

func TestDownloadClosesResponseBodyOnError(t *testing.T) {
	t.Parallel()

	errDownloadWrite := errors.New("download writer failed")

	tests := []struct {
		name          string
		statusCode    int
		contentLength int64
		writer        io.Writer
		wantError     string
		wantErrorIs   error
	}{
		{
			name:          "non-success status",
			statusCode:    http.StatusBadGateway,
			contentLength: 4,
			writer:        io.Discard,
			wantError:     "http status code: 502",
		},
		{
			name:          "writer failure",
			statusCode:    http.StatusOK,
			contentLength: 4,
			writer:        failingDownloadWriter{err: errDownloadWrite},
			wantErrorIs:   errDownloadWrite,
		},
		{
			name:          "content length mismatch",
			statusCode:    http.StatusOK,
			contentLength: 5,
			writer:        io.Discard,
			wantError:     "file transfer interrupted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &trackedDownloadBody{reader: bytes.NewBufferString("body")}
			client := newDownloadTestClient(body, tt.statusCode, tt.contentLength)

			response, err := client.Download(context.Background(), "https://example.com/song", nil, nil, tt.writer, nil) //nolint:bodyclose // Download closes the response body before returning.
			if tt.wantErrorIs != nil {
				require.ErrorIs(t, err, tt.wantErrorIs)
			} else {
				require.EqualError(t, err, tt.wantError)
			}

			assert.Nil(t, response)
			assert.Equal(t, 1, body.closeCalls)
		})
	}
}

func TestWeapiRequestSetsVisitorCookies(t *testing.T) {
	var (
		nnid string
		nuid string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if value, err := r.Cookie("_ntes_nnid"); err == nil {
			nnid = value.Value
		}

		if value, err := r.Cookie("_ntes_nuid"); err == nil {
			nuid = value.Value
		}

		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer server.Close()

	home := t.TempDir()
	logger := log.New(&log.Config{Level: "error"})
	client, err := NewClient(&Config{
		Timeout: time.Second,
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: filepath.Join(home, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	})

	opts := NewOptions()
	opts.Cookies = append(opts.Cookies, nil)

	var response map[string]any

	_, err = client.Request(
		context.Background(),
		server.URL+"/weapi/test",
		map[string]string{"id": "1"},
		&response,
		opts,
	)
	require.NoError(t, err)

	parts := strings.SplitN(nnid, ",", 2)
	require.Len(t, parts, 2)
	assert.Equal(t, nuid, parts[0])
	assert.Len(t, nuid, 32)
	assert.NotEmpty(t, parts[1])
}

func newDownloadTestClient(body io.ReadCloser, statusCode int, contentLength int64) *Client {
	restyClient := resty.New()
	restyClient.SetTransport(testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    statusCode,
			Header:        make(http.Header),
			Body:          body,
			ContentLength: contentLength,
		}, nil
	}))

	return &Client{cli: restyClient}
}

// The fixture is hex-encoded only for source readability; the HTTP response uses decoded binary bytes.
const eapiTestResponseCiphertextHex = "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08"

type eapiTestResponse struct {
	Code int  `json:"code"`
	Data bool `json:"data"`
}

type typedEAPIRequest struct {
	types.EApiReqCommon

	ID string `json:"id"`
}

type pointerEmbeddedEAPIRequest struct {
	*types.EApiReqCommon

	ID string `json:"id"`
}

type customEAPIRequest struct{}

func (customEAPIRequest) MarshalJSON() ([]byte, error) {
	return []byte(`{"id":"custom"}`), nil
}

func TestRequestEAPINormalizesUntypedPayloads(t *testing.T) {
	encryptedResponse := decodeEAPICiphertextFixture(t)

	tests := []struct {
		name string
		req  any
		id   string
	}{
		{
			name: "map",
			req:  map[string]any{"id": "map"},
			id:   "map",
		},
		{
			name: "custom marshaler",
			req:  customEAPIRequest{},
			id:   "custom",
		},
		{
			name: "empty header",
			req:  map[string]any{"id": "empty-header", "header": ""},
			id:   "empty-header",
		},
		{
			name: "null header",
			req:  map[string]any{"id": "null-header", "header": nil},
			id:   "null-header",
		},
		{
			name: "nil embedded common",
			req:  &pointerEmbeddedEAPIRequest{ID: "nil-embedded"},
			id:   "nil-embedded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var form url.Values

			client := newEAPIRequestTestClient(t, encryptedResponse, &form)

			var reply eapiTestResponse

			_, err := client.Request(
				context.Background(),
				"https://example.test/eapi/test",
				tt.req,
				&reply,
				NewOptions().SetEAPI().SetHeader("x-aeapi", "false"),
			)
			require.NoError(t, err)
			assert.Equal(t, eapiTestResponse{Code: http.StatusOK, Data: true}, reply)

			payload := decryptEAPIFormPayload(t, form)
			assertEAPIDefaults(t, payload, true)

			var id string
			require.NoError(t, json.Unmarshal(payload["id"], &id))
			assert.Equal(t, tt.id, id)
		})
	}
}

func TestRequestEAPITypedPayloadHonorsPlainResponse(t *testing.T) {
	var (
		form          url.Values
		requestHeader http.Header
	)

	client := newEAPIRequestTestClientWithHeaders(t, []byte(`{"code":200,"data":false}`), &form, &requestHeader)

	req := &typedEAPIRequest{ID: "typed"}
	req.SetResponseEncrypted(false)

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		req,
		&reply,
		NewOptions().SetEAPI(),
	)
	require.NoError(t, err)
	assert.Equal(t, eapiTestResponse{Code: http.StatusOK}, reply)
	require.NotNil(t, req.ER)
	assert.False(t, *req.ER)
	assert.Empty(t, req.Header)

	payload := decryptEAPIFormPayload(t, form)
	assertEAPIDefaults(t, payload, false)
	assert.Equal(t, "true", requestHeader.Get("X-Aeapi"))
}

func TestRequestEAPIResponseModes(t *testing.T) {
	plainResponse := []byte(`{"code":200,"data":true}`)
	tests := []struct {
		name              string
		responseEncrypted bool
		xAEAPI            string
		responseBody      []byte
	}{
		{
			name:              "e_r=false x-aeapi=false",
			responseEncrypted: false,
			xAEAPI:            "false",
			responseBody:      plainResponse,
		},
		{
			name:              "e_r=false x-aeapi=true",
			responseEncrypted: false,
			xAEAPI:            "true",
			responseBody:      plainResponse,
		},
		{
			name:              "e_r=true x-aeapi=false",
			responseEncrypted: true,
			xAEAPI:            "false",
			responseBody:      decodeEAPICiphertextFixture(t),
		},
		{
			name:              "e_r=true x-aeapi=true",
			responseEncrypted: true,
			xAEAPI:            "true",
			responseBody:      encryptEAPIResponse(t, gzipEAPIResponse(t, plainResponse)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				form          url.Values
				requestHeader http.Header
			)

			client := newEAPIRequestTestClientWithHeaders(t, tt.responseBody, &form, &requestHeader)

			var reply eapiTestResponse

			_, err := client.Request(
				context.Background(),
				"https://example.test/eapi/test",
				map[string]any{"id": tt.name, "e_r": tt.responseEncrypted},
				&reply,
				NewOptions().SetEAPI().SetHeader("x-aeapi", tt.xAEAPI),
			)
			require.NoError(t, err)
			assert.Equal(t, eapiTestResponse{Code: http.StatusOK, Data: true}, reply)
			assert.Equal(t, tt.xAEAPI, requestHeader.Get("X-Aeapi"))
			assertEAPIDefaults(t, decryptEAPIFormPayload(t, form), tt.responseEncrypted)
		})
	}
}

func TestRequestEAPIRejectsPlainResponseWhenEncrypted(t *testing.T) {
	client := newEAPIRequestTestClient(t, []byte(`{"code":200}`), nil)

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		map[string]any{"id": "map"},
		&reply,
		NewOptions().SetEAPI().SetHeader("x-aeapi", "false"),
	)
	require.ErrorContains(t, err, "EApiDecrypt")
}

func TestRequestEAPITypedPayloadDefaultsToEncryptedResponse(t *testing.T) {
	encryptedResponse := decodeEAPICiphertextFixture(t)

	var (
		form          url.Values
		requestHeader http.Header
	)

	client := newEAPIRequestTestClientWithHeaders(t, encryptedResponse, &form, &requestHeader)
	req := &typedEAPIRequest{ID: "typed"}

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		req,
		&reply,
		NewOptions().SetEAPI().SetHeader("x-aeapi", "false"),
	)
	require.NoError(t, err)
	assert.Equal(t, eapiTestResponse{Code: http.StatusOK, Data: true}, reply)
	assert.Nil(t, req.ER)
	assert.Empty(t, req.Header)

	payload := decryptEAPIFormPayload(t, form)
	assertEAPIDefaults(t, payload, true)
	assert.Equal(t, "false", requestHeader.Get("X-Aeapi"))
}

func TestRequestEAPIDefaultsToEncryptedGzipResponse(t *testing.T) {
	var (
		form          url.Values
		requestHeader http.Header
	)

	responseBody := encryptEAPIResponse(t, gzipEAPIResponse(t, []byte(`{"code":200,"data":true}`)))
	client := newEAPIRequestTestClientWithHeaders(t, responseBody, &form, &requestHeader)

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		map[string]any{"id": "gzip"},
		&reply,
		NewOptions().SetEAPI(),
	)
	require.NoError(t, err)
	assert.Equal(t, eapiTestResponse{Code: http.StatusOK, Data: true}, reply)
	assert.Equal(t, "true", requestHeader.Get("X-Aeapi"))
	assertEAPIDefaults(t, decryptEAPIFormPayload(t, form), true)
}

func TestRequestEAPIRejectsInvalidResponseEncryptionFlag(t *testing.T) {
	var calls int

	client := newEAPIRequestTestClient(t, []byte(`{"code":200}`), nil)
	client.GetClient().Transport = testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, assert.AnError
	})

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		map[string]any{"e_r": "true"},
		&reply,
		NewOptions().SetEAPI(),
	)
	require.EqualError(t, err, "prepare EAPI request: decode e_r: json: cannot unmarshal string into Go value of type bool")
	assert.Zero(t, calls)
}

func TestRequestRejectsTypedNilEAPIRequest(t *testing.T) {
	var calls int

	client := newEAPIRequestTestClient(t, []byte(`{"code":200}`), nil)
	client.GetClient().Transport = testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, assert.AnError
	})

	var req *typedEAPIRequest

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		req,
		&reply,
		NewOptions().SetEAPI(),
	)
	require.EqualError(t, err, "request args invalid")
	assert.Zero(t, calls)
}

func TestRequestEAPIError(t *testing.T) {
	encryptedResponse := decodeEAPICiphertextFixture(t)

	tests := []struct {
		name          string
		req           any
		responseBody  []byte
		wantErr       string
		wantReply     eapiTestResponse
		wantEncrypted bool
	}{
		{
			name:          "plain response",
			req:           map[string]any{"id": "plain", "e_r": false},
			responseBody:  []byte(`{"code":502,"message":"upstream unavailable"}`),
			wantReply:     eapiTestResponse{Code: http.StatusBadGateway},
			wantEncrypted: false,
		},
		{
			name:          "binary encrypted response",
			req:           map[string]any{"id": "encrypted"},
			responseBody:  encryptedResponse,
			wantReply:     eapiTestResponse{Code: http.StatusOK, Data: true},
			wantEncrypted: true,
		},
		{
			name:          "plain body when encrypted response requested",
			req:           map[string]any{"id": "fallback"},
			responseBody:  []byte(`{"code":502,"message":"upstream unavailable"}`),
			wantErr:       "EApiDecrypt",
			wantEncrypted: true,
		},
		{
			name:          "malformed encrypted response",
			req:           map[string]any{"id": "malformed"},
			responseBody:  []byte("not-json"),
			wantErr:       "EApiDecrypt",
			wantEncrypted: true,
		},
		{
			name:          "invalid plain response",
			req:           map[string]any{"id": "invalid", "e_r": false},
			responseBody:  []byte("not-json"),
			wantErr:       "json.NewDecoder",
			wantEncrypted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var form url.Values

			client := newEAPIResponseTestClient(t, http.StatusBadGateway, tt.responseBody, &form)

			var reply eapiTestResponse

			response, err := client.Request(
				context.Background(),
				"https://example.test/eapi/test",
				tt.req,
				&reply,
				NewOptions().SetEAPI().SetHeader("x-aeapi", "false"),
			)
			require.Nil(t, response)

			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			assert.Equal(t, tt.wantReply, reply)

			if tt.wantErr == "" {
				require.NoError(t, apiErr.Err)
				assert.Equal(t, "http status code: 502", err.Error())
			} else {
				require.ErrorContains(t, apiErr.Err, tt.wantErr)
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorIs(t, err, apiErr.Err)
			}

			assertEAPIDefaults(t, decryptEAPIFormPayload(t, form), tt.wantEncrypted)
		})
	}
}

func newEAPIRequestTestClient(t *testing.T, responseBody []byte, capturedForm *url.Values) *Client {
	t.Helper()

	return newEAPIResponseTestClient(t, http.StatusOK, responseBody, capturedForm)
}

func newEAPIRequestTestClientWithHeaders(t *testing.T, responseBody []byte, capturedForm *url.Values, capturedHeader *http.Header) *Client {
	t.Helper()

	return newEAPIResponseTestClientWithHeaders(t, http.StatusOK, responseBody, capturedForm, capturedHeader)
}

func newEAPIResponseTestClient(t *testing.T, statusCode int, responseBody []byte, capturedForm *url.Values) *Client {
	t.Helper()

	return newEAPIResponseTestClientWithHeaders(t, statusCode, responseBody, capturedForm, nil)
}

func newEAPIResponseTestClientWithHeaders(t *testing.T, statusCode int, responseBody []byte, capturedForm *url.Values, capturedHeader *http.Header) *Client {
	t.Helper()

	home := t.TempDir()
	logger := log.New(&log.Config{Level: "error"})
	client, err := NewClient(&Config{
		Timeout: time.Second,
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: filepath.Join(home, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	})

	client.GetClient().Transport = testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}

		if capturedHeader != nil {
			*capturedHeader = request.Header.Clone()
		}

		if capturedForm != nil {
			form, err := url.ParseQuery(string(body))
			if err != nil {
				return nil, err
			}

			*capturedForm = form
		}

		return &http.Response{
			StatusCode:    statusCode,
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(responseBody)),
			ContentLength: int64(len(responseBody)),
			Request:       request,
		}, nil
	})
	return client
}

func gzipEAPIResponse(t *testing.T, plaintext []byte) []byte {
	t.Helper()

	var compressed bytes.Buffer

	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write(plaintext)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return compressed.Bytes()
}

func decodeEAPICiphertextFixture(t *testing.T) []byte {
	t.Helper()

	ciphertext, err := hex.DecodeString(eapiTestResponseCiphertextHex)
	require.NoError(t, err)
	return ciphertext
}

func encryptEAPIResponse(t *testing.T, plaintext []byte) []byte {
	t.Helper()

	block, err := aes.NewCipher([]byte("e82ckenh8dichen8"))
	require.NoError(t, err)

	padded, err := crypto.Pkcs7Padding(plaintext, block.BlockSize())
	require.NoError(t, err)

	return crypto.AesEncryptECB(block, padded)
}

func decryptEAPIFormPayload(t *testing.T, form url.Values) map[string]json.RawMessage {
	t.Helper()

	plaintext, err := crypto.EApiDecrypt(form.Get("params"), "hex")
	require.NoError(t, err)

	parts := strings.Split(string(plaintext), "-36cd479b6b5-")
	require.Len(t, parts, 3)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(parts[1]), &payload))
	return payload
}

func assertEAPIDefaults(t *testing.T, payload map[string]json.RawMessage, wantEncrypted bool) {
	t.Helper()

	var (
		header    string
		encrypted bool
	)
	require.NoError(t, json.Unmarshal(payload["header"], &header))
	require.NoError(t, json.Unmarshal(payload["e_r"], &encrypted))
	assert.Equal(t, "{}", header)
	assert.Equal(t, wantEncrypted, encrypted)
}
