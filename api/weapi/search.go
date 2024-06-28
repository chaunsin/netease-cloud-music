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

package weapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

type SearchDefaultReq struct{}

type SearchDefaultResp struct {
	types.RespCommon[SearchDefaultRespData]
}

type SearchDefaultRespData struct {
	RefreshTime int `json:"refreshTime"`
	Keywords    []struct {
		Action       int         `json:"action"`
		Alg          string      `json:"alg"`
		BizQueryInfo string      `json:"bizQueryInfo"`
		Gap          int         `json:"gap"`
		ImageUrl     interface{} `json:"imageUrl"`
		LogInfo      interface{} `json:"logInfo"`
		Realkeyword  string      `json:"realkeyword"`
		SearchType   int         `json:"searchType"`
		ShowKeyword  string      `json:"showKeyword"`
		Source       interface{} `json:"source"`
		StyleKeyword struct {
			DescWord *string `json:"descWord"`
			KeyWord  string  `json:"keyWord"`
		} `json:"styleKeyword"`
		TrpId   interface{} `json:"trp_id"`
		TrpType interface{} `json:"trp_type"`
	} `json:"keywords"`
}

// SearchDefault 首页搜索输入框默认搜索关键词
// url: testdata/har/11.har
// needLogin: 未知
func (a *Api) SearchDefault(ctx context.Context, req *SearchDefaultReq) (*SearchDefaultResp, error) {
	var (
		url   = "https://interface.music.163.com/eapi/search/default/keyword/get"
		reply SearchDefaultResp
	)

	resp, err := a.client.Request(ctx, http.MethodPost, url, "eapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
