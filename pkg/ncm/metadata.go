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

package ncm

import "encoding/json"

type Artist struct {
	Name string
	Id   int64
}

func (a *Artist) UnmarshalJSON(data []byte) error {
	var v []interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	a.Name = v[0].(string)
	a.Id = int64(v[1].(float64))
	return nil
}

// AlbumPicDocId 解决有的歌曲id为int有的歌曲id为string问题
type AlbumPicDocId string

func (a *AlbumPicDocId) UnmarshalJSON(data []byte) error {
	*a = AlbumPicDocId(data)
	return nil
}

// MetadataMusic .
type MetadataMusic struct {
	Id            int64         `json:"musicId"`
	Name          string        `json:"musicName"`
	Artists       []Artist      `json:"artist"`
	AlbumId       int64         `json:"albumId"`
	Album         string        `json:"album"`
	AlbumPic      string        `json:"albumPic"`      // eg: https://p4.music.126.net/cg5ZWookeo8bBHlwp906_Q==/109951165709257937.jpg
	AlbumPicDocId AlbumPicDocId `json:"albumPicDocId"` // eg: 109951165709257937
	BitRate       int64         `json:"bitrate"`       //
	Mp3DocId      string        `json:"mp3DocId"`      // eg: 7caa09bd32c62d0f415e45c0eec3da43
	MvId          int64         `json:"mvId"`
	Alias         []interface{} `json:"alias"`
	TransNames    []interface{} `json:"transNames"`
	Duration      int64         `json:"duration"` // 单位毫秒
	Format        string        `json:"format"`   // eg: flac

	Comment string `json:"-"` // 为了方便放到此处，此字段不属于ncm内容
}

type MetadataDJ struct {
	ProgramID          int64         `json:"programId"`
	ProgramName        string        `json:"programName"`
	MainMusic          MetadataMusic `json:"mainMusic"`
	DjID               int64         `json:"djId"`
	DjName             string        `json:"djName"`
	DjAvatarURL        string        `json:"djAvatarUrl"`
	CreateTime         int64         `json:"createTime"`
	Brand              string        `json:"brand"`
	Serial             int64         `json:"serial"`
	ProgramDesc        string        `json:"programDesc"`
	ProgramFeeType     int64         `json:"programFeeType"`
	ProgramBuyed       bool          `json:"programBuyed"`
	RadioID            int64         `json:"radioId"`
	RadioName          string        `json:"radioName"`
	RadioCategory      string        `json:"radioCategory"`
	RadioCategoryID    int64         `json:"radioCategoryId"`
	RadioDesc          string        `json:"radioDesc"`
	RadioFeeType       int64         `json:"radioFeeType"`
	RadioFeeScope      int64         `json:"radioFeeScope"`
	RadioBuyed         bool          `json:"radioBuyed"`
	RadioPrice         int64         `json:"radioPrice"`
	RadioPurchaseCount int64         `json:"radioPurchaseCount"`
}

type MetadataType string

const (
	MetadataTypeMusic MetadataType = "music"
	MetadataTypeDJ    MetadataType = "dj"
)

type Metadata struct {
	mt    MetadataType
	music *MetadataMusic
	dj    *MetadataDJ
}

func (m *Metadata) GetType() MetadataType {
	return m.mt
}

func (m *Metadata) GetMusic() *MetadataMusic {
	return m.music
}

func (m *Metadata) GetDJ() *MetadataDJ {
	return m.dj
}
