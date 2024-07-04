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

package types

// Level 音乐品质
type Level string

const (
	// LevelStandard 标准品质 128000
	LevelStandard Level = "standard"
	// LevelHigher 较高品质 192000
	LevelHigher Level = "higher"
	// LevelExhigh 极高品质 320000
	LevelExhigh Level = "exhigh"
	// LevelLossless 无损品质
	LevelLossless Level = "lossless"
	// LevelHires Hi-Res品质
	LevelHires Level = "hires"
	// LevelJyeffect 高清环绕声品质
	LevelJyeffect Level = "jyeffect"
	// LevelSky 沉浸环绕声品质
	LevelSky Level = "sky"
	// LevelJymaster 超清母带品质
	LevelJymaster Level = "jymaster"
)

var LevelString = map[Level]string{
	LevelStandard: "标准品质",
	LevelHigher:   "较高品质",
	LevelExhigh:   "极高品质",
	LevelLossless: "无损品质",
	LevelHires:    "Hi-Res品质",
	LevelJyeffect: "高清环绕声品质",
	LevelSky:      "沉浸环绕声品质",
	LevelJymaster: "超清母带品质",
}

type Quality struct {
	// Br(Bit Rate) 码率
	Br int `json:"br"`
	// Fid 貌似是对应网易云存储中得文件ID
	Fid int `json:"fid"`
	// Size 文件大小
	Size int `json:"size"`
	// Vd(Volume Delta) 音量增量
	Vd float64 `json:"vd"`
	// Sr(Sample Rate) 采样率
	Sr int `json:"sr"`
}
