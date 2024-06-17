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

package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseBytes(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{
			name:    "empty",
			args:    args{input: ""},
			want:    0,
			wantErr: false,
		},
		{
			name:    "default",
			args:    args{input: "1"},
			want:    B,
			wantErr: false,
		},
		{
			name:    "b",
			args:    args{input: "1024"},
			want:    1024,
			wantErr: false,
		},
		{
			name:    "B",
			args:    args{input: "1024"},
			want:    1024,
			wantErr: false,
		},
		{
			name:    "k",
			args:    args{input: "1k"},
			want:    KB,
			wantErr: false,
		},
		{
			name:    "K",
			args:    args{input: "1K"},
			want:    KB,
			wantErr: false,
		},
		{
			name:    "kB",
			args:    args{input: "1kB"},
			want:    KB,
			wantErr: false,
		},
		{
			name:    "Kb",
			args:    args{input: "1Kb"},
			want:    KB,
			wantErr: false,
		},
		{
			name:    "KB",
			args:    args{input: "1KB"},
			want:    KB,
			wantErr: false,
		},
		{
			name:    "M",
			args:    args{input: "1M"},
			want:    MB,
			wantErr: false,
		},
		{
			name:    "Mb",
			args:    args{input: "1Mb"},
			want:    MB,
			wantErr: false,
		},
		{
			name:    "mB",
			args:    args{input: "1mB"},
			want:    MB,
			wantErr: false,
		},
		{
			name:    "MB",
			args:    args{input: "1MB"},
			want:    MB,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBytes(tt.args.input)
			assert.Equal(t, tt.wantErr, err != nil, "ParseBytes(%v)", tt.args.input)
			assert.Equalf(t, tt.want, got, "ParseBytes(%v)", tt.args.input)
		})
	}
}

func TestMd5Hex(t *testing.T) {
	var filename = "../../testdata/music/record1.m4a"
	file, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	md5, err := MD5Hex(file)
	assert.NoError(t, err)
	t.Logf("md5:%s", md5)
}
