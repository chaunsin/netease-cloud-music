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

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type AlbumReq struct {
	Id string `json:"id"` // 专辑id
}

type AlbumResp struct {
	types.RespCommon[any]
	ResourceState bool             `json:"resourceState"`
	Songs         []AlbumRespSongs `json:"songs"`
	Album         AlbumRespAlbum   `json:"album"`
}

type AlbumRespSongs struct {
	Id              int64          `json:"id"`
	A               interface{}    `json:"a"`
	Al              types.Album    `json:"al"`
	Alia            []interface{}  `json:"alia"`
	Ar              []types.Artist `json:"ar"`
	Cd              string         `json:"cd"`
	Cf              string         `json:"cf"`
	Cp              int64          `json:"cp"`
	Crbt            interface{}    `json:"crbt"`
	DjId            int64          `json:"djId"`
	Dt              int64          `json:"dt"`
	Fee             int64          `json:"fee"`
	Ftype           int64          `json:"ftype"`
	H               *types.Quality `json:"h"`
	Hr              *types.Quality `json:"hr"`
	L               *types.Quality `json:"l"`
	M               *types.Quality `json:"m"`
	Sq              *types.Quality `json:"sq"`
	Mst             int64          `json:"mst"`
	Mv              int64          `json:"mv"`
	Name            string         `json:"name"`
	No              int64          `json:"no"`
	NoCopyrightRcmd interface{}    `json:"noCopyrightRcmd"`
	Pop             float64        `json:"pop"`
	Pst             int64          `json:"pst"`
	Rt              string         `json:"rt"`
	RtUrl           interface{}    `json:"rtUrl"`
	RtUrls          []interface{}  `json:"rtUrls"`
	Rtype           int64          `json:"rtype"`
	Rurl            interface{}    `json:"rurl"`
	SongJumpInfo    interface{}    `json:"songJumpInfo"`
	St              int64          `json:"st"`
	T               int64          `json:"t"`
	V               int64          `json:"v"`
	Privilege       struct {
		types.Privileges
		Code    int64       `json:"code"`
		Message interface{} `json:"message"`
	} `json:"privilege"`
}

type AlbumRespAlbumArtist struct {
	AlbumSize   int64         `json:"albumSize"`
	Alias       []interface{} `json:"alias"`
	BriefDesc   string        `json:"briefDesc"`
	Followed    bool          `json:"followed"`
	Id          int64         `json:"id"`
	Img1V1Id    int64         `json:"img1v1Id"`
	Img1V1IdStr string        `json:"img1v1Id_str"`
	Img1V1Url   string        `json:"img1v1Url"`
	MusicSize   int64         `json:"musicSize"`
	Name        string        `json:"name"`
	PicId       int64         `json:"picId"`
	PicIdStr    string        `json:"picId_str"`
	PicUrl      string        `json:"picUrl"`
	TopicPerson int64         `json:"topicPerson"`
	Trans       string        `json:"trans"`
}

type AlbumRespAlbum struct {
	Alias           []interface{}          `json:"alias"`
	Artist          AlbumRespAlbumArtist   `json:"artist"`
	Artists         []AlbumRespAlbumArtist `json:"artists"`
	AwardTags       interface{}            `json:"awardTags"`
	BlurPicUrl      string                 `json:"blurPicUrl"`
	BriefDesc       interface{}            `json:"briefDesc"`
	CommentThreadId string                 `json:"commentThreadId"`
	Company         string                 `json:"company"`
	CompanyId       int64                  `json:"companyId"`
	CopyrightId     int64                  `json:"copyrightId"`
	Description     string                 `json:"description"`
	Id              int64                  `json:"id"`
	Info            struct {
		CommentCount  int64 `json:"commentCount"`
		CommentThread struct {
			CommentCount     int64       `json:"commentCount"`
			HotCount         int64       `json:"hotCount"`
			Id               string      `json:"id"`
			LatestLikedUsers interface{} `json:"latestLikedUsers"`
			LikedCount       int64       `json:"likedCount"`
			ResourceId       int64       `json:"resourceId"`
			ResourceInfo     struct {
				Creator   interface{} `json:"creator"`
				EncodedId interface{} `json:"encodedId"`
				Id        int64       `json:"id"`
				ImgUrl    string      `json:"imgUrl"`
				Name      string      `json:"name"`
				SubTitle  interface{} `json:"subTitle"`
				UserId    int64       `json:"userId"`
				WebUrl    interface{} `json:"webUrl"`
			} `json:"resourceInfo"`
			ResourceOwnerId int64  `json:"resourceOwnerId"`
			ResourceTitle   string `json:"resourceTitle"`
			ResourceType    int64  `json:"resourceType"`
			ShareCount      int64  `json:"shareCount"`
		} `json:"commentThread"`
		Comments         interface{} `json:"comments"`
		LatestLikedUsers interface{} `json:"latestLikedUsers"`
		Liked            bool        `json:"liked"`
		LikedCount       int64       `json:"likedCount"`
		ResourceId       int64       `json:"resourceId"`
		ResourceType     int64       `json:"resourceType"`
		ShareCount       int64       `json:"shareCount"`
		ThreadId         string      `json:"threadId"`
	} `json:"info"`
	Mark        int64         `json:"mark"`
	Name        string        `json:"name"`
	OnSale      bool          `json:"onSale"`
	Paid        bool          `json:"paid"`
	Pic         int64         `json:"pic"`
	PicId       int64         `json:"picId"`
	PicIdStr    string        `json:"picId_str"`
	PicUrl      string        `json:"picUrl"`
	PublishTime int64         `json:"publishTime"`
	Size        int64         `json:"size"`
	Songs       []interface{} `json:"songs"`
	Status      int64         `json:"status"`
	SubType     string        `json:"subType"`
	Tags        string        `json:"tags"`
	Type        string        `json:"type"`
}

// Album 专辑内容
// url:
// needLogin:
func (a *Api) Album(ctx context.Context, req *AlbumReq) (*AlbumResp, error) {
	var (
		url   = fmt.Sprintf("https://music.163.com/weapi/v1/album/%v", req.Id)
		reply AlbumResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
