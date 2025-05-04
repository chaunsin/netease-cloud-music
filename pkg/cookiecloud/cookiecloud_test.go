// MIT License
//
// Copyright (c) 2025 chaunsin
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

package cookiecloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	password = "kTduz4A61D4a9LwS7jGKRu"
	encrypt  = ""
	database = map[string]Cookie{
		"uuid-01": {
			CookieData: map[string][]CookieData{
				"muisc.163.com": {
					{
						Domain:         ".music.163.com",
						ExpirationDate: 1780372733.231195,
						HostOnly:       false,
						HttpOnly:       true,
						Name:           "MUSIC_A_T",
						Path:           "/openapi/clientlog",
						SameSite:       "unspecified",
						Secure:         false,
						Session:        false,
						StoreId:        "0",
						Value:          "1510296972270",
					},
					{
						Domain:         ".music.163.com",
						ExpirationDate: 1761364733.230737,
						HostOnly:       false,
						HttpOnly:       true,
						Name:           "MUSIC_U",
						Path:           "/",
						SameSite:       "unspecified",
						Secure:         false,
						Session:        false,
						StoreId:        "0",
						Value:          "00E439E5xxxyyyzzz",
					},
				},
			},
			LocalStorageData: map[string]map[string]string{
				"muisc.163.com": {
					"uid":      "1234456",
					"loglevel": "SILENT",
					"debug":    "false",
				},
			},
		},
	}
)

// fakeServer https://github.com/easychen/CookieCloud/blob/master/api/app.js
func fakeServer(t *testing.T, now time.Time) *httptest.Server {
	mux := http.NewServeMux()

	// 模拟/push接口
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req Body
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		if req.Uuid == "" || req.Encrypted == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(PushResp{Action: "done"}); err != nil {
			t.Errorf("json.NewEncoder() error = %v", err)
		}
	})

	// 模拟/get接口
	mux.HandleFunc("/get/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		// fmt.Println("uuid:", uuid)
		if uuid == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Decode:"+err.Error())
				return
			}
		}

		cookie, ok := database[uuid]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		cookie.UpdateTime = now

		// 不为空则走解密逻辑
		if req.Password == "" {
			data, err := json.Marshal(cookie)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Marshal: "+err.Error())
				return
			}

			keyPassword := Md5String(uuid, "-", password)[:16]
			encrypt, err = Encrypt(keyPassword, string(data))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Encrypt: "+err.Error())
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(Body{
				Uuid:      uuid,
				Encrypted: encrypt,
			}); err != nil {
				t.Errorf("json.NewEncoder() error = %v", err)
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// 服务端解密逻辑
			if err := json.NewEncoder(w).Encode(cookie); err != nil {
				t.Errorf("json.NewEncoder() error = %v", err)
			}
		}
	})

	return httptest.NewServer(mux)
}

// GODEBUG=httpmuxgo121=0 go test -v -run TestClient_Get
func TestClient_Get(t *testing.T) {
	now := time.Now().UTC()
	server := fakeServer(t, now)
	defer server.Close()

	cli, err := NewClient(&Config{
		ApiUrl: server.URL,
		Debug:  false,
	})
	if err != nil {
		t.Errorf("NewClient() error = %v", err)
		return
	}

	type args struct {
		ctx context.Context
		req *GetReq
	}
	tests := []struct {
		name    string
		args    args
		want    *GetResp
		wantErr bool
	}{
		{
			name:    "not found uid",
			args:    args{ctx: context.Background(), req: &GetReq{Uuid: "notfound_id", Password: password}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "required password params",
			args:    args{ctx: context.Background(), req: &GetReq{Uuid: "notfound_id"}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "local decrypt",
			args: args{ctx: context.Background(), req: &GetReq{Uuid: "uuid-01", Password: password, CloudDecryption: false}},
			want: &GetResp{
				Body: Body{Uuid: "uuid-01"},
				Cookie: func() Cookie {
					ck := database["uuid-01"]
					ck.UpdateTime = now
					return ck
				}(),
			},
			wantErr: false,
		},
		{
			name: "server decrypt",
			args: args{ctx: context.Background(), req: &GetReq{Uuid: "uuid-01", Password: password, CloudDecryption: true}},
			want: &GetResp{
				Body: Body{Uuid: "", Encrypted: ""},
				Cookie: func() Cookie {
					ck := database["uuid-01"]
					ck.UpdateTime = now
					return ck
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.Get(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want != nil {
				if !assert.Equal(t, tt.want.Cookie, got.Cookie) {
					t.Errorf("Cookie not equal Get() got = %v, want = %v", got.Cookie, tt.want.Cookie)
					return
				}
				if !assert.Equal(t, tt.want.Uuid, got.Uuid) {
					t.Errorf("uuid not equal Get() got = %v, want = %v", got.Uuid, tt.want.Uuid)
					return
				}
			}
		})
	}
}

func TestClient_Push(t *testing.T) {
	now := time.Now().UTC()
	server := fakeServer(t, now)
	defer server.Close()

	cli, err := NewClient(&Config{
		ApiUrl: server.URL,
		Debug:  false,
	})
	if err != nil {
		t.Errorf("NewClient() error = %v", err)
		return
	}
	type args struct {
		ctx context.Context
		req *PushReq
	}
	tests := []struct {
		name    string
		args    args
		want    *PushResp
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				ctx: context.Background(),
				req: &PushReq{
					Password: password,
					Uuid:     "uuid-01",
					Cookie:   database["uuid-01"],
				},
			},
			want: &PushResp{
				Action: "done",
			},
			wantErr: false,
		},
		{
			name: "required uid",
			args: args{
				ctx: context.Background(),
				req: &PushReq{
					Password: password,
					Cookie:   database["uuid-01"],
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "required password",
			args: args{
				ctx: context.Background(),
				req: &PushReq{
					Uuid:   "uuid-01",
					Cookie: database["uuid-01"],
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.Push(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Push() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Push() got = %v, want %v", got, tt.want)
			}
		})
	}
}
