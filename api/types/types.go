package types

import (
	"encoding/json"
)

// ReqCommon weapi通用请求字段
type ReqCommon struct {
	CSRFToken string `json:"csrf_token,omitempty"`
}

// RespCommon weapi通用返回字段
type RespCommon[T any] struct {
	Code    int64  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Msg     string `json:"msg,omitempty"`
	Data    T      `json:"data,omitempty"`
}

// ApiRespCommon api接口通用返回结构
type ApiRespCommon[T any] struct {
	Code      int64       `json:"code,omitempty"`
	Message   string      `json:"message,omitempty"`
	Msg       string      `json:"msg,omitempty"`
	DebugInfo interface{} `json:"debugInfo,omitempty"`
	FailData  interface{} `json:"failData,omitempty"`
	Data      T           `json:"data,omitempty"`
}

// // SendSMSReq 暂定此结构
// //
// //	{
// //	 "deviceId": "4cdb39bf34a848781b89663e1e546789",
// //	 "os": "OSX",
// //	 "cellphone": "188****8888",
// //	 "header": "{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\\\"IuRPVVmc3WWul9fT\\\\\":{\\\\\"version\\\\\":143360,\\\\\"appver\\\\\":\\\\\"2.3.17\\\\\"}}\",\"MG-Product-Name\":\"music\"}",
// //	 "ctcode": "86",
// //	 "verifyId": 1,
// //	 "e_r": true
// //	}
// type SendSMSReq struct {
// 	DeviceId  string `json:"deviceId"`  // 设备id 格式:4cdb39bf34a848781b89663e1e546789 可参考:https://github.com/mos9527/pyncm/blob/master/pyncm/utils/constant.py todo:哪里获得？
// 	Os        string `json:"os"`        // 系统 OSX
// 	Cellphone string `json:"cellphone"` // 手机号
// 	Header    string `json:"header"`    // Header
// 	CtCode    string `json:"ctcode"`    // 国家码
// 	VerifyId  int64    `json:"verifyId"`  //
// 	ER        bool   `json:"e_r"`       // 控制相应返回值是否加密，true为加密，false为明文。
// }
//
// // SendSMSReqHeader .
// //
// //	{
// //		"os": "osx",
// //		"appver": "2.3.17",
// //		"deviceId": "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B",
// //		"requestId": "93487028",
// //		"clientSign": "",
// //		"osver": "%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89",
// //		"Nm-GCore-Status": "1",
// //		"MConfig-Info": "{\\"IuRPVVmc3WWul9fT\\":{\\"version\\":143360,\\"appver\\":\\"2.3 17\\"}}",
// //		"MG-Product-Name": "music"
// //	}
// type SendSMSReqHeader struct {
// 	Os            string `json:"os"`              // 系统 OSX
// 	AppVer        string `json:"appver"`          // 应用版本2.3.17 如果是pc mac此内容可以在设置中找到
// 	DeviceId      string `json:"deviceId"`        // 设备id mac苹果中得硬件UUID或者预置UDID,它俩值可能一样,另外此设备id是两个id拼接,中间用|分隔
// 	RequestId     string `json:"requestId"`       // 格式:93487028
// 	ClientSign    string `json:"clientSign"`      // todo: 何时为空
// 	OsVer         string `json:"osver"`           // 系统版本，采用url编码内容:%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89 解码后原内容为: 版本12.6（版本21G115）
// 	NmGCoreStatus string `json:"Nm-GCore-Status"` // 1 todo: 何时为1 1是否是死值
// 	MConfigInfo   string `json:"MConfig-Info"`    // MConfigInfo 貌似是写死得 {"IuRPVVmc3WWul9fT":{"version":143360,"appver":"2.3.17"}} 请参考:https://github.com/Zifeiyu-0/Script/blob/73bfe9608bdd086eca2f58befdcb71cd2bb64093/QX/wyymusic.js#L23
// 	MGProductName string `json:"MG-Product-Name"` // 猜测是产品名字，死值:music
// }
//
// // MConfigInfo .
// type MConfigInfo struct {
// 	IuRPVVmc3WWul9FT struct {
// 		Version int64    `json:"version"` // 143360
// 		Appver  string `json:"appver"`  // 同 SendSMSReqHeader.AppVer 格式:2.3.17
// 	} `json:"IuRPVVmc3WWul9fT"`
// }

type (
	IntsString []int64
	intsString []int64
)

func (i IntsString) MarshalJSON() ([]byte, error) {
	var ii intsString
	for _, v := range i {
		ii = append(ii, v)
	}
	data, err := json.Marshal(ii)
	if err != nil {
		return nil, err
	}
	return []byte("\"" + string(data) + "\""), nil
}
