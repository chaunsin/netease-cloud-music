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
	A  interface{} `json:"a"`
	Al struct {
		Id     int    `json:"id"`
		Name   string `json:"name"`
		Pic    int64  `json:"pic"`
		PicUrl string `json:"picUrl"`
		PicStr string `json:"pic_str"`
	} `json:"al"`
	Alia []interface{} `json:"alia"`
	Ar   []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"ar"`
	Cd              string         `json:"cd"`
	Cf              string         `json:"cf"`
	Cp              int            `json:"cp"`
	Crbt            interface{}    `json:"crbt"`
	DjId            int            `json:"djId"`
	Dt              int            `json:"dt"`
	Fee             int            `json:"fee"`
	Ftype           int            `json:"ftype"`
	H               *types.Quality `json:"h"`
	Hr              *types.Quality `json:"hr"`
	Id              int64          `json:"id"`
	L               *types.Quality `json:"l"`
	M               *types.Quality `json:"m"`
	Mst             int            `json:"mst"`
	Mv              int            `json:"mv"`
	Name            string         `json:"name"`
	No              int            `json:"no"`
	NoCopyrightRcmd interface{}    `json:"noCopyrightRcmd"`
	Pop             float64        `json:"pop"`
	Privilege       struct {
		ChargeInfoList []struct {
			ChargeMessage interface{} `json:"chargeMessage"`
			ChargeType    int         `json:"chargeType"`
			ChargeUrl     interface{} `json:"chargeUrl"`
			Rate          int         `json:"rate"`
		} `json:"chargeInfoList"`
		Code               int    `json:"code"`
		Cp                 int    `json:"cp"`
		Cs                 bool   `json:"cs"`
		Dl                 int    `json:"dl"`
		DlLevel            string `json:"dlLevel"`
		DownloadMaxBrLevel string `json:"downloadMaxBrLevel"`
		DownloadMaxbr      int    `json:"downloadMaxbr"`
		Fee                int    `json:"fee"`
		Fl                 int    `json:"fl"`
		FlLevel            string `json:"flLevel"`
		Flag               int    `json:"flag"`
		FreeTrialPrivilege struct {
			CannotListenReason interface{} `json:"cannotListenReason"`
			ListenType         interface{} `json:"listenType"`
			PlayReason         interface{} `json:"playReason"`
			ResConsumable      bool        `json:"resConsumable"`
			UserConsumable     bool        `json:"userConsumable"`
		} `json:"freeTrialPrivilege"`
		Id             int64       `json:"id"`
		MaxBrLevel     string      `json:"maxBrLevel"`
		Maxbr          int         `json:"maxbr"`
		Message        interface{} `json:"message"`
		Payed          int         `json:"payed"`
		Pl             int         `json:"pl"`
		PlLevel        string      `json:"plLevel"`
		PlayMaxBrLevel string      `json:"playMaxBrLevel"`
		PlayMaxbr      int         `json:"playMaxbr"`
		PreSell        bool        `json:"preSell"`
		RightSource    int         `json:"rightSource"`
		Rscl           interface{} `json:"rscl"`
		Sp             int         `json:"sp"`
		St             int         `json:"st"`
		Subp           int         `json:"subp"`
		Toast          bool        `json:"toast"`
	} `json:"privilege"`
	Pst          int            `json:"pst"`
	Rt           string         `json:"rt"`
	RtUrl        interface{}    `json:"rtUrl"`
	RtUrls       []interface{}  `json:"rtUrls"`
	Rtype        int            `json:"rtype"`
	Rurl         interface{}    `json:"rurl"`
	SongJumpInfo interface{}    `json:"songJumpInfo"`
	Sq           *types.Quality `json:"sq"`
	St           int            `json:"st"`
	T            int            `json:"t"`
	V            int            `json:"v"`
}

type AlbumRespAlbum struct {
	Alias  []interface{} `json:"alias"`
	Artist struct {
		AlbumSize   int           `json:"albumSize"`
		Alias       []interface{} `json:"alias"`
		BriefDesc   string        `json:"briefDesc"`
		Followed    bool          `json:"followed"`
		Id          int           `json:"id"`
		Img1V1Id    int64         `json:"img1v1Id"`
		Img1V1IdStr string        `json:"img1v1Id_str"`
		Img1V1Url   string        `json:"img1v1Url"`
		MusicSize   int           `json:"musicSize"`
		Name        string        `json:"name"`
		PicId       int64         `json:"picId"`
		PicIdStr    string        `json:"picId_str"`
		PicUrl      string        `json:"picUrl"`
		TopicPerson int           `json:"topicPerson"`
		Trans       string        `json:"trans"`
	} `json:"artist"`
	Artists []struct {
		AlbumSize   int           `json:"albumSize"`
		Alias       []interface{} `json:"alias"`
		BriefDesc   string        `json:"briefDesc"`
		Followed    bool          `json:"followed"`
		Id          int           `json:"id"`
		Img1V1Id    int64         `json:"img1v1Id"`
		Img1V1IdStr string        `json:"img1v1Id_str"`
		Img1V1Url   string        `json:"img1v1Url"`
		MusicSize   int           `json:"musicSize"`
		Name        string        `json:"name"`
		PicId       int           `json:"picId"`
		PicUrl      string        `json:"picUrl"`
		TopicPerson int           `json:"topicPerson"`
		Trans       string        `json:"trans"`
	} `json:"artists"`
	AwardTags       interface{} `json:"awardTags"`
	BlurPicUrl      string      `json:"blurPicUrl"`
	BriefDesc       interface{} `json:"briefDesc"`
	CommentThreadId string      `json:"commentThreadId"`
	Company         string      `json:"company"`
	CompanyId       int         `json:"companyId"`
	CopyrightId     int         `json:"copyrightId"`
	Description     string      `json:"description"`
	Id              int         `json:"id"`
	Info            struct {
		CommentCount  int `json:"commentCount"`
		CommentThread struct {
			CommentCount     int         `json:"commentCount"`
			HotCount         int         `json:"hotCount"`
			Id               string      `json:"id"`
			LatestLikedUsers interface{} `json:"latestLikedUsers"`
			LikedCount       int         `json:"likedCount"`
			ResourceId       int         `json:"resourceId"`
			ResourceInfo     struct {
				Creator   interface{} `json:"creator"`
				EncodedId interface{} `json:"encodedId"`
				Id        int         `json:"id"`
				ImgUrl    string      `json:"imgUrl"`
				Name      string      `json:"name"`
				SubTitle  interface{} `json:"subTitle"`
				UserId    int         `json:"userId"`
				WebUrl    interface{} `json:"webUrl"`
			} `json:"resourceInfo"`
			ResourceOwnerId int    `json:"resourceOwnerId"`
			ResourceTitle   string `json:"resourceTitle"`
			ResourceType    int    `json:"resourceType"`
			ShareCount      int    `json:"shareCount"`
		} `json:"commentThread"`
		Comments         interface{} `json:"comments"`
		LatestLikedUsers interface{} `json:"latestLikedUsers"`
		Liked            bool        `json:"liked"`
		LikedCount       int         `json:"likedCount"`
		ResourceId       int         `json:"resourceId"`
		ResourceType     int         `json:"resourceType"`
		ShareCount       int         `json:"shareCount"`
		ThreadId         string      `json:"threadId"`
	} `json:"info"`
	Mark        int           `json:"mark"`
	Name        string        `json:"name"`
	OnSale      bool          `json:"onSale"`
	Paid        bool          `json:"paid"`
	Pic         int64         `json:"pic"`
	PicId       int64         `json:"picId"`
	PicIdStr    string        `json:"picId_str"`
	PicUrl      string        `json:"picUrl"`
	PublishTime int64         `json:"publishTime"`
	Size        int           `json:"size"`
	Songs       []interface{} `json:"songs"`
	Status      int           `json:"status"`
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
	)
	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
