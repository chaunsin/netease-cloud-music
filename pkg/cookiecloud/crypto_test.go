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
	"hash"
	"reflect"
	"testing"
)

func TestDecrypt(t *testing.T) {
	type args struct {
		password   string
		ciphertext string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "ok",
			args:    args{"kTduz4A61D4a9LwS7jGKRu", "U2FsdGVkX18WZOw9PJ32j1zkgpfswFuHXQZxyq/01QE="},
			want:    "cookies value",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.args.password, tt.args.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("Decrypt() got = [%s], want [%v]", got, tt.want)
			}
		})
	}
}

func TestEncrypt(t *testing.T) {
	type args struct {
		uuid     string
		password string
		data     string
	}
	tests := []struct {
		name    string
		args    args
		want    int // length of encrypted string
		wantErr bool
	}{
		{
			name:    "ok",
			args:    args{password: "kTduz4A61D4a9LwS7jGKRu", data: "cookies value"},
			want:    44,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encrypt(tt.args.password, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncryptCookieData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("got = %v", got)
			if len(got) != tt.want {
				t.Errorf("EncryptCookieData() got = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestBytesToKey(t *testing.T) {
	type args struct {
		salt     []byte
		data     []byte
		h        hash.Hash
		keyLen   int
		blockLen int
	}
	tests := []struct {
		name    string
		args    args
		wantKey []byte
		wantIv  []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotIv, err := BytesToKey(tt.args.salt, tt.args.data, tt.args.h, tt.args.keyLen, tt.args.blockLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("BytesToKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotKey, tt.wantKey) {
				t.Errorf("BytesToKey() gotKey = %v, want %v", gotKey, tt.wantKey)
			}
			if !reflect.DeepEqual(gotIv, tt.wantIv) {
				t.Errorf("BytesToKey() gotIv = %v, want %v", gotIv, tt.wantIv)
			}
		})
	}
}

func TestBytesToKeyAES256CBC(t *testing.T) {
	type args struct {
		salt []byte
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		wantKey []byte
		wantIv  []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotIv, err := BytesToKeyAES256CBC(tt.args.salt, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("BytesToKeyAES256CBC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotKey, tt.wantKey) {
				t.Errorf("BytesToKeyAES256CBC() gotKey = %v, want %v", gotKey, tt.wantKey)
			}
			if !reflect.DeepEqual(gotIv, tt.wantIv) {
				t.Errorf("BytesToKeyAES256CBC() gotIv = %v, want %v", gotIv, tt.wantIv)
			}
		})
	}
}

func TestBytesToKeyAES256CBCMD5(t *testing.T) {
	type args struct {
		salt []byte
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		wantKey []byte
		wantIv  []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotIv, err := BytesToKeyAES256CBCMD5(tt.args.salt, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("BytesToKeyAES256CBCMD5() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotKey, tt.wantKey) {
				t.Errorf("BytesToKeyAES256CBCMD5() gotKey = %v, want %v", gotKey, tt.wantKey)
			}
			if !reflect.DeepEqual(gotIv, tt.wantIv) {
				t.Errorf("BytesToKeyAES256CBCMD5() gotIv = %v, want %v", gotIv, tt.wantIv)
			}
		})
	}
}

func TestMd5String(t *testing.T) {
	type args struct {
		inputs []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Md5String(tt.args.inputs...); got != tt.want {
				t.Errorf("Md5String() = %v, want %v", got, tt.want)
			}
		})
	}
}
