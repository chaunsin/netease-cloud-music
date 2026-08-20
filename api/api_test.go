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
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cheggaaa/pb/v3"
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

func TestDownloadConcurrentRequestsPreserveClientTimeout(t *testing.T) {
	const requestCount = 32

	restyClient := resty.New().SetTimeout(time.Second)
	restyClient.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		_, hasDeadline := request.Context().Deadline()
		switch request.URL.Path {
		case "/song":
			if hasDeadline {
				return nil, errors.New("download request inherited the API client timeout")
			}
		case "/api":
			if !hasDeadline {
				return nil, errors.New("API request lost the client timeout")
			}
		}

		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader("x")),
			ContentLength: 1,
			Request:       request,
		}, nil
	}))
	client := &Client{cli: restyClient}

	start := make(chan struct{})
	errs := make(chan error, requestCount*2)

	var wg sync.WaitGroup

	for range requestCount {
		wg.Add(2)

		go func() {
			defer wg.Done()

			<-start

			_, err := client.Download(context.Background(), "https://example.com/song", nil, nil, io.Discard, nil) //nolint:bodyclose // Download closes the response body before returning.
			errs <- err
		}()

		go func() {
			defer wg.Done()

			<-start

			request, err := http.NewRequest(http.MethodGet, "https://example.com/api", http.NoBody)
			if err != nil {
				errs <- err
				return
			}

			response, err := client.GetClient().Do(request)
			if response != nil {
				err = errors.Join(err, response.Body.Close())
			}

			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, time.Second, client.GetClient().Timeout)
}

func TestWeapiRequestSetsVisitorCookies(t *testing.T) {
	var (
		nnid string
		nuid string
	)

	client := newCookieTransportTestClient(t)
	client.SetTransport(testRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if value, err := r.Cookie("_ntes_nnid"); err == nil {
			nnid = value.Value
		}

		if value, err := r.Cookie("_ntes_nuid"); err == nil {
			nuid = value.Value
		}

		return responseWithCookies(r, http.StatusOK, nil), nil
	}))

	opts := NewOptions()
	opts.SetCookies(nil)

	var response map[string]any

	_, err := client.Request(
		context.Background(),
		"https://music.163.com/weapi/test",
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

func newIdentitySeedClient(t *testing.T, cookieFile string) (*Client, *log.Logger) {
	t.Helper()

	logger := log.New(&log.Config{Level: "error"})
	client, err := NewClient(&Config{
		Timeout: time.Second,
		HomeDir: t.TempDir(),
		Cookie: cookie.Config{
			Filepath: cookieFile,
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)
	return client, logger
}

func TestNewClientSeedsIdentityCookies(t *testing.T) {
	musicURL := mustParseURL(t, "https://music.163.com")
	identityNames := []string{"WNMCID", "_ntes_nnid", "_ntes_nuid"}

	first, logger := newIdentitySeedClient(t, filepath.Join(t.TempDir(), "cookie.json"))
	firstValues := cookieValues(first.GetCookies(musicURL))

	for _, name := range identityNames {
		assert.NotEmpty(t, firstValues[name], name)
	}

	require.NoError(t, first.Close(context.Background()))
	require.NoError(t, logger.Close())

	// 第二次启动必须复用 jar 中持久化的值,而不是重新生成。
	second, logger := newIdentitySeedClient(t, first.cfg.Cookie.Filepath)
	defer func() {
		require.NoError(t, second.Close(context.Background()))
		require.NoError(t, logger.Close())
	}()

	secondValues := cookieValues(second.GetCookies(musicURL))
	for _, name := range identityNames {
		assert.Equal(t, firstValues[name], secondValues[name], name)
	}
}

func TestNewClientKeepsExistingIdentityCookies(t *testing.T) {
	musicURL := mustParseURL(t, "https://music.163.com")

	first, logger := newIdentitySeedClient(t, filepath.Join(t.TempDir(), "cookie.json"))
	first.SetCookies(musicURL, []*http.Cookie{{Name: "_ntes_nuid", Value: "custom-nuid"}})
	require.NoError(t, first.Close(context.Background()))
	require.NoError(t, logger.Close())

	// 已存在的 _ntes_nuid 不能被 seed 覆盖,缺失的其余身份 Cookie 仍会被补齐。
	second, logger := newIdentitySeedClient(t, first.cfg.Cookie.Filepath)
	defer func() {
		require.NoError(t, second.Close(context.Background()))
		require.NoError(t, logger.Close())
	}()

	values := cookieValuesByName(second.GetCookies(musicURL), "_ntes_nuid")
	assert.Contains(t, values, "custom-nuid")

	secondValues := cookieValues(second.GetCookies(musicURL))
	assert.NotEmpty(t, secondValues["WNMCID"])
	assert.NotEmpty(t, secondValues["_ntes_nnid"])
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

type omitemptyEAPIRequest struct {
	DeviceID string `json:"deviceId,omitempty"`
	ID       string `json:"id"`
}

func TestRequestEAPIPreservesOmittedDeviceID(t *testing.T) {
	var form url.Values

	client := newEAPIRequestTestClient(t, decodeEAPICiphertextFixture(t), &form)
	req := &omitemptyEAPIRequest{ID: "omitempty"}

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://music.163.com/eapi/test",
		req,
		&reply,
		NewOptions().SetEAPI(),
	)
	require.NoError(t, err)
	assert.Equal(t, eapiTestResponse{Code: http.StatusOK, Data: true}, reply)
	assert.Empty(t, req.DeviceID)

	payload := decryptEAPIFormPayload(t, form)
	assert.NotContains(t, payload, "deviceId")
}

func TestRequestEAPIOutsideProtocolCookieDomainDoesNotInjectDefaultDeviceID(t *testing.T) {
	var form url.Values

	client := newEAPIRequestTestClient(t, decodeEAPICiphertextFixture(t), &form)
	req := &omitemptyEAPIRequest{ID: "non-music"}

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		req,
		&reply,
		NewOptions().SetEAPI(),
	)
	require.NoError(t, err)
	assert.Empty(t, req.DeviceID)

	payload := decryptEAPIFormPayload(t, form)
	assert.NotContains(t, payload, "deviceId")
}

type customIdentityEAPIRequest struct{}

func (customIdentityEAPIRequest) MarshalJSON() ([]byte, error) {
	return []byte(`{"deviceId":"caller-device","id":"custom"}`), nil
}

func TestRequestEAPIPreservesDeviceIDFromCustomJSON(t *testing.T) {
	var form url.Values

	client := newEAPIRequestTestClient(t, decodeEAPICiphertextFixture(t), &form)

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://music.163.com/eapi/test",
		customIdentityEAPIRequest{},
		&reply,
		NewOptions().SetEAPI(),
	)
	require.NoError(t, err)
	assert.Equal(t, eapiTestResponse{Code: http.StatusOK, Data: true}, reply)

	payload := decryptEAPIFormPayload(t, form)
	assertEAPIStringField(t, payload, "deviceId", "caller-device")
	assertEAPIStringField(t, payload, "id", "custom")
}

func TestRequestWEAPIInjectsSnapshotCSRFTokWithoutMutatingPayload(t *testing.T) {
	var requestURL *url.URL

	client := newEAPIRequestTestClient(t, []byte(`{"code":200}`), nil)
	client.SetCookies(mustParseURL(t, "https://music.163.com/weapi/test"), []*http.Cookie{{Name: "__csrf", Value: "snapshot-csrf"}})
	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		requestURL = cloneURL(request.URL)
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	req := map[string]string{"id": "weapi"}

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://music.163.com/weapi/test",
		req,
		&reply,
		NewOptions().SetWEAPI(),
	)
	require.NoError(t, err)
	assert.Equal(t, eapiTestResponse{Code: http.StatusOK}, reply)
	assert.Equal(t, map[string]string{"id": "weapi"}, req)
	require.NotNil(t, requestURL)
	assert.Equal(t, "snapshot-csrf", requestURL.Query().Get("csrf_token"))
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
	client.SetTransport(testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, assert.AnError
	}))

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
	client.SetTransport(testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, assert.AnError
	}))

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

func TestRequestRejectsNilContext(t *testing.T) {
	var calls int

	client := newEAPIRequestTestClient(t, []byte(`{"code":200}`), nil)
	client.SetTransport(testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, assert.AnError
	}))

	var reply eapiTestResponse

	_, err := client.Request(
		nil, //nolint:staticcheck // A nil Context is the invalid input under test.
		"https://example.test/eapi/test",
		map[string]string{"id": "1"},
		&reply,
		NewOptions().SetEAPI(),
	)
	require.EqualError(t, err, "request context is nil")
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

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
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
	}))
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

func assertEAPIStringField(t *testing.T, payload map[string]json.RawMessage, name, want string) {
	t.Helper()

	var got string
	require.NoError(t, json.Unmarshal(payload[name], &got))
	assert.Equal(t, want, got)
}

func TestClientCookieRetrievalAndUnescape(t *testing.T) {
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

	uri := mustParseURL(t, "https://music.163.com")
	client.SetCookies(uri, []*http.Cookie{
		{Name: "__csrf", Value: "hello%20world%2Btest"},
		{Name: "deviceId", Value: "device%2F123"},
		{Name: "sDeviceId", Value: "sdevice%3D456"},
		{Name: "MUSIC_U", Value: "token%26user"},
		{Name: "MUSIC_A", Value: "anon%3Dtoken"},
	})

	csrf, ok := client.GetCSRF("https://music.163.com")
	require.True(t, ok)
	assert.Equal(t, "hello world+test", csrf)

	assert.Equal(t, "device/123", client.GetDeviceId())
	assert.Equal(t, "sdevice=456", client.GetSDeviceId())
	assert.Equal(t, "token&user", client.GetMusicU())
	assert.Equal(t, "anon=token", client.GetMusicA())
	assert.Equal(t, "device/123", client.GetCookieValue("deviceId"))

	// 测试 fallback 当 URL 解码非法时
	client.SetCookies(uri, []*http.Cookie{
		{Name: "invalid_escape", Value: "bad%percent%"},
	})
	assert.Equal(t, "bad%percent%", client.GetCookieValue("invalid_escape"))
}

func TestDownloadValidationAndChunked(t *testing.T) {
	client := newDownloadTestClient(&trackedDownloadBody{reader: bytes.NewBufferString("chunked body")}, http.StatusOK, -1)

	var output bytes.Buffer
	// nil context 校验
	//nolint:bodyclose,staticcheck // A nil Context fails before Download creates a response.
	_, err := client.Download(nil, "https://example.com/song", nil, nil, &output, nil)
	require.Error(t, err)

	// nil writer 校验
	//nolint:bodyclose // A nil writer fails before Download creates a response.
	_, err = client.Download(context.Background(), "https://example.com/song", nil, nil, nil, nil)
	require.Error(t, err)

	// ContentLength = -1 时下载成功
	//nolint:bodyclose // Download closes the response body before returning.
	resp, err := client.Download(context.Background(), "https://example.com/song", nil, nil, &output, nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "chunked body", output.String())
}

type uploadTestResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	DocID   string `json:"docId"`
}

func TestUploadValidation(t *testing.T) {
	t.Parallel()

	client := newCookieTransportTestClient(t)

	var (
		validData         = strings.NewReader("hello upload")
		varTypedNilReader *bytes.Reader
		varTypedNilResp   *uploadTestResp
		reply             uploadTestResp
	)

	// 1. ctx == nil
	_, err := client.Upload(nil, "https://music.163.com/upload", validData, &reply, nil, nil) //nolint:staticcheck // A nil Context is the invalid input under test.
	require.EqualError(t, err, "upload context is nil")

	// 2. url == ""
	_, err = client.Upload(context.Background(), "", validData, &reply, nil, nil)
	require.EqualError(t, err, "upload args invalid")

	// 3. data == nil
	_, err = client.Upload(context.Background(), "https://music.163.com/upload", nil, &reply, nil, nil)
	require.EqualError(t, err, "upload args invalid")

	// 4. typed nil data
	_, err = client.Upload(context.Background(), "https://music.163.com/upload", varTypedNilReader, &reply, nil, nil)
	require.EqualError(t, err, "upload args invalid")

	// 5. typed nil resp
	_, err = client.Upload(context.Background(), "https://music.163.com/upload", validData, varTypedNilResp, nil, nil)
	require.EqualError(t, err, "upload args invalid")

	// 6. opts.cookieErr != nil
	badOpts := NewOptions()
	badOpts.SetCookies(&http.Cookie{Name: ""}) // invalid cookie name causes cookieErr

	_, err = client.Upload(context.Background(), "https://music.163.com/upload", validData, &reply, badOpts, nil)
	require.ErrorContains(t, err, "prepare upload Cookies")

	// 7. unknown crypto mode
	unknownOpts := &Options{CryptoMode: CryptoMode("unknown")}

	_, err = client.Upload(context.Background(), "https://music.163.com/upload", validData, &reply, unknownOpts, nil)
	require.ErrorContains(t, err, "unknown crypto mode unknown")
}

func TestUploadMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		opts       *Options
		wantMethod string
		wantErr    string
	}{
		{
			name:       "nil opts defaults to POST",
			opts:       nil,
			wantMethod: http.MethodPost,
		},
		{
			name:       "empty method defaults to POST",
			opts:       &Options{CryptoMode: CryptoModeWEAPI},
			wantMethod: http.MethodPost,
		},
		{
			name:       "explicit POST",
			opts:       NewOptions().SetMethod(http.MethodPost),
			wantMethod: http.MethodPost,
		},
		{
			name:       "explicit PUT",
			opts:       NewOptions().SetMethod(http.MethodPut),
			wantMethod: http.MethodPut,
		},
		{
			name:    "unsupported method GET",
			opts:    NewOptions().SetMethod(http.MethodGet),
			wantErr: "GET not surpport http method",
		},
		{
			name:    "unsupported method DELETE",
			opts:    NewOptions().SetMethod(http.MethodDelete),
			wantErr: "DELETE not surpport http method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedMethod string

			client := newCookieTransportTestClient(t)
			client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				capturedMethod = req.Method

				return responseWithCookies(req, http.StatusOK, nil), nil
			}))

			var reply uploadTestResp

			resp, err := client.Upload(
				context.Background(),
				"https://music.163.com/upload",
				strings.NewReader("data"),
				&reply,
				tt.opts,
				nil,
			)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, resp)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.wantMethod, capturedMethod)
		})
	}
}

func TestUploadHeadersAndCookies(t *testing.T) {
	t.Parallel()

	t.Run("protocol domain injects defaults and merges options", func(t *testing.T) {
		t.Parallel()

		var (
			capturedHeader http.Header
			capturedBody   []byte
		)

		client := newCookieTransportTestClient(t)
		client.GetAnonymous().Set("test-anonymous-token")
		client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Clone()

			var readErr error

			capturedBody, readErr = io.ReadAll(req.Body)
			if readErr != nil {
				return nil, readErr
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":200,"message":"ok","docId":"12345"}`))),
				Request:    req,
			}, nil
		}))

		opts := NewOptions().
			SetMethod(http.MethodPut).
			SetHeader("X-Nos-Token", "test-token").
			SetHeader("Content-Type", "image/png").
			SetHeader("User-Agent", "CustomUAUploader/1.0")
		opts.SetCookies(&http.Cookie{Name: "custom_opt_cookie", Value: "opt_val"})

		var reply uploadTestResp

		resp, err := client.Upload(
			context.Background(),
			"https://music.163.com/upload/file",
			strings.NewReader("binary-image-data"),
			&reply,
			opts,
			nil,
		)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, uploadTestResp{Code: 200, Message: "ok", DocID: "12345"}, reply)
		assert.Equal(t, "binary-image-data", string(capturedBody))

		// 检查 Headers
		assert.Equal(t, "test-token", capturedHeader.Get("X-Nos-Token"))
		assert.Equal(t, "image/png", capturedHeader.Get("Content-Type"))
		assert.Equal(t, "CustomUAUploader/1.0", capturedHeader.Get("User-Agent"))
		assert.Equal(t, "https://music.163.com", capturedHeader.Get("Referer"))

		// 检查 Cookies
		cookieHeader := capturedHeader.Get("Cookie")
		assert.Contains(t, cookieHeader, "custom_opt_cookie=opt_val")
		assert.Contains(t, cookieHeader, "WNMCID=")
		assert.Contains(t, cookieHeader, "deviceId=")
		assert.Contains(t, cookieHeader, "MUSIC_A=test-anonymous-token")
	})

	t.Run("external domain does not inject protocol default cookies", func(t *testing.T) {
		t.Parallel()

		var capturedHeader http.Header

		client := newCookieTransportTestClient(t)
		client.GetAnonymous().Set("test-anonymous-token")
		client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Clone()

			return responseWithCookies(req, http.StatusOK, nil), nil
		}))

		opts := NewOptions().
			SetHeader("X-Nos-Token", "nos-token")
		opts.SetCookies(&http.Cookie{Name: "ext_cookie", Value: "ext_val"})

		resp, err := client.Upload(
			context.Background(),
			"http://nosup-hz1.127.net/upload",
			strings.NewReader("part-data"),
			nil,
			opts,
			nil,
		)
		require.NoError(t, err)
		assert.NotNil(t, resp)

		cookieHeader := capturedHeader.Get("Cookie")
		assert.Contains(t, cookieHeader, "ext_cookie=ext_val")
		assert.NotContains(t, cookieHeader, "WNMCID=")
		assert.NotContains(t, cookieHeader, "deviceId=")
		assert.NotContains(t, cookieHeader, "MUSIC_A=")
	})

	t.Run("protocol domain with existing MUSIC_U skips MUSIC_A default", func(t *testing.T) {
		t.Parallel()

		var capturedHeader http.Header

		client := newCookieTransportTestClient(t)
		client.GetAnonymous().Set("test-anonymous-token")
		client.SetCookies(mustParseURL(t, "https://music.163.com"), []*http.Cookie{
			{Name: "MUSIC_U", Value: "user_login_token"},
		})
		client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Clone()

			return responseWithCookies(req, http.StatusOK, nil), nil
		}))

		_, err := client.Upload(
			context.Background(),
			"https://music.163.com/upload",
			strings.NewReader("data"),
			nil,
			nil,
			nil,
		)
		require.NoError(t, err)

		cookieHeader := capturedHeader.Get("Cookie")
		assert.Contains(t, cookieHeader, "MUSIC_U=user_login_token")
		assert.NotContains(t, cookieHeader, "MUSIC_A=")
	})
}

func TestUploadProgressBar(t *testing.T) {
	t.Parallel()

	var (
		capturedBody []byte
		content      = "progress bar upload payload test"
	)

	client := newCookieTransportTestClient(t)
	client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var readErr error

		capturedBody, readErr = io.ReadAll(req.Body)
		if readErr != nil {
			return nil, readErr
		}

		return responseWithCookies(req, http.StatusOK, nil), nil
	}))

	bar := pb.New64(int64(len(content)))
	bar.SetWriter(io.Discard)
	bar.Start()

	resp, err := client.Upload(
		context.Background(),
		"https://music.163.com/upload",
		strings.NewReader(content),
		nil,
		nil,
		bar,
	)
	bar.Finish()

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, content, string(capturedBody))
}

func TestUploadResponseHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		withResp      bool
		wantErrIsAPI  bool
		wantAPIStatus int
		wantErr       string
		wantReply     uploadTestResp
	}{
		{
			name:         "http 200 with valid json response",
			statusCode:   http.StatusOK,
			responseBody: `{"code":200,"message":"success","docId":"doc_999"}`,
			withResp:     true,
			wantReply:    uploadTestResp{Code: 200, Message: "success", DocID: "doc_999"},
		},
		{
			name:         "http 200 with nil resp arg",
			statusCode:   http.StatusOK,
			responseBody: `{"code":200,"message":"ignored"}`,
			withResp:     false,
		},
		{
			name:          "http 500 with json error response",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"code":500,"message":"server internal error"}`,
			withResp:      true,
			wantErrIsAPI:  true,
			wantAPIStatus: http.StatusInternalServerError,
			wantErr:       "http status code: 500",
			wantReply:     uploadTestResp{Code: 500, Message: "server internal error"},
		},
		{
			name:          "http 200 with malformed json response",
			statusCode:    http.StatusOK,
			responseBody:  `not-valid-json`,
			withResp:      true,
			wantErrIsAPI:  true,
			wantAPIStatus: http.StatusOK,
			wantErr:       "json.NewDecoder",
		},
		{
			name:          "http 502 with non-json response",
			statusCode:    http.StatusBadGateway,
			responseBody:  `bad gateway html`,
			withResp:      true,
			wantErrIsAPI:  true,
			wantAPIStatus: http.StatusBadGateway,
			wantErr:       "json.NewDecoder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := newCookieTransportTestClient(t)
			client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tt.statusCode,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader([]byte(tt.responseBody))),
					Request:    req,
				}, nil
			}))

			var (
				reply  uploadTestResp
				target any
			)

			if tt.withResp {
				target = &reply
			}

			resp, err := client.Upload(
				context.Background(),
				"https://music.163.com/upload",
				strings.NewReader("payload"),
				target,
				nil,
				nil,
			)

			if tt.wantErrIsAPI {
				require.Error(t, err)

				var apiErr *APIError

				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tt.wantAPIStatus, apiErr.StatusCode)
				assert.Contains(t, err.Error(), tt.wantErr)

				if tt.wantReply != (uploadTestResp{}) {
					assert.Equal(t, tt.wantReply, reply)
				}

				assert.Nil(t, resp)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)

			if tt.withResp {
				assert.Equal(t, tt.wantReply, reply)
			}
		})
	}
}

func TestUploadContextCancellation(t *testing.T) {
	t.Parallel()

	client := newCookieTransportTestClient(t)
	client.SetTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()

		return nil, req.Context().Err()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := client.Upload(ctx, "https://music.163.com/upload", strings.NewReader("data"), nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), context.Canceled.Error())
}
