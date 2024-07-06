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

// 音质从低到高排序
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
	// LevelJyeffect 高清环绕声品质/高清臻音
	LevelJyeffect Level = "jyeffect"
	// LevelSky 沉浸环绕声品质
	LevelSky Level = "sky"
	// LevelJymaster 超清母带品质
	LevelJymaster Level = "jymaster"
	// LevelDolby 杜比 暂时未知
	// LevelDolby Level = ""
)

var LevelString = map[Level]string{
	LevelStandard: "标准",
	LevelHigher:   "较高",
	LevelExhigh:   "极高(HQ)",
	LevelLossless: "无损(SQ)",
	LevelHires:    "高解析度无损(Hi-Res)",
	LevelJyeffect: "高清臻音(Spatial Audio)",
	LevelSky:      "沉浸环绕声品质(Surround Audio)",
	LevelJymaster: "超清母带品质(Master)",
	// LevelDolby: "杜比全景声(Dolby Atmos)",
}

type Quality struct {
	// Br(Bit Rate) 码率
	Br int64 `json:"br"`
	// Fid 貌似是对应网易云存储中得文件ID
	Fid int64 `json:"fid"`
	// Size 文件大小
	Size int64 `json:"size"`
	// Vd(Volume Delta) 音量增量
	Vd float64 `json:"vd"`
	// Sr(Sample Rate) 采样率
	Sr int64 `json:"sr"`
}

type Qualities struct {
	// L 标准品质
	L *Quality `json:"l"`
	// M 高品质音质,通常客户端好像看不到这个音质了目前
	M *Quality `json:"m"`
	// H 极高品质
	H *Quality `json:"h"`
	// Sq 无损品质
	Sq *Quality `json:"sq"`
	// Hr Hi-Res品质
	Hr *Quality `json:"hr"`
	// Je 高清环绕声品质
	Je *Quality `json:"je"`
	// Sk 沉浸环绕声品质
	Sk *Quality `json:"sk"`
	// Jm 超清母带品质
	Jm *Quality `json:"jm"`
	// 杜比目前未知
	// Dl *Quality `json:""`
}

// FindBetter 根据指定l获取音质信息,如果找到则返回对应级别得音乐信息并返回true，
// 如果找不到则降级返回最接近得音质信息，并返回false
func (q Qualities) FindBetter(l Level) (*Quality, Level, bool) {
	var match = true
	switch l {
	case LevelJymaster:
		if q.M != nil {
			return q.M, LevelJyeffect, true
		}
		match = false
		fallthrough
	case LevelSky:
		if q.Sk != nil {
			return q.Sk, LevelSky, match
		}
		match = false
		fallthrough
	case LevelJyeffect:
		if q.Je != nil {
			return q.Je, LevelJyeffect, match
		}
		match = false
		fallthrough
	case LevelHires:
		if q.Hr != nil {
			return q.Hr, LevelHires, match
		}
		match = false
		fallthrough
	case LevelLossless:
		if q.Sq != nil {
			return q.Sq, LevelLossless, match
		}
		match = false
		fallthrough
	case LevelExhigh:
		if q.H != nil {
			return q.H, LevelExhigh, match
		}
		match = false
		fallthrough
	case LevelHigher:
		if q.M != nil {
			return q.M, LevelHigher, match
		}
		match = false
		fallthrough
	case LevelStandard:
		if q.L != nil {
			return q.L, LevelStandard, match
		}
		fallthrough
	default:
		return q.L, LevelStandard, false
	}
}
