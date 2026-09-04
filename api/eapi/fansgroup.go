// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

// FansGroup (乐迷团) API — 乐迷团任务相关接口
// Endpoints:
//   - /api/fans/group/mission/all (获取乐迷团任务列表)
//   - /api/social/fansgroup/bff/detail/get (获取乐迷团详情含boardId)
//   - /api/social/fansgroup/bff/user/group/detail/get (获取用户所处乐迷团详情)
//   - /api/fans/group/feed/recommend/get (获取乐迷团推荐Feed)
//   - /api/fans/group/mission/forward/progress (分享进度上报)
//   - /api/resource/like (点赞资源)

package eapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

// FansGroupDetailGetReq 获取乐迷团详情请求.
type FansGroupDetailGetReq struct {
	types.EApiReqCommon

	GroupId string `json:"groupId"` // 乐迷团ID
	Scene   string `json:"scene"`   // 场景, 可留空
}

// FansGroupDetailGetResp 获取乐迷团详情响应.
type FansGroupDetailGetResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		FansGroupInfo struct {
			FansGroupId       string `json:"fansGroupId"`
			FansGroupName     string `json:"fansGroupName"`
			FansGroupPureName string `json:"fansGroupPureName"`
			HeadId            int64  `json:"headId"`        // 歌手/头像ID
			ArtistName        string `json:"artistName"`    // 歌手名
			BoardId           string `json:"boardId"`       // 看板ID = activityInfoList 中的 id
			TopicId           string `json:"topicId"`       // 话题ID
			HeadAvatarUrl     string `json:"headAvatarUrl"` // 头像URL
			Musician          bool   `json:"musician"`      // 是否音乐人
			// 以下为 2026-09-04 载荷实测补充字段 (其余 UI 向嵌套结构 kolUsers/liveDTO/
			// lotteryDTO/listenTogether 等暂未建模, 需要时再补)
			HeadIdType              string `json:"headIdType"`      // eg: "ARTIST_ID"
			HeadHomepageUrl         string `json:"headHomepageUrl"` // 歌手主页跳转链接
			FansNameplate           string `json:"fansNameplate"`   // 铭牌名 eg: "音乐合伙人"
			Hidden                  bool   `json:"hidden"`
			Brief                   string `json:"brief"`
			MainState               bool   `json:"mainState"`
			ShowFansSay             bool   `json:"showFansSay"`
			NewMembersThisWeekCount int64  `json:"newMembersThisWeekCount"` // 本周新增成员
			ActiveMembersCount      int64  `json:"activeMembersCount"`      // 活跃成员
			PlayedSongUvCount       int64  `json:"playedSongUvCount"`       // 播放歌曲UV
			FansNoteUvCount         int64  `json:"fansNoteUvCount"`         // 乐迷笔记UV
			TotalMembersCount       int64  `json:"totalMembersCount"`       // 成员总数
		} `json:"fansGroupInfo"`
		IsNewPage bool `json:"isNewPage"`
	} `json:"data"`
	Error bool `json:"error"`
}

// FansGroupDetailGet 获取乐迷团详情 (含 boardId 等关键信息).
func (a *Api) FansGroupDetailGet(ctx context.Context, req *FansGroupDetailGetReq) (*FansGroupDetailGetResp, error) {
	queryParams := "groupId=" + req.GroupId
	if req.Scene != "" {
		queryParams += "&scene=" + req.Scene
	}

	var (
		url   = "https://interface3.music.163.com/eapi/social/fansgroup/bff/detail/get?" + queryParams
		reply FansGroupDetailGetResp
		opts  = api.NewOptions("eapi.FansGroupDetailGet").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// FansGroupMissionAllReq 获取乐迷团全部任务列表请求.
type FansGroupMissionAllReq struct {
	types.EApiReqCommon

	FansGroupId string `json:"fansGroupId"` // 乐迷团ID
}

type FansGroupMissionAllRespDataNormalData struct {
	MissionId        int64  `json:"missionId"`
	MissionType      string `json:"missionType"`      // "normal" / "userSurprise"(貌似是加速任务类型)
	Title            string `json:"title"`            // 任务标题: 播放歌曲/发布图文笔记/分享歌曲/点赞乐迷笔记
	Subtitle         string `json:"subtitle"`         // eg: 评论ta的一首歌
	TitleForArchives string `json:"titleForArchives"` // eg: 今日加速任务
	Tags             []any  `json:"tags"`             //
	Note             any    `json:"note"`             //
	MissionDetail    any    `json:"missionDetail"`    // eg: {}
	Status           string `json:"status"`           // "INIT"=未开始 "PROCESSING"=进行中 "COMPLETED"=已完成
	CurrentProgress  int    `json:"currentProgress"`  // 当前进度 eg: 0
	AllProgress      int    `json:"allProgress"`      // 总进度 eg: 2
	DeadlineTime     int64  `json:"deadlineTime"`     // eg: 1788451199999
	Integral         string `json:"integral"`         // 奖励积分
	Order            int    `json:"order"`            // 排序 eg: 1
	LogInfo          string `json:"logInfo"`          // 日志信息JSON eg: "{\"missionId\":3899306,\"missionType\":\"normal\"}"
	IconUi           struct {
		IconUrl   string `json:"iconUrl"`   // eg: "http://p5.music.126.net/obj/wo7DlsKTwrbDjjjDjsOk/34804307132/d93a/88e5/b658/08ff0abcadff4c3e423f81aa456f3b03.png"
		TargetUrl string `json:"targetUrl"` // 包含任务参数的JSON eg: "{\"actionType\":\"mnb\",\"actionMnbName\":\"nm.play.playSongs\",\"actionMnbParams\":{\"songIndex\":0,\"songIds\":[3357688069,3372977536,3366130466,3395738149,3395784051,3372979876,3366125663,3399935435,3357361025,3372978254,3372978747,3366280099,3372978601,3338605600,3395222584,3384793624],\"playParams\":{\"playerType\":\"music\",\"showUI\":\"true\"}}}"
	} `json:"iconUi"`
	Button struct {
		Copywriter string `json:"copywriter"` // 按钮文案 eg: 去播放
		Url        string `json:"url"`        // 包含任务参数的JSON (与 TargetUrl 结构相同) eg: "{\"actionType\":\"mnb\",\"actionMnbName\":\"nm.play.playSongs\",\"actionMnbParams\":{\"songIndex\":0,\"songIds\":[3357688069,3372977536,3366130466,3395738149,3395784051,3372979876,3366125663,3399935435,3357361025,3372978254,3372978747,3366280099,3372978601,3338605600,3395222584,3384793624],\"playParams\":{\"playerType\":\"music\",\"showUI\":\"true\"}}}"
	} `json:"button"`
}

type FansGroupMissionAllRespDataNormal struct {
	Success bool                                    `json:"success"`
	Code    any                                     `json:"code,omitempty"`
	Message any                                     `json:"message,omitempty"`
	Ignore  bool                                    `json:"ignore"`
	Present bool                                    `json:"present"`
	Empty   bool                                    `json:"empty"`
	Data    []FansGroupMissionAllRespDataNormalData `json:"data"`
}

type FansGroupMissionAllRespData struct {
	Normal      FansGroupMissionAllRespDataNormal `json:"normal"` // 普通任务
	Originality struct {
		Success bool                                  `json:"success"`
		Code    any                                   `json:"code,omitempty"`
		Message any                                   `json:"message,omitempty"`
		Ignore  bool                                  `json:"ignore"`
		Present bool                                  `json:"present"`
		Empty   bool                                  `json:"empty"`
		Data    FansGroupMissionAllRespDataNormalData `json:"data"`
	} `json:"originality"` // 加速任务(随机任务)
	RemainingIntegral   int      `json:"remainingIntegral"`   // eg: 14
	DailyMaxIntimacy    int      `json:"dailyMaxIntimacy"`    // eg: 17
	SignShowOutIcon     string   `json:"signShowOutIcon"`     // icon 地址
	SignShowOutTextList []string `json:"signShowOutTextList"` // eg: [ "已连续更新档案2天，太棒了","已累积更新档案4天，继续加油"]
	BridgingTextList    []string `json:"bridgingTextList"`    // eg: ["每次收听和支持, 都将存档记录并积累亲密值","选择一种方式更新你们今天的回忆吧"]
}

// FansGroupMissionAllResp 获取乐迷团全部任务列表响应.
type FansGroupMissionAllResp struct {
	Code    int                         `json:"code"`
	Message string                      `json:"message"`
	Data    FansGroupMissionAllRespData `json:"data"`
}

// FansGroupMissionAll 获取乐迷团全部任务列表.
// testdata/har/78.json .
func (a *Api) FansGroupMissionAll(ctx context.Context, req *FansGroupMissionAllReq) (*FansGroupMissionAllResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/fans/group/mission/all?fansGroupId=" + req.FansGroupId
		reply FansGroupMissionAllResp
		opts  = api.NewOptions("eapi.FansGroupMissionAll").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// FansGroupFeedRecommendReq 获取乐迷团推荐Feed请求.
type FansGroupFeedRecommendReq struct {
	types.EApiReqCommon

	ArtistSelf  string `json:"artistSelf"`  // 固定 "0"
	Cursor      string `json:"cursor"`      // 游标, 首次 "0"
	FansGroupId string `json:"fansGroupId"` // 乐迷团ID
	Size        string `json:"size"`        // 数量, 默认 "10"
}

// FansGroupFeedRecommendResp 获取乐迷团推荐Feed响应.
type FansGroupFeedRecommendResp struct {
	Code    int                            `json:"code"`
	Message string                         `json:"message"`
	Data    FansGroupFeedRecommendRespData `json:"data"`
}

type FansGroupFeedRecommendRespData struct {
	Records []FansGroupFeedRecommendRespDataRecords `json:"records"`
	Page    struct {
		Cursor string `json:"cursor"`
		More   bool   `json:"more"`
		Size   int    `json:"size"`
	} `json:"page"`
}

type FansGroupFeedRecommendRespDataRecords struct {
	ActId   int     `json:"actId"`
	ActName *string `json:"actName"`
	AdInfo  struct {
		HasDynamicEnvelopeAd int `json:"hasDynamicEnvelopeAd"`
		HasNoteEnvelopeAd    int `json:"hasNoteEnvelopeAd"`
		HasNoteResourceAd    int `json:"hasNoteResourceAd"`
		NoteAdInfo           any `json:"noteAdInfo"`
	} `json:"adInfo"`
	AirborneActivityInfo any             `json:"airborneActivityInfo"`
	AlgResourceType      any             `json:"algResourceType"`
	AlterLinkUrl         any             `json:"alterLinkUrl"`
	AlterLinkWebviewUrl  any             `json:"alterLinkWebviewUrl"`
	AnonymityInfo        AnonymityInfo   `json:"anonymityInfo"`
	AppVersionLimit      any             `json:"appVersionLimit"`
	ArtistComments       []ArtistComment `json:"artistComments"`
	BottomActivityInfos  []ActivityInfo  `json:"bottomActivityInfos"`
	ChallengeTopicInfo   any             `json:"challengeTopicInfo"`
	CommentInfo          any             `json:"commentInfo"`
	CommentTargetUrl     any             `json:"commentTargetUrl"`
	CopyrightIconDark    any             `json:"copyrightIconDark"`
	CopyrightIconLight   any             `json:"copyrightIconLight"`
	Ctrp                 any             `json:"ctrp"`
	DiscussId            string          `json:"discussId"`
	DistributionType     any             `json:"distributionType"`
	EventActionToast     any             `json:"eventActionToast"`
	EventTime            int64           `json:"eventTime"`
	ExpireTime           int             `json:"expireTime"`
	ExtJsonInfo          ExtJsonInfo     `json:"extJsonInfo"`
	ExtPageParam         any             `json:"extPageParam"`
	ExtSource            any             `json:"extSource"`
	ExtType              string          `json:"extType"`
	FansActivityEntrance any             `json:"fansActivityEntrance"`
	FeedType             int             `json:"feedType"`
	FirstEditTime        int             `json:"firstEditTime"`
	ForwardCount         int             `json:"forwardCount"`
	FreePlaybackCode     any             `json:"freePlaybackCode"`
	HighLine             bool            `json:"highLine"`
	Id                   int64           `json:"id"`
	Info                 Info            `json:"info"`
	InsertTime           int64           `json:"insertTime"`
	InsiteForwardCount   int             `json:"insiteForwardCount"`
	IpLocation           IpLocation      `json:"ipLocation"`
	Json                 string          `json:"json"` // 内嵌 JSON 字符串
	LikeAnimationMap     map[string]any  `json:"likeAnimationMap"`
	Location             any             `json:"location"`
	LogInfo              any             `json:"logInfo"`
	LotteryEventData     any             `json:"lotteryEventData"`
	Medal                any             `json:"medal"`
	MusicianSay          bool            `json:"musicianSay"`
	Owner                bool            `json:"owner"`
	PendantData          *struct {
		Id              int64  `json:"id"`
		ImageAndroidUrl string `json:"imageAndroidUrl"`
		ImageIosUrl     string `json:"imageIosUrl"`
		ImageUrl        string `json:"imageUrl"`
	} `json:"pendantData"` // 挂件数据，有些记录可能为 null
	// Pics 动态图片 (2026-09-04 实测: 扁平层仅含 URL/Str-ID/尺寸, 数字 ID 全集在嵌套 picInfo 内).
	Pics               []FeedPic     `json:"pics"`
	PlaylistInfo       any           `json:"playlistInfo"`
	PointTopicInfo     *ActivityInfo `json:"pointTopicInfo"`
	PrivacySetting     int           `json:"privacySetting"`
	PrivacySettingInfo struct {
		Desc string `json:"desc"`
	} `json:"privacySettingInfo"` // 隐私设置描述
	ProduceInfo any `json:"produceInfo"` // 可能为 string 或 null
	Question    any `json:"question"`
	RcmdInfo    struct {
		Alg        string `json:"alg"`
		CircleId   any    `json:"circleId"`
		Pos        int    `json:"pos"`
		Reason     any    `json:"reason"`
		Scene      string `json:"scene"`
		Type       int    `json:"type"`
		UserReason string `json:"userReason"`
	} `json:"rcmdInfo"`
	RedEnvelopeDTO     any            `json:"redEnvelopeDTO"`
	RelationTopic      any            `json:"relationTopic"` // 可能是 bool 或 null
	Reward             any            `json:"reward"`
	ShowFollowButton   any            `json:"showFollowButton"`
	ShowTime           int64          `json:"showTime"`
	SocialSpaceVisible any            `json:"socialSpaceVisible"`
	SocialUserId       any            `json:"socialUserId"`
	SongStarInfo       any            `json:"songStarInfo"`
	SrcResId           any            `json:"srcResId"`
	SrcResThreadId     any            `json:"srcResThreadId"`
	SrcResType         any            `json:"srcResType"`
	TagInfo            any            `json:"tagInfo"`
	TailMark           *TailMark      `json:"tailMark"` // 可能为 null
	ThreadId           string         `json:"threadId"`
	TimingInfo         any            `json:"timingInfo"`
	TitleAlias         any            `json:"titleAlias"`
	TmplId             int            `json:"tmplId"`
	TopActivityInfos   []any          `json:"topActivityInfos"`
	TopEvent           bool           `json:"topEvent"`
	TopicActivity      any            `json:"topicActivity"`
	Type               int            `json:"type"`
	TypeDesc           string         `json:"typeDesc"`
	User               User           `json:"user"`
	UserBizLevels      []UserBizLevel `json:"userBizLevels"`
	UserInfoExt        struct {
		MemberRoleType int   `json:"memberRoleType"`
		UserId         int64 `json:"userId"`
	} `json:"userInfoExt"`
	UserNameplates any    `json:"userNameplates"`
	Uuid           string `json:"uuid"`
	Voice          any    `json:"voice"`
}

// AnonymityInfo 匿名信息.
type AnonymityInfo struct {
	Anonymous  int `json:"anonymous"`
	AvatarUrl  any `json:"avatarUrl"`
	LabelIcons any `json:"labelIcons"`
	Me         any `json:"me"`
	Name       any `json:"name"`
}

// ArtistComment 合伙人评论.
type ArtistComment struct {
	Action         int    `json:"action"`
	CommentContent any    `json:"commentContent"`
	Content        string `json:"content"`
	ResourceUserId int64  `json:"resourceUserId"`
	ThreadId       string `json:"threadId"`
	TimeStamp      any    `json:"timeStamp"`
	UserId         int64  `json:"userId"`
}

// ActivityInfo 活动/话题信息（也用于 bottomActivityInfos 和 extJsonInfo.activityInfos）.
type ActivityInfo struct {
	ArtistId           any     `json:"artistId"`
	Desc               any     `json:"desc"`
	Ext                any     `json:"ext"`
	Hot                bool    `json:"hot"`
	HotDiscussNumDesc  any     `json:"hotDiscussNumDesc"`
	HotIcon            any     `json:"hotIcon"`
	Icon               *string `json:"icon"`
	Id                 string  `json:"id"`
	LogInfo            any     `json:"logInfo"`
	MomentTopic        bool    `json:"momentTopic"`
	Name               string  `json:"name"`
	Parent             any     `json:"parent"`
	Pic                any     `json:"pic"`
	PubGuide           bool    `json:"pubGuide"`
	PubGuideActionText any     `json:"pubGuideActionText"`
	PubGuideIcon       any     `json:"pubGuideIcon"`
	PubGuideText       any     `json:"pubGuideText"`
	SquareDesc         any     `json:"squareDesc"`
	SubType            int     `json:"subType"`
	Target             string  `json:"target"`
	ThroughInfo        any     `json:"throughInfo"`
	Type               int     `json:"type"`
}

// ExtJsonInfo 扩展 JSON 信息.
type ExtJsonInfo struct {
	ActId              int               `json:"actId"`
	ActIds             []int             `json:"actIds"`
	ActivityInfos      []ActivityInfo    `json:"activityInfos"`
	AiPrivatePicInfo   any               `json:"aiPrivatePicInfo"`
	AiTitleMap         map[string]string `json:"aiTitleMap"` // 例如 "default": "快来看看我的八月听歌足迹呀"
	AnonymityInfo      AnonymityInfo     `json:"anonymityInfo"`
	CircleId           string            `json:"circleId"`
	CirclePubType      any               `json:"circlePubType"`
	DistributionType   any               `json:"distributionType"`
	EditTime           int               `json:"editTime"`
	ExtId              string            `json:"extId"`
	ExtParams          map[string]string `json:"extParams"`
	ExtSource          any               `json:"extSource"`
	ExtType            string            `json:"extType"`
	FirstRecommendTime int64             `json:"firstRecommendTime"`
	ImageScore         any               `json:"imageScore"`
	MomentScore        any               `json:"momentScore"`
	MultiAiPicInfo     any               `json:"multiAiPicInfo"`
	MustShowEventFeed  bool              `json:"mustShowEventFeed"`
	NoteArtistIdList   any               `json:"noteArtistIdList"`
	NoteArtistNameList any               `json:"noteArtistNameList"`
	NoteIpIdList       any               `json:"noteIpIdList"`
	NoteIpNameList     any               `json:"noteIpNameList"`
	PicColorMap        any               `json:"picColorMap"`
	PointTopicInfo     *ActivityInfo     `json:"pointTopicInfo"`
	PrivacySetting     int               `json:"privacySetting"`
	PubSource          *struct {
		BizCode         string `json:"bizCode"`
		EntranceCode    string `json:"entranceCode"`
		NeedReachNotice bool   `json:"needReachNotice"`
		TagDto          any    `json:"tagDto"`
	} `json:"pubSource"`
	QuestionId             any    `json:"questionId"`
	RecommendStatus        int    `json:"recommendStatus"`
	RedEnvelopeDTO         any    `json:"redEnvelopeDTO"`
	Reward                 any    `json:"reward"`
	RiskControlComplainDTO any    `json:"riskControlComplainDTO"`
	SocialSpaceVisible     any    `json:"socialSpaceVisible"`
	SocialUserId           any    `json:"socialUserId"`
	SrcResId               any    `json:"srcResId"`
	SrcResType             any    `json:"srcResType"`
	TailMark               any    `json:"tailMark"`
	TitleAlias             any    `json:"titleAlias"`
	TitlePicInfo           any    `json:"titlePicInfo"`
	TitlePicType           any    `json:"titlePicType"`
	TypeDesc               any    `json:"typeDesc"`
	Uuid                   string `json:"uuid"`
	VoiceInfo              any    `json:"voiceInfo"`
}

// Info 互动信息.
type Info struct {
	CommentCount     int               `json:"commentCount"`
	CommentThread    CommentThread     `json:"commentThread"`
	Comments         []any             `json:"comments"`
	LatestLikedUsers []LatestLikedUser `json:"latestLikedUsers"`
	Liked            bool              `json:"liked"`
	LikedCount       int               `json:"likedCount"`
	ResourceId       int64             `json:"resourceId"`
	ResourceType     int               `json:"resourceType"`
	ShareCount       int               `json:"shareCount"`
	ThreadId         string            `json:"threadId"`
}

// CommentThread 评论线程.
type CommentThread struct {
	CommentCount     int               `json:"commentCount"`
	HotCount         int               `json:"hotCount"`
	Id               string            `json:"id"`
	LatestLikedUsers []LatestLikedUser `json:"latestLikedUsers"`
	LikedCount       int               `json:"likedCount"`
	ResourceId       int64             `json:"resourceId"`
	ResourceInfo     *struct {
		Creator any    `json:"creator"`
		Id      int64  `json:"id"`
		ImgUrl  any    `json:"imgUrl"`
		Name    string `json:"name"`
		UserId  int64  `json:"userId"`
	} `json:"resourceInfo"`
	ResourceOwnerId int64 `json:"resourceOwnerId"`
	ResourceType    int   `json:"resourceType"`
	ShareCount      int   `json:"shareCount"`
}

// LatestLikedUser 最近点赞用户.
type LatestLikedUser struct {
	Time   int64 `json:"time"`
	UserId int64 `json:"userId"`
}

// IpLocation IP 位置.
type IpLocation struct {
	Ip       any    `json:"ip"`
	Location string `json:"location"`
}

// FeedPic 乐迷团动态图片元素 (2026-09-04 实测载荷): 扁平层承载展示 URL、Str 形态 ID 与尺寸,
// 数字形态 ID 全集位于嵌套 picInfo 对象内, 故拆为 FeedPic(扁平)+PicInfo(嵌套) 两层建模.
type FeedPic struct {
	// 扁平层
	OriginUrl        string  `json:"originUrl"`
	SquareUrl        string  `json:"squareUrl"`
	RectangleUrl     string  `json:"rectangleUrl"`
	PcSquareUrl      any     `json:"pcSquareUrl"`    // 实测为 URL string
	PcRectangleUrl   any     `json:"pcRectangleUrl"` // 实测为 URL string
	OriginId         int64   `json:"originId"`
	OriginIdStr      string  `json:"originIdStr"`
	SquareIdStr      string  `json:"squareIdStr"`
	RectangleIdStr   string  `json:"rectangleIdStr"`
	PcSquareIdStr    string  `json:"pcSquareIdStr"`
	PcRectangleIdStr string  `json:"pcRectangleIdStr"`
	Format           string  `json:"format"`
	Width            float64 `json:"width"`
	Height           float64 `json:"height"`
	VideoNosKey      any     `json:"videoNosKey"`
	VideoDurationMs  int     `json:"videoDurationMs"`
	VideoUrl         any     `json:"videoUrl"`
	VideoOriginalUrl any     `json:"videoOriginalUrl"`
	Tags             any     `json:"tags"`

	// 嵌套层: 数字 ID 全集与转码信息
	PicInfo PicInfo `json:"picInfo"`
}

// PicInfo 乐迷团动态图片嵌套 picInfo 对象 (数字 ID 全集).
type PicInfo struct {
	OriginId         int64   `json:"originId"`
	SquareId         int64   `json:"squareId"`
	RectangleId      int64   `json:"rectangleId"`
	PcSquareId       int64   `json:"pcSquareId"`
	PcRectangleId    int64   `json:"pcRectangleId"`
	OriginJpgId      int64   `json:"originJpgId"`
	OriginIdStr      string  `json:"originIdStr"`
	SquareIdStr      string  `json:"squareIdStr"`
	RectangleIdStr   string  `json:"rectangleIdStr"`
	PcSquareIdStr    string  `json:"pcSquareIdStr"`
	PcRectangleIdStr string  `json:"pcRectangleIdStr"`
	PcSquareUrl      any     `json:"pcSquareUrl"`
	PcRectangleUrl   any     `json:"pcRectangleUrl"`
	Format           string  `json:"format"`
	Width            float64 `json:"width"`
	Height           float64 `json:"height"`
	VideoNosKey      any     `json:"videoNosKey"`
	VideoDurationMs  int     `json:"videoDurationMs"`
	VideoUrl         any     `json:"videoUrl"`
	VideoOriginalUrl any     `json:"videoOriginalUrl"`
	VideoId          any     `json:"videoId"`
	TranscodeStatus  any     `json:"transcodeStatus"`
	Tags             any     `json:"tags"`
}

// TailMark 尾部标记（可能为 null）.
type TailMark struct {
	Circle struct {
		ImageUrl  string `json:"imageUrl"`
		Member    string `json:"member"`
		PostCount string `json:"postCount"`
	} `json:"circle"` // 圈子信息.
	ExtInfo        any    `json:"extInfo"`
	MarkOrpheusUrl string `json:"markOrpheusUrl"`
	MarkResourceId string `json:"markResourceId"`
	MarkTitle      string `json:"markTitle"`
	MarkType       string `json:"markType"`
}

// User 用户信息.
type User struct {
	AccountStatus       int `json:"accountStatus"`
	AuthStatus          int `json:"authStatus"`
	AuthenticationTypes int `json:"authenticationTypes"`
	Authority           int `json:"authority"`
	AvatarDetail        *struct {
		IdentityIconUrl string `json:"identityIconUrl"`
		IdentityLevel   int    `json:"identityLevel"`
		UserType        int    `json:"userType"`
	} `json:"avatarDetail"`
	AvatarImgId        int64  `json:"avatarImgId"`
	AvatarImgIdStr     string `json:"avatarImgIdStr"`
	AvatarImgId_str    string `json:"avatarImgId_str"` // 注意字段名
	AvatarUrl          string `json:"avatarUrl"`
	BackgroundImgId    int64  `json:"backgroundImgId"`
	BackgroundImgIdStr string `json:"backgroundImgIdStr"`
	BackgroundUrl      string `json:"backgroundUrl"`
	Birthday           int64  `json:"birthday"`
	City               int    `json:"city"`
	CommonIdentity     *struct {
		IconUrl string `json:"iconUrl"`
		Link    string `json:"link"`
		Target  string `json:"target"`
		Title   string `json:"title"`
	} `json:"commonIdentity"`
	DefaultAvatar     bool      `json:"defaultAvatar"`
	Description       string    `json:"description"`
	DetailDescription string    `json:"detailDescription"`
	DjStatus          int       `json:"djStatus"`
	EncryptUserId     any       `json:"encryptUserId"`
	ExpertTags        any       `json:"expertTags"`
	Experts           any       `json:"experts"`
	Followed          bool      `json:"followed"`
	Followeds         int       `json:"followeds"`
	Gender            int       `json:"gender"`
	IdentityLabels    any       `json:"identityLabels"`
	MusicianSay       bool      `json:"musicianSay"`
	Mutual            bool      `json:"mutual"`
	Nickname          string    `json:"nickname"`
	Province          int       `json:"province"`
	RelationTag       any       `json:"relationTag"`
	RemarkName        any       `json:"remarkName"`
	Signature         string    `json:"signature"`
	SocialUserId      any       `json:"socialUserId"`
	Target            any       `json:"target"`
	UrlAnalyze        bool      `json:"urlAnalyze"`
	UserId            int64     `json:"userId"`
	UserType          int       `json:"userType"`
	VipRights         VipRights `json:"vipRights"`
	VipType           int       `json:"vipType"`
}

// VipRights VIP 权益.
type VipRights struct {
	Associator *struct {
		IconUrl string `json:"iconUrl"`
		Rights  bool   `json:"rights"`
		VipCode int    `json:"vipCode"`
	} `json:"associator"`
	ExtInfo    VipExtInfo `json:"extInfo"`
	MemberLogo *struct {
		ActionUrl  string  `json:"actionUrl"`
		Height     float64 `json:"height"`
		InterestId int64   `json:"interestId"`
		Url        string  `json:"url"`
		Width      float64 `json:"width"`
	} `json:"memberLogo"`
	MusicPackage *struct {
		IconUrl string `json:"iconUrl"`
		Rights  bool   `json:"rights"`
		VipCode int    `json:"vipCode"`
	} `json:"musicPackage"`
	RedVipAnnualCount int `json:"redVipAnnualCount"`
	RedVipLevel       int `json:"redVipLevel"`
	Redplus           *struct {
		IconUrl string `json:"iconUrl"`
		Rights  bool   `json:"rights"`
		VipCode int    `json:"vipCode"`
	} `json:"redplus"`
	RelationType int `json:"relationType"`
}

// VipExtInfo VIP 扩展信息.
type VipExtInfo struct {
	Logo *struct {
		LogoDto struct {
			ActionUrl  string  `json:"actionUrl"`
			Height     float64 `json:"height"`
			InterestId int64   `json:"interestId"`
			LogoType   int     `json:"logoType"`
			Url        string  `json:"url"`
			Width      float64 `json:"width"`
		} `json:"logoDto"`
	} `json:"logo"`
}

// UserBizLevel 用户业务等级.
type UserBizLevel struct {
	BackgroundColor        any    `json:"backgroundColor"`
	BackgroundEdgeUrl      string `json:"backgroundEdgeUrl"`
	BackgroundUrl          string `json:"backgroundUrl"`
	BizCode                string `json:"bizCode"`
	Degrade                bool   `json:"degrade"`
	ExtParams              any    `json:"extParams"`
	Level                  string `json:"level"`
	LevelUrl               string `json:"levelUrl"`
	NameplateTagImgExtInfo any    `json:"nameplateTagImgExtInfo"`
	RelatedId              any    `json:"relatedId"`
	SocialUserTargetMap    any    `json:"socialUserTargetMap"`
	Target                 string `json:"target"`
	Text                   string `json:"text"`
	TextColor              any    `json:"textColor"`
	Type                   string `json:"type"`
}

// FansGroupFeedRecommend 获取乐迷团推荐Feed.
// testdata/har/79.json .
func (a *Api) FansGroupFeedRecommend(ctx context.Context, req *FansGroupFeedRecommendReq) (*FansGroupFeedRecommendResp, error) {
	if req.ArtistSelf == "" {
		req.ArtistSelf = "0"
	}

	if req.Cursor == "" {
		req.Cursor = "0"
	}

	if req.Size == "" {
		req.Size = "10"
	}

	var (
		reply      FansGroupFeedRecommendResp
		opts       = api.NewOptions("eapi.FansGroupFeedRecommend").SetEAPI()
		requestURL = fmt.Sprintf(
			"https://interface3.music.163.com/eapi/fans/group/feed/recommend/get?artistSelf=%s&cursor=%s&fansGroupId=%s&size=%s",
			req.ArtistSelf,
			req.Cursor,
			req.FansGroupId,
			req.Size,
		)
	)

	if _, err := a.client.Request(ctx, requestURL, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	return &reply, nil
}

// FansGroupMissionForwardProgressReq 分享进度上报请求.
type FansGroupMissionForwardProgressReq struct {
	types.EApiReqCommon

	ResourceId   string `json:"resourceId"`   // 歌曲ID (从任务列表的 button.url 中解析)
	Action       string `json:"action"`       // 固定 "share"
	FansGroupId  string `json:"fansGroupId"`  // 固定 "null" (HAR中观察到的值)
	ResourceType string `json:"resourceType"` // 固定 "4" (歌曲类型)
}

// FansGroupMissionForwardProgressResp 分享进度上报响应.
type FansGroupMissionForwardProgressResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// FansGroupMissionForwardProgress 分享进度上报.
func (a *Api) FansGroupMissionForwardProgress(ctx context.Context, req *FansGroupMissionForwardProgressReq) (*FansGroupMissionForwardProgressResp, error) {
	if req.Action == "" {
		req.Action = "share"
	}

	if req.FansGroupId == "" {
		req.FansGroupId = "null"
	}

	if req.ResourceType == "" {
		req.ResourceType = "4"
	}

	var (
		reply      FansGroupMissionForwardProgressResp
		opts       = api.NewOptions("eapi.FansGroupMissionForwardProgress").SetEAPI()
		requestURL = fmt.Sprintf(
			"https://interface3.music.163.com/eapi/fans/group/mission/forward/progress?resourceId=%s&action=%s&fansGroupId=%s&resourceType=%s",
			req.ResourceId,
			req.Action,
			req.FansGroupId,
			req.ResourceType,
		)
	)

	if _, err := a.client.Request(ctx, requestURL, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	return &reply, nil
}

// ResourceLikeReq 点赞资源请求.
type ResourceLikeReq struct {
	types.EApiReqCommon

	ThreadId  string `json:"threadId"`  // 动态的ThreadId, 格式如: A_EV_2_{eventId}_{userId}
	AppLogExt string `json:"appLogExt"` // 日志扩展字段, 包含乐迷团归属信息
}

// ResourceLikeResp 点赞资源响应.
type ResourceLikeResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ResourceLike 点赞资源 (用于点赞乐迷团笔记).
func (a *Api) ResourceLike(ctx context.Context, req *ResourceLikeReq) (*ResourceLikeResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/resource/like"
		reply ResourceLikeResp
		opts  = api.NewOptions("eapi.ResourceLike").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// FansGroupUserGroupDetailGetReq 获取用户在乐迷团的详情请求.
type FansGroupUserGroupDetailGetReq struct {
	types.EApiReqCommon

	GroupId string `json:"groupId"` // 乐迷团ID
}

type FansGroupUserGroupDetailGetRespDataFansGroupMemberDetail struct {
	UserId           int64  `json:"userId"`
	Nickname         string `json:"nickname"`
	AvatarUrl        string `json:"avatarUrl"`
	AvatarDetail     any    `json:"avatarDetail"`
	AccountStatus    int64  `json:"accountStatus"`
	Active           bool   `json:"active"`
	Joined           bool   `json:"joined"`
	UserHidden       bool   `json:"userHidden"`
	UserPriority     bool   `json:"userPriority"`
	Follow           bool   `json:"follow"`
	FansGroupId      string `json:"fansGroupId"`
	RemainingUpgrade int64  `json:"remainingUpgrade"`
	// Identify 身份标识码 (2026-09-04 实测键名为 identify, eg: 99; 原误写为 identity 导致永不解析).
	Identify  int64     `json:"identify"`
	VipRights VipRights `json:"vipRights"`
	Integral  string    `json:"integral"` // 当前已获得的亲密值
	No        string    `json:"no"`
	Level     struct {
		Level              string `json:"level"`
		LvelPicUrl         string `json:"levelPicUrl"`
		LevelIntegral      int64  `json:"levelIntegral"`
		LevelUpPopPicUrl   any    `json:"levelUpPopPicUrl"`
		LevelMedalIconUrl  string `json:"levelMedalIconUrl"`
		LevelLightImageUrl string `json:"levelLightImageUrl"`
		MedalCode          string `json:"medalCode"`   // eg: 20170503_96511727
		FanTitle           string `json:"fanTitle"`    // eg: 新秀乐迷
		Segment            string `json:"segment"`     // eg: LV.2
		SegmentCode        string `json:"segmentCode"` // eg: 2
	} `json:"level"`
	// FansNameplate 乐迷团铭牌 (2026-09-04 实测载荷补全内层字段与外层 json tag, 此前仅靠大小写不敏感兜底).
	FansNameplate struct {
		BackgroundColor        any    `json:"backgroundColor"`
		BackgroundEdgeUrl      string `json:"backgroundEdgeUrl"`
		BackgroundUrl          string `json:"backgroundUrl"`
		BizCode                string `json:"bizCode"` // eg: FANS_NAMEPLATE
		Degrade                bool   `json:"degrade"`
		ExtParams              any    `json:"extParams"`
		Level                  string `json:"level"` // eg: "2"
		LevelUrl               string `json:"levelUrl"`
		NameplateTagImgExtInfo any    `json:"nameplateTagImgExtInfo"`
		RelatedId              any    `json:"relatedId"`
		SocialUserTargetMap    any    `json:"socialUserTargetMap"`
		Target                 string `json:"target"`
		Text                   string `json:"text"` // eg: 音乐合伙人乐迷
		TextColor              any    `json:"textColor"`
		Type                   string `json:"type"` // eg: "1"
		// 以下为 2026-09-04 载荷实测补充字段
		NameplateType            string `json:"nameplateType"` // eg: "normal"
		ActiveStartDate          any    `json:"activeStartDate"`
		ActiveEndDate            any    `json:"activeEndDate"`
		WearingNameplate         bool   `json:"wearingNameplate"`
		FansGroupUserNameplateId int64  `json:"fansGroupUserNameplateId"`
		FansGroupId              int64  `json:"fansGroupId"`
		UserId                   int64  `json:"userId"`
	} `json:"fansNameplate"`
}

type FansGroupUserGroupDetailGetRespData struct {
	FansGroupMemberDetail FansGroupUserGroupDetailGetRespDataFansGroupMemberDetail `json:"fansGroupMemberDetail"`
}

// FansGroupUserGroupDetailGetResp 获取用户在乐迷团的详情响应.
type FansGroupUserGroupDetailGetResp struct {
	Code    int                                 `json:"code"`
	Message string                              `json:"message"`
	Error   bool                                `json:"error"`
	Data    FansGroupUserGroupDetailGetRespData `json:"data"`
}

// FansGroupUserGroupDetailGet 获取用户在乐迷团的详情.
// testdata/har/80.json .
func (a *Api) FansGroupUserGroupDetailGet(ctx context.Context, req *FansGroupUserGroupDetailGetReq) (*FansGroupUserGroupDetailGetResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/social/fansgroup/bff/user/group/detail/get?groupId=" + req.GroupId
		reply FansGroupUserGroupDetailGetResp
		opts  = api.NewOptions("eapi.FansGroupUserGroupDetailGet").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}
