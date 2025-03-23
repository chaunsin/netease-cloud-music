package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
)

type DjRadioSub struct {
	TargetUserId string `json:"targetUserId"` // 用户id
	Limit        string `json:"limit"`
}

type DjRadioSubResp struct {
	Count    int64 `json:"count"` // 总条数
	DjRadios []struct {
		Dj struct {
			DefaultAvatar       bool        `json:"defaultAvatar"`
			Province            int64       `json:"province"`
			AuthStatus          int64       `json:"authStatus"`
			Followed            bool        `json:"followed"`
			AvatarUrl           string      `json:"avatarUrl"`
			AccountStatus       int64       `json:"accountStatus"`
			Gender              int64       `json:"gender"`
			City                int64       `json:"city"`
			Birthday            int64       `json:"birthday"`
			UserId              int64       `json:"userId"`
			UserType            int64       `json:"userType"`
			Nickname            string      `json:"nickname"`
			Signature           string      `json:"signature"`
			Description         string      `json:"description"`
			DetailDescription   string      `json:"detailDescription"`
			AvatarImgId         int64       `json:"avatarImgId"`
			BackgroundImgId     int64       `json:"backgroundImgId"`
			BackgroundUrl       string      `json:"backgroundUrl"`
			Authority           int64       `json:"authority"`
			Mutual              bool        `json:"mutual"`
			ExpertTags          interface{} `json:"expertTags"`
			Experts             interface{} `json:"experts"`
			DjStatus            int64       `json:"djStatus"`
			VipType             int64       `json:"vipType"`
			RemarkName          interface{} `json:"remarkName"`
			AuthenticationTypes int64       `json:"authenticationTypes"`
			AvatarDetail        interface{} `json:"avatarDetail"`
			AvatarImgIdStr      string      `json:"avatarImgIdStr"`
			BackgroundImgIdStr  string      `json:"backgroundImgIdStr"`
			Anchor              bool        `json:"anchor"`
			AvatarImgIdStr1     string      `json:"avatarImgId_str"`
		} `json:"dj"`
		Category        string      `json:"category"`
		SecondCategory  string      `json:"secondCategory"`
		Buyed           bool        `json:"buyed"`
		Price           int64       `json:"price"`
		OriginalPrice   int64       `json:"originalPrice"`
		DiscountPrice   interface{} `json:"discountPrice"`
		PurchaseCount   int64       `json:"purchaseCount"`
		LastProgramName string      `json:"lastProgramName"`
		Videos          interface{} `json:"videos"`
		Finished        bool        `json:"finished"`
		UnderShelf      bool        `json:"underShelf"`
		LiveInfo        interface{} `json:"liveInfo"`
		PlayCount       int64       `json:"playCount"`
		Privacy         bool        `json:"privacy"`
		Icon            interface{} `json:"icon"`
		ManualTagsDTO   interface{} `json:"manualTagsDTO"`
		DescPicList     []struct {
			Type       int64       `json:"type"`
			Id         int64       `json:"id"`
			Content    string      `json:"content"`
			Height     *int        `json:"height"`
			Width      *int        `json:"width"`
			TimeStamp  interface{} `json:"timeStamp"`
			NestedData interface{} `json:"nestedData"`
		} `json:"descPicList"`
		ReplaceRadioId        int64         `json:"replaceRadioId"`
		ReplaceRadio          interface{}   `json:"replaceRadio"`
		PicUrl                string        `json:"picUrl"`
		ShortName             interface{}   `json:"shortName"`
		FeeScope              int64         `json:"feeScope"`
		LastProgramId         int64         `json:"lastProgramId"`
		IntervenePicUrl       string        `json:"intervenePicUrl"`
		LastProgramCreateTime int64         `json:"lastProgramCreateTime"`
		RadioFeeType          int64         `json:"radioFeeType"`
		PicId                 int64         `json:"picId"`
		CategoryId            int64         `json:"categoryId"`
		TaskId                int64         `json:"taskId"`
		ProgramCount          int64         `json:"programCount"`
		SubCount              int64         `json:"subCount"`
		ParticipateUidList    []interface{} `json:"participateUidList"`
		OperateUidList        []interface{} `json:"operateUidList"`
		IntervenePicId        int64         `json:"intervenePicId"`
		Dynamic               bool          `json:"dynamic"`
		Name                  string        `json:"name"`
		Id                    int64         `json:"id"`
		Desc                  string        `json:"desc"`
		CreateTime            int64         `json:"createTime"`
		Rcmdtext              *string       `json:"rcmdtext"`
		NewProgramCount       int64         `json:"newProgramCount"`
	} `json:"djRadios"`
	Time    int64 `json:"time"` // eg:1625317200000
	HasMore bool  `json:"hasMore"`
	Code    int64 `json:"code"` // 200: success
}

// DjRadioSub 获取订阅博客列表
// har: 34.har
func (a *Api) DjRadioSub(ctx context.Context, req *DjRadioSub) (*DjRadioSubResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/djradio/get/subed"
		reply DjRadioSubResp
		opts  = api.NewOptions()
	)
	if req.Limit == "" {
		req.Limit = "1000"
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}
