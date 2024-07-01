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
	Id   float64
}

func (a *Artist) UnmarshalJSON(data []byte) error {
	var v []interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	a.Name = v[0].(string)
	a.Id = v[1].(float64)
	return nil
}

// Metadata .
type Metadata struct {
	Id            int64         `json:"musicId"`
	Name          string        `json:"musicName"`
	Artists       []Artist      `json:"artist"`
	AlbumId       int64         `json:"albumId"`
	Album         string        `json:"album"`
	AlbumPic      string        `json:"albumPic"`      // eg: https://p4.music.126.net/cg5ZWookeo8bBHlwp906_Q==/109951165709257937.jpg
	AlbumPicDocId string        `json:"albumPicDocId"` // eg: 109951165709257937
	BitRate       int64         `json:"bitrate"`       //
	Mp3DocId      string        `json:"mp3DocId"`      // eg: 7caa09bd32c62d0f415e45c0eec3da43
	MvId          int64         `json:"mvId"`
	Alias         []interface{} `json:"alias"`
	TransNames    []interface{} `json:"transNames"`
	Duration      int64         `json:"duration"` // 单位毫秒
	Format        string        `json:"format"`   // eg: flac
}
