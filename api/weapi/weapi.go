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
	"net/url"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type Api struct {
	client *api.Client
}

func New(client *api.Client) *Api {
	a := Api{client: client}
	return &a
}

func (a *Api) NeedLogin(ctx context.Context) bool {
	u, _ := url.Parse("https://music.163.com")
	for _, ck := range a.client.GetClient().Jar.Cookies(u) {
		// 判断用户是否有登录信息,如果有登录信息,还需要调用接口进行判断,单纯的判断cookie过期时间是不行的
		if ck.Name == "MUSIC_U" && ck.Expires.Before(time.Now()) {
			reply, err := a.GetUserInfo(ctx, &GetUserInfoReq{})
			if err != nil {
				return true
			}
			log.Debug("NeedLogin: %+v", reply)
			if reply.Code != 200 || reply.Account == nil || reply.Profile == nil {
				return true
			}
			return false
		}
	}
	return true
}
