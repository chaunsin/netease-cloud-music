// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

const dailySongShareMConfigInfo = `{"IuRPVVmc3WWul9fT":{"version":"115240960","appver":"9.5.37"},"tPJJnts2H31BZXmp":{"version":"5230592","appver":"4.74.0"},"c0Ve6C0uNl2Am0Rl":{"version":"276480","appver":"1.4.30"},"zr4bw6pKFDIZScpo":{"version":"3758080","appver":"2.40.0"}}`

// DailySongShareRegisterReq registers the current user for the sharing activity.
type DailySongShareRegisterReq struct {
	types.EApiReqCommon
}

// DailySongShareRegisterResp is the sharing activity registration response.
type DailySongShareRegisterResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		NoteAttendance bool `json:"noteAttendance"`
	} `json:"data"`
}

// DailySongShareRegister registers the current user for the daily sharing activity.
func (a *Api) DailySongShareRegister(ctx context.Context, req *DailySongShareRegisterReq) (*DailySongShareRegisterResp, error) {
	if req == nil {
		req = &DailySongShareRegisterReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/common/activity/in/registration"
		reply    DailySongShareRegisterResp
		opts     = api.NewOptions("eapi.DailySongShareRegister").SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share registration: %w", err)
	}
	return &reply, nil
}

// DailySongShareAttendanceRegisterReq registers an activity attendance cycle.
type DailySongShareAttendanceRegisterReq struct {
	types.EApiReqCommon

	ActivityId      int64 `json:"activityId"`
	ActivityCycleId int64 `json:"activityCycleId"`
	AutoRegister    bool  `json:"autoRegister"`
}

// DailySongShareAttendanceRegisterResp is the attendance registration response.
type DailySongShareAttendanceRegisterResp struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// DailySongShareAttendanceRegister enrolls the current activity cycle.
func (a *Api) DailySongShareAttendanceRegister(ctx context.Context, req *DailySongShareAttendanceRegisterReq) (*DailySongShareAttendanceRegisterResp, error) {
	if req == nil {
		return nil, errors.New("daily song share attendance register request is nil")
	}

	req.AutoRegister = true

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/attendance/activity/register"
		reply    DailySongShareAttendanceRegisterResp
		opts     = api.NewOptions("eapi.DailySongShareAttendanceRegister").SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share attendance registration: %w", err)
	}
	return &reply, nil
}

// DailySongShareRegistrationGuideReq requests the current activity guide.
type DailySongShareRegistrationGuideReq struct {
	types.EApiReqCommon
}

// DailySongShareRegistrationGuideResp describes the current activity and progress.
type DailySongShareRegistrationGuideResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RegisterStatus                      string `json:"registerStatus"`                 // NOREGISTER:没有报名活动 REGISTER:已参与报名
		ActivityId                          int64  `json:"activityId"`                     // eg: 1
		ActivityCycleId                     int64  `json:"activityCycleId"`                // 活动周期id 通常每周+1 eg: 501572
		ActivityInterestId                  int64  `json:"activityInterestId"`             // eg: 11066304
		RewardJumpUrl                       string `json:"rewardJumpUrl"`                  // eg: https://st.music.163.com/g/platform/lottery?activityIds=11066304\u0026newVersion=1
		Duration                            string `json:"duration"`                       // eg: 第0817-0823期
		VipPopNotificationVo                any    `json:"vipPopNotificationVo,omitempty"` //
		NoteAttendanceNoRegistrationGuideVo struct {
			Title      any `json:"title"`
			Content    any `json:"content"`
			SubContent any `json:"subContent"`
			Pic        any `json:"pic"`
			SignUp     any `json:"signUp"`
			SignUpTip  any `json:"signUpTip"`
		} `json:"noteAttendanceNoRegistrationGuideVo"`
		RegisteredGuide struct {
			AvatarUrl       string                      `json:"avatarUrl"`       // eg: http://p2.music.126.net/WieETOwCMTVpV7CKerCjJA==/109951163670838912.jpg
			AvatarTip       string                      `json:"avatarTip"`       // eg: 第0824期 · 挑战1天
			Title           string                      `json:"title"`           // eg: 每日推歌挑战赛
			Content         string                      `json:"content"`         // eg: 连续发布7天可额外获得黑胶VIP会员奖励
			RichContent     RegisteredGuideRichContent  `json:"richContent"`     //
			RewardCardList  []RegisteredGuideRewardCard `json:"rewardCardList"`  // 奖励内容描述
			SignUp          string                      `json:"signUp"`          // eg: 暂无抽奖机会、立即抽奖(1次)
			SignTip         string                      `json:"signTip"`         // eg: 剩余0次机会、剩余1次机会
			RewardCount     int64                       `json:"rewardCount"`     //
			HaveRewardCount int64                       `json:"haveRewardCount"` // 今日做任务可获得抽奖次数
			AlreadyPubEvent bool                        `json:"alreadyPubEvent"` // 貌似是今日是否有发布动态，待确认？
			OnlyOnceLeft    bool                        `json:"onlyOnceLeft"`    //
			PubEventCount   int64                       `json:"pubEventCount"`   // 本周期内已发布的动态次数
		} `json:"noteAttendanceRegisteredGuideVo"`
	} `json:"data"`
}

type RegisteredGuideRichContent struct {
	Text      string `json:"text"`      // eg: 连续发布%s天可额外获得黑胶VIP会员奖励
	PlaceText string `json:"placeText"` // eg: 7
}

type RegisteredGuideRewardCard struct {
	Pic          string `json:"pic"`          // eg: https://p6.music.126.net/obj/wonDlsKUwrLClGjCm8Kx/60688668715/4db5/6020/3d7f/e7c578763f91f986c8c6832351ab560a.png
	Name         string `json:"name"`         // 包含换行符，eg: "VIP\n年卡"
	Outline      string `json:"outline"`      // eg: #918787
	GradientMask string `json:"gradientMask"` // eg: #81714D
	Projection   string `json:"projection"`   // eg: #918787
}

type NoteAttendanceNoRegistrationGuideVo struct {
	Title      any `json:"title"`
	Content    any `json:"content"`
	SubContent any `json:"subContent"`
	Pic        any `json:"pic"`
	SignUp     any `json:"signUp"`
	SignUpTip  any `json:"signUpTip"`
}

// DailySongShareRegistrationGuide gets the current activity guide and identifiers.
func (a *Api) DailySongShareRegistrationGuide(ctx context.Context, req *DailySongShareRegistrationGuideReq) (*DailySongShareRegistrationGuideResp, error) {
	if req == nil {
		req = &DailySongShareRegistrationGuideReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/attendance/activity/registration/v2/guide"
		reply    DailySongShareRegistrationGuideResp
		opts     = api.NewOptions("eapi.DailySongShareRegistrationGuide").SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share registration guide: %w", err)
	}
	return &reply, nil
}

// DailySongSharePublishReq publishes a note used by the sharing activity.
type DailySongSharePublishReq struct {
	types.EApiReqCommon

	AddComment         bool   `json:"addComment"`
	AutoSaveDraft      bool   `json:"autoSaveDraft"`
	Id                 string `json:"id,omitempty"`
	ThreadId           string `json:"threadId,omitempty"`
	ResourceId         string `json:"resourceId,omitempty"`
	Msg                string `json:"msg"`
	SessionId          string `json:"sessionId"` // 格式貌似为 a1b2c3d4-e5f
	TargetPublishTime  any    `json:"targetPublishTime"`
	ServerUuid         string `json:"serverUuid"`
	UseNewUpload       bool   `json:"useNewUpload"`
	FromRn             bool   `json:"fromRN"`
	ActivityInfoList   string `json:"activityInfoList,omitempty"`
	PubSource          string `json:"pubSource"`
	PubTraceId         string `json:"pubTraceId"`
	PublishTime        any    `json:"publishTime"`
	Title              string `json:"title,omitempty"`
	SocialSpaceVisible int    `json:"socialSpaceVisible"`
	PrivacySetting     string `json:"privacySetting"`
	Uid                string `json:"uid"`
	ContainAiContent   bool   `json:"containAiContent"`
	Uuid               string `json:"uuid"`
	Type               string `json:"type"`
	Pics               string `json:"pics,omitempty"`
	NeedsGuardianToken bool   `json:"needsGuardianToken"`
	ContentDeclaration string `json:"contentDeclaration,omitempty"`
	ProduceInfo        string `json:"produceInfo,omitempty"`
	RepostSource       string `json:"repostSource,omitempty"`
}

type DailySonSharePublishResp struct {
	Code           int64                         `json:"code"`
	Message        string                        `json:"message"`
	ErrDetail      string                        `json:"errDetail"`
	UserID         int64                         `json:"userId"`
	ID             int64                         `json:"id"`
	Event          DailySonSharePublishRespEvent `json:"event"`
	SNS            map[string]any                `json:"sns"`
	ResURL         string                        `json:"resUrl"`
	AfterAction    any                           `json:"afterAction"`
	JustReturn     bool                          `json:"justReturn"`
	NeedPolling    bool                          `json:"needPolling"`
	PollingTime    int64                         `json:"pollingTime"`
	PollingExtInfo any                           `json:"pollingExtInfo"`
}

type DailySonSharePublishRespEvent struct {
	DiscussID        string  `json:"discussId"`
	Owner            bool    `json:"owner"`
	ActName          *string `json:"actName"`
	PendantData      any     `json:"pendantData"`
	ForwardCount     int     `json:"forwardCount"`
	LotteryEventData any     `json:"lotteryEventData"`

	// JSON 注意：这里是 JSON 字符串，不是 object。
	// 如果需要解析内部 JSON，可以 json.Unmarshal([]byte(Event.JSON), &xxx)
	JSON                 string  `json:"json"`
	TitleAlias           *string `json:"titleAlias"`
	TagInfo              any     `json:"tagInfo"`
	FansActivityEntrance any     `json:"fansActivityEntrance"`
	Location             any     `json:"location"`
	IPLocation           struct {
		IP       any    `json:"ip"`
		Location string `json:"location"`
	} `json:"ipLocation"`
	User                DailySonSharePublishRespEventUser        `json:"user"`
	UUID                string                                   `json:"uuid"`
	ExpireTime          int64                                    `json:"expireTime"`
	RcmdInfo            any                                      `json:"rcmdInfo"`
	EventTime           int64                                    `json:"eventTime"`
	ActID               int64                                    `json:"actId"`
	Pics                []DailySonSharePublishRespEventPic       `json:"pics"`
	TmplID              int                                      `json:"tmplId"`
	ShowTime            int64                                    `json:"showTime"`
	InsertTime          int64                                    `json:"insertTime"`
	ID                  int64                                    `json:"id"`
	ThreadID            string                                   `json:"threadId"`
	Type                int                                      `json:"type"`
	ExtType             string                                   `json:"extType"`
	ExtSource           any                                      `json:"extSource"`
	DistributionType    any                                      `json:"distributionType"`
	SrcResID            any                                      `json:"srcResId"`
	SrcResType          any                                      `json:"srcResType"`
	SrcResThreadID      any                                      `json:"srcResThreadId"`
	TopEvent            bool                                     `json:"topEvent"`
	InsiteForwardCount  int                                      `json:"insiteForwardCount"`
	Info                DailySonSharePublishRespEventInfo        `json:"info"`
	TailMark            any                                      `json:"tailMark"`
	TypeDesc            string                                   `json:"typeDesc"`
	AlterLinkURL        any                                      `json:"alterLinkUrl"`
	AlterLinkWebviewURL any                                      `json:"alterLinkWebviewUrl"`
	ExtJSONInfo         DailySonSharePublishRespEventExtJSONInfo `json:"extJsonInfo"`
	PrivacySetting      int                                      `json:"privacySetting"`
	PrivacySettingInfo  struct {
		Desc string `json:"desc"`
	} `json:"privacySettingInfo"`
	ExtPageParam         any                                         `json:"extPageParam"`
	LogInfo              any                                         `json:"logInfo"`
	Question             any                                         `json:"question"`
	TopActivityInfos     []any                                       `json:"topActivityInfos"`
	BottomActivityInfos  []any                                       `json:"bottomActivityInfos"`
	PointTopicInfo       DailySonSharePublishRespEventPointTopicInfo `json:"pointTopicInfo"`
	ChallengeTopicInfo   any                                         `json:"challengeTopicInfo"`
	Voice                any                                         `json:"voice"`
	TimingInfo           any                                         `json:"timingInfo"`
	EventActionToast     any                                         `json:"eventActionToast"`
	RelationTopic        any                                         `json:"relationTopic"`
	AnonymityInfo        DailySonSharePublishRespEventAnonymityInfo  `json:"anonymityInfo"`
	CTRP                 any                                         `json:"ctrp"`
	CommentInfo          any                                         `json:"commentInfo"`
	UserBizLevels        any                                         `json:"userBizLevels"`
	CopyrightIconLight   any                                         `json:"copyrightIconLight"`
	CopyrightIconDark    any                                         `json:"copyrightIconDark"`
	UserNameplates       any                                         `json:"userNameplates"`
	SocialUserID         any                                         `json:"socialUserId"`
	SocialSpaceVisible   any                                         `json:"socialSpaceVisible"`
	CommentTargetURL     any                                         `json:"commentTargetUrl"`
	AirborneActivityInfo any                                         `json:"airborneActivityInfo"`
	ShowFollowButton     any                                         `json:"showFollowButton"`
	MusicianSay          bool                                        `json:"musicianSay"`
	TopicActivity        any                                         `json:"topicActivity"`
	Medal                any                                         `json:"medal"`
	AppVersionLimit      any                                         `json:"appVersionLimit"`
	RedEnvelopeDTO       any                                         `json:"redEnvelopeDTO"`
	LikeAnimationMap     map[string]any                              `json:"likeAnimationMap"`
	Reward               any                                         `json:"reward"`
	AlgResourceType      any                                         `json:"algResourceType"`
	SongStarInfo         struct {
		StarCount     int64  `json:"starCount"`
		StarCountDesc string `json:"starCountDesc"`
		StarStatus    int    `json:"starStatus"`
	} `json:"songStarInfo"`
	PlaylistInfo any `json:"playlistInfo"`
	AdInfo       struct {
		HasNoteResourceAd    int `json:"hasNoteResourceAd"`
		HasDynamicEnvelopeAd int `json:"hasDynamicEnvelopeAd"`
		HasNoteEnvelopeAd    int `json:"hasNoteEnvelopeAd"`
		NoteAdInfo           any `json:"noteAdInfo"`
	} `json:"adInfo"`
	FreePlaybackCode           any    `json:"freePlaybackCode"`
	ProduceInfo                any    `json:"produceInfo"`
	FirstEditTime              int64  `json:"firstEditTime"`
	SearchExplicitTitle        any    `json:"searchExplicitTitle"`
	PushExplicitTitle          any    `json:"pushExplicitTitle"`
	QuickDiscoveryAITitle      any    `json:"quickDiscoveryAiTitle"`
	SongPlayStyle              string `json:"songPlayStyle"`
	EventDetailFeedConfig      any    `json:"eventDetailFeedConfig"`
	ContainAIContent           bool   `json:"containAiContent"`
	ContentDeclaration         any    `json:"contentDeclaration"`
	RepostSource               any    `json:"repostSource"`
	RelatedComment             any    `json:"relatedComment"`
	AlreadyProcessStaticMVTask any    `json:"alreadyProcessStaticMvTask"`
	OuterTag                   any    `json:"outerTag"`
	DefaultPicInfo             any    `json:"defaultPicInfo"`
	PubSource                  any    `json:"pubSource"`
	CanShare                   bool   `json:"canShare"`
	CommentVoice               any    `json:"commentVoice"`
}

type DailySonSharePublishRespEventUser struct {
	DefaultAvatar       bool                                       `json:"defaultAvatar"`
	Province            int64                                      `json:"province"`
	AuthStatus          int                                        `json:"authStatus"`
	Followed            bool                                       `json:"followed"`
	AvatarURL           string                                     `json:"avatarUrl"`
	AccountStatus       int                                        `json:"accountStatus"`
	Gender              int                                        `json:"gender"`
	City                int64                                      `json:"city"`
	Birthday            int64                                      `json:"birthday"`
	UserID              int64                                      `json:"userId"`
	EncryptUserID       any                                        `json:"encryptUserId"`
	UserType            int                                        `json:"userType"`
	Nickname            string                                     `json:"nickname"`
	Signature           string                                     `json:"signature"`
	Description         string                                     `json:"description"`
	DetailDescription   string                                     `json:"detailDescription"`
	AvatarImgID         int64                                      `json:"avatarImgId"`
	BackgroundImgID     int64                                      `json:"backgroundImgId"`
	BackgroundURL       string                                     `json:"backgroundUrl"`
	Authority           int                                        `json:"authority"`
	Mutual              bool                                       `json:"mutual"`
	ExpertTags          any                                        `json:"expertTags"`
	Experts             any                                        `json:"experts"`
	DJStatus            int                                        `json:"djStatus"`
	VIPType             int                                        `json:"vipType"`
	RemarkName          any                                        `json:"remarkName"`
	URLAnalyze          bool                                       `json:"urlAnalyze"`
	Followeds           int                                        `json:"followeds"`
	AvatarImgIDStr      string                                     `json:"avatarImgId_str"`
	AvatarImgIDStr2     string                                     `json:"avatarImgIdStr"`
	BackgroundImgIDStr  string                                     `json:"backgroundImgIdStr"`
	VIPRights           DailySonSharePublishRespEventUserVIPRights `json:"vipRights"`
	AvatarDetail        any                                        `json:"avatarDetail"`
	CommonIdentity      any                                        `json:"commonIdentity"`
	RelationTag         any                                        `json:"relationTag"`
	AuthenticationTypes int                                        `json:"authenticationTypes"`
	Target              any                                        `json:"target"`
	IdentityLabels      any                                        `json:"identityLabels"`
	SocialUserID        any                                        `json:"socialUserId"`
	MusicianSay         bool                                       `json:"musicianSay"`
}

type DailySonSharePublishRespEventUserVIPRights struct {
	Associator        any            `json:"associator"`
	MusicPackage      any            `json:"musicPackage"`
	RedPlus           any            `json:"redplus"`
	RedVipAnnualCount int            `json:"redVipAnnualCount"`
	RedVipLevel       int            `json:"redVipLevel"`
	RelationType      int            `json:"relationType"`
	MemberLogo        any            `json:"memberLogo"`
	ExtInfo           map[string]any `json:"extInfo"`
}

type DailySonSharePublishRespEventPic struct {
	OriginURL        string                               `json:"originUrl"`
	OriginID         int64                                `json:"originId"`
	OriginIDStr      string                               `json:"originIdStr"`
	SquareURL        string                               `json:"squareUrl"`
	SquareIDStr      string                               `json:"squareIdStr"`
	RectangleURL     string                               `json:"rectangleUrl"`
	RectangleIDStr   string                               `json:"rectangleIdStr"`
	PCSquareURL      string                               `json:"pcSquareUrl"`
	PCSquareIDStr    string                               `json:"pcSquareIdStr"`
	PCRectangleURL   string                               `json:"pcRectangleUrl"`
	PCRectangleIDStr string                               `json:"pcRectangleIdStr"`
	Format           string                               `json:"format"`
	Width            int                                  `json:"width"`
	Height           int                                  `json:"height"`
	VideoNosKey      any                                  `json:"videoNosKey"`
	VideoDurationMS  int64                                `json:"videoDurationMs"`
	VideoURL         any                                  `json:"videoUrl"`
	VideoOriginalURL any                                  `json:"videoOriginalUrl"`
	Tags             any                                  `json:"tags"`
	Mute             bool                                 `json:"mute"`
	LiveType         any                                  `json:"liveType"`
	PicInfo          DailySonSharePublishRespEventPicInfo `json:"picInfo"`
}

type DailySonSharePublishRespEventPicInfo struct {
	OriginID         int64  `json:"originId"`
	SquareID         int64  `json:"squareId"`
	RectangleID      int64  `json:"rectangleId"`
	PCSquareID       int64  `json:"pcSquareId"`
	PCRectangleID    int64  `json:"pcRectangleId"`
	OriginJPGID      int64  `json:"originJpgId"`
	Format           string `json:"format"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	VideoNosKey      any    `json:"videoNosKey"`
	VideoDurationMS  int64  `json:"videoDurationMs"`
	VideoURL         any    `json:"videoUrl"`
	VideoOriginalURL any    `json:"videoOriginalUrl"`
	VideoID          any    `json:"videoId"`
	TranscodeStatus  any    `json:"transcodeStatus"`
	Tags             any    `json:"tags"`
	Mute             bool   `json:"mute"`
	PCRectangleURL   any    `json:"pcRectangleUrl"`
	SquareIDStr      string `json:"squareIdStr"`
	RectangleIDStr   string `json:"rectangleIdStr"`
	OriginIDStr      string `json:"originIdStr"`
	PCSquareIDStr    string `json:"pcSquareIdStr"`
	PCRectangleIDStr string `json:"pcRectangleIdStr"`
	PCSquareURL      any    `json:"pcSquareUrl"`
}

type DailySonSharePublishRespEventInfo struct {
	CommentThread    DailySonSharePublishRespEventInfoInfoCommentThread `json:"commentThread"`
	LatestLikedUsers []any                                              `json:"latestLikedUsers"`
	Liked            bool                                               `json:"liked"`
	Comments         []any                                              `json:"comments"`
	ResourceType     int                                                `json:"resourceType"`
	ResourceID       int64                                              `json:"resourceId"`
	ThreadID         string                                             `json:"threadId"`
	ShareCount       int                                                `json:"shareCount"`
	CommentCount     int                                                `json:"commentCount"`
	LikedCount       int                                                `json:"likedCount"`
}

type DailySonSharePublishRespEventInfoInfoCommentThread struct {
	ID                string `json:"id"`
	ResourceInfo      any    `json:"resourceInfo"`
	ResourceType      int    `json:"resourceType"`
	CommentCount      int    `json:"commentCount"`
	FloorCommentCount int    `json:"floorCommentCount"`
	LikedCount        int    `json:"likedCount"`
	ShareCount        int    `json:"shareCount"`
	HotCount          int    `json:"hotCount"`
	LatestLikedUsers  []any  `json:"latestLikedUsers"`
	ResourceOwnerID   int64  `json:"resourceOwnerId"`
	ResourceID        int64  `json:"resourceId"`
}

type DailySonSharePublishRespEventExtJSONInfo struct {
	ActID          int64  `json:"actId"`
	ActIDs         []any  `json:"actIds"`
	UUID           string `json:"uuid"`
	ExtType        string `json:"extType"`
	ExtSource      any    `json:"extSource"`
	ExtID          string `json:"extId"`
	CircleID       string `json:"circleId"`
	CirclePubType  any    `json:"circlePubType"`
	TailMark       any    `json:"tailMark"`
	TypeDesc       any    `json:"typeDesc"`
	PrivacySetting int    `json:"privacySetting"`
	QuestionID     any    `json:"questionId"`
	ExtParams      struct {
		PubBizCode string `json:"pubBizCode"`
	} `json:"extParams"`
	VoiceInfo                 any                                         `json:"voiceInfo"`
	CommentVoiceMetaDTO       any                                         `json:"commentVoiceMetaDTO"`
	PointTopicInfo            DailySonSharePublishRespEventPointTopicInfo `json:"pointTopicInfo"`
	ActivityInfos             []any                                       `json:"activityInfos"`
	AnonymityInfo             DailySonSharePublishRespEventAnonymityInfo  `json:"anonymityInfo"`
	TitleAlias                any                                         `json:"titleAlias"`
	SocialUserID              any                                         `json:"socialUserId"`
	SocialSpaceVisible        any                                         `json:"socialSpaceVisible"`
	SrcResID                  any                                         `json:"srcResId"`
	SrcResType                any                                         `json:"srcResType"`
	DistributionType          any                                         `json:"distributionType"`
	RecommendStatus           int                                         `json:"recommendStatus"`
	FirstRecommendTime        int64                                       `json:"firstRecommendTime"`
	MomentScore               any                                         `json:"momentScore"`
	ImageScore                any                                         `json:"imageScore"`
	NoteArtistIDList          any                                         `json:"noteArtistIdList"`
	NoteArtistNameList        any                                         `json:"noteArtistNameList"`
	OperationBindArtistIDs    any                                         `json:"operationBindArtistIds"`
	OperationPositionMap      any                                         `json:"operationPositionMap"`
	NoteIPIDList              any                                         `json:"noteIpIdList"`
	NoteIPNameList            any                                         `json:"noteIpNameList"`
	RiskControlComplainDTO    any                                         `json:"riskControlComplainDTO"`
	PicColorMap               any                                         `json:"picColorMap"`
	RedEnvelopeDTO            any                                         `json:"redEnvelopeDTO"`
	Reward                    any                                         `json:"reward"`
	MustShowEventFeed         bool                                        `json:"mustShowEventFeed"`
	AITitleMap                any                                         `json:"aiTitleMap"`
	WikiAITitleMap            any                                         `json:"wikiAiTitleMap"`
	PubSource                 any                                         `json:"pubSource"`
	TitlePicInfo              any                                         `json:"titlePicInfo"`
	EditTime                  int64                                       `json:"editTime"`
	TitlePicType              any                                         `json:"titlePicType"`
	MultiAIPicInfo            any                                         `json:"multiAiPicInfo"`
	AIPrivatePicInfo          any                                         `json:"aiPrivatePicInfo"`
	SearchExplicitTitle       any                                         `json:"searchExplicitTitle"`
	PushExplicitTitle         any                                         `json:"pushExplicitTitle"`
	QuickDiscoveryAITitle     any                                         `json:"quickDiscoveryAiTitle"`
	ContainAIContent          bool                                        `json:"containAiContent"`
	ContentDeclaration        any                                         `json:"contentDeclaration"`
	RepostSource              any                                         `json:"repostSource"`
	RelateQuery               any                                         `json:"relateQuery"`
	DefaultAIPicInfo          any                                         `json:"defaultAiPicInfo"`
	EventDynamicCoverVideoMap any                                         `json:"eventDynamicCoverVideoMap"`
}

type DailySonSharePublishRespEventPointTopicInfo struct {
	ID                 any  `json:"id"`
	Type               any  `json:"type"`
	SubType            any  `json:"subType"`
	Name               any  `json:"name"`
	Icon               any  `json:"icon"`
	Desc               any  `json:"desc"`
	Target             any  `json:"target"`
	ThroughInfo        any  `json:"throughInfo"`
	Ext                any  `json:"ext"`
	Hot                bool `json:"hot"`
	HotIcon            any  `json:"hotIcon"`
	HotDiscussNumDesc  any  `json:"hotDiscussNumDesc"`
	SquareDesc         any  `json:"squareDesc"`
	MomentTopic        bool `json:"momentTopic"`
	PubGuide           bool `json:"pubGuide"`
	PubGuideIcon       any  `json:"pubGuideIcon"`
	PubGuideText       any  `json:"pubGuideText"`
	PubGuideActionText any  `json:"pubGuideActionText"`
	Parent             any  `json:"parent"`
	Pic                any  `json:"pic"`
	ArtistID           any  `json:"artistId"`
	LogInfo            any  `json:"logInfo"`
}

type DailySonSharePublishRespEventAnonymityInfo struct {
	Anonymous  int `json:"anonymous"`
	Name       any `json:"name"`
	AvatarURL  any `json:"avatarUrl"`
	Me         any `json:"me"`
	LabelIcons any `json:"labelIcons"`
}

// DailySongSharePublish publishes a note or song share for the activity.
func (a *Api) DailySongSharePublish(ctx context.Context, req *DailySongSharePublishReq) (*DailySonSharePublishResp, error) {
	if req == nil {
		return nil, errors.New("daily song share publish request is nil")
	}

	req.AutoSaveDraft = true
	req.UseNewUpload = true
	req.FromRn = true
	req.NeedsGuardianToken = true

	if req.Uuid == "" {
		req.Uuid = uuid.NewString()
	}

	if req.OS == "" {
		req.OS = "android"
	}

	if req.TargetPublishTime == nil {
		req.TargetPublishTime = "-1"
	}

	if req.PublishTime == nil {
		req.PublishTime = "0"
	}

	if req.PrivacySetting == "" {
		req.PrivacySetting = "0"
	}

	if req.SocialSpaceVisible == 0 {
		req.SocialSpaceVisible = 1
	}

	if req.Type == "" {
		req.Type = "noresource"
	}

	if req.Type == "song" && req.Id != "" {
		if req.ThreadId == "" {
			req.ThreadId = "R_SO_4_" + req.Id
		}

		if req.ResourceId == "" {
			req.ResourceId = req.Id
		}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/share/friends/resource"
		reply    DailySonSharePublishResp
		opts     = api.NewOptions("eapi.DailySongSharePublish").
				SetXEAPI().
				SetHeader("CMPageId", "page_songlist").
				SetHeader("MConfig-Info", dailySongShareMConfigInfo)
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share publish: %w", err)
	}
	return &reply, nil
}

// DailySongShareTriggerReq reports a song-sharing trigger.
type DailySongShareTriggerReq struct {
	types.EApiReqCommon

	SongId  string `json:"songId"`
	Channel string `json:"channel"`
}

// DailySongShareTriggerResp is the song-sharing trigger response.
type DailySongShareTriggerResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

// DailySongShareTrigger reports that a song was shared through a channel.
func (a *Api) DailySongShareTrigger(ctx context.Context, req *DailySongShareTriggerReq) (*DailySongShareTriggerResp, error) {
	if req == nil {
		return nil, errors.New("daily song share trigger request is nil")
	}

	if strings.TrimSpace(req.Channel) == "" {
		req.Channel = "cloudmusic"
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/music/song/share/trigger"
		reply    DailySongShareTriggerResp
		opts     = api.NewOptions("eapi.DailySongShareTrigger").SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share trigger: %w", err)
	}
	return &reply, nil
}

// DailySongShareLotteryReq draws a prize from the sharing activity.
type DailySongShareLotteryReq struct {
	types.EApiReqCommon

	ActivityId int64 `json:"activityId"` // 对应 DailySongShareRegistrationGuideResp.Data.ActivityInterestId
}

// DailySongShareLotteryPrizeDetail describes one possible lottery prize.
type DailySongShareLotteryPrizeDetail struct {
	PrizeName    string   `json:"prizeName"`
	WinPrizeDesc string   `json:"winPrizeDesc"`
	PrizeImgList []string `json:"prizeImgList"`
	ExchangeUrl  string   `json:"exchangeUrl"`
	PrizeType    int64    `json:"prizeType"`
	SubType      int64    `json:"subType"`
	ContentId    string   `json:"contentId"`
	DefaultPrize int64    `json:"defaultPrize"`
	PrizeLevel   int64    `json:"prizeLevel"`
}

// DailySongShareLotteryResp TODO: 响应内容不完整需要补充。
type DailySongShareLotteryResp struct {
	Code      int64  `json:"code"` // 457:很遗憾，您本次权益不足，谢谢您的参与 455:活动太火爆，请稍后再试，错误码:507(提示:传入错误id会出现这个)
	Message   string `json:"message"`
	DebugInfo any    `json:"debugInfo,omitempty"`
	FailData  any    `json:"failData,omitempty"`
	Data      struct {
		UserId             int64                                       `json:"userId"`
		BatchIdemKey       any                                         `json:"batchIdemKey"`
		IdempotentId       string                                      `json:"idempotentId"`
		ActivityId         int64                                       `json:"activityId"`
		PrizeSchemeId      int64                                       `json:"prizeSchemeId"`
		DrawPrizeTime      int64                                       `json:"drawPrizeTime"`
		PrizeDetailInfoMap map[string]DailySongShareLotteryPrizeDetail `json:"prizeDetailInfoMap"`
		NoLotteryContent   any                                         `json:"noLotteryContent"`
		RestChance         int64                                       `json:"restChance"`
		CollectDTO         any                                         `json:"collectDTO"`
	} `json:"data"`
}

// DailySongShareLottery draws a prize from the current sharing activity.
func (a *Api) DailySongShareLottery(ctx context.Context, req *DailySongShareLotteryReq) (*DailySongShareLotteryResp, error) {
	if req == nil {
		req = &DailySongShareLotteryReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/middle/play/do/lottery"
		reply    DailySongShareLotteryResp
		opts     = api.NewOptions("eapi.DailySongShareLottery").SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share lottery: %w", err)
	}
	return &reply, nil
}
