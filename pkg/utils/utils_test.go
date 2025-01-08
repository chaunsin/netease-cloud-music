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
	"fmt"
	"os"
	"testing"
	"time"

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
	// var filename = "../../testdata/music/record1.m4a"
	var filename = "../../testdata/music/Maroon 5 - Animals.flac"
	file, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	md5, err := MD5Hex(file)
	assert.NoError(t, err)
	t.Logf("md5:%s", md5)
	assert.Equal(t, "afc48be2ca7c8afc38fbcb67ed7ff610", md5)
}

func TestSplitSlice(t *testing.T) {
	type args[T any] struct {
		input     []T
		chunkSize int
	}
	type testCase[T any] struct {
		name    string
		args    args[T]
		want    [][]T
		wantErr assert.ErrorAssertionFunc
	}
	tests := []testCase[int64]{
		{
			name: "ok",
			args: args[int64]{input: []int64{1, 2, 3, 4, 5, 6, 7, 8}, chunkSize: 3},
			want: [][]int64{{1, 2, 3}, {4, 5, 6}, {7, 8}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					return false
				}
				return true
			},
		},
		{
			name: "chunk>len",
			args: args[int64]{input: []int64{1, 2, 3}, chunkSize: 4},
			want: [][]int64{{1, 2, 3}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					return false
				}
				return true
			},
		},
		{
			name: "len<=0",
			args: args[int64]{input: []int64{1, 2, 3}, chunkSize: 0},
			want: [][]int64{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitSlice(tt.args.input, tt.args.chunkSize)
			if !tt.wantErr(t, err, fmt.Sprintf("SplitSlice(%v, %v)", tt.args.input, tt.args.chunkSize)) {
				return
			}
			assert.Equalf(t, tt.want, got, "SplitSlice(%v, %v)", tt.args.input, tt.args.chunkSize)
		})
	}
}

func calculateTime(t *testing.T, zone string) int64 {
	if zone == "" {
		zone = "Local"
	}
	l, err := time.LoadLocation(zone)
	assert.NoError(t, err)
	now := time.Now().In(l)
	will := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, l)
	return int64(will.Sub(now).Seconds())
}

func TestTimeUntilMidnight(t *testing.T) {
	type args struct {
		timeZone string
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "cst",
			args: args{timeZone: "Asia/Shanghai"},
			want: calculateTime(t, "Asia/Shanghai"), // 由于时间获取在方法内部，此处构造的时间和待测试得基本雷同,允许时间误差在1秒之内。
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					return false
				}
				return true
			},
		},
		{
			name: "local",
			args: args{timeZone: ""},
			want: calculateTime(t, ""), // 由于时间获取在方法内部，此处构造的时间和待测试得基本雷同,允许时间误差在1秒之内。
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					return false
				}
				return true
			},
		},
		{
			name: "UTC",
			args: args{timeZone: "UTC"},
			want: calculateTime(t, "UTC"), // 由于时间获取在方法内部，此处构造的时间和待测试得基本雷同,允许时间误差在1秒之内。
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TimeUntilMidnight(tt.args.timeZone)
			if !tt.wantErr(t, err, fmt.Sprintf("TimeUntilMidnight(%v)", tt.args.timeZone)) {
				return
			}
			assert.Equalf(t, tt.want, int64(got.Seconds()), "TimeUntilMidnight(%v)", tt.args.timeZone)
		})
	}
}

func TestFilename(t *testing.T) {
	type args struct {
		path string
		new  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "/",
			args: args{
				path: "path//to/file",
				new:  "_",
			},
			want: "path__to_file",
		},
		{
			name: "\\",
			args: args{
				path: "path\\\\to\\file",
				new:  "_",
			},
			want: "path__to_file",
		},
		{
			name: ":",
			args: args{
				path: "path:to::file",
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: "*",
			args: args{
				path: "path*to**file",
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: "?",
			args: args{
				path: "path?to??file",
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: "\"",
			args: args{
				path: `path"to""file`,
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: "<",
			args: args{
				path: "path<to<<file",
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: ">",
			args: args{
				path: "path>to>>file",
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: "|",
			args: args{
				path: "path|to||file",
				new:  "_",
			},
			want: "path_to__file",
		},
		{
			name: "replace empty",
			args: args{
				path: "path|to||file",
				new:  "",
			},
			want: "pathtofile",
		},
		{
			name: "empty1",
			args: args{
				path: "",
				new:  "_",
			},
			want: "",
		},
		{
			name: "empty2",
			args: args{
				path: "",
				new:  "",
			},
			want: "",
		},
		{
			name: "",
			args: args{
				path: "Empty string",
				new:  "",
			},
			want: "Empty string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, Filename(tt.args.path, tt.args.new), "Filename(%v, %v)", tt.args.path, tt.args.new)
		})
	}
}
