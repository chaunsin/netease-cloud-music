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

package crypto

import (
	"crypto/md5"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomKey(t *testing.T) {
	text := randomKey()
	assert.NotEmpty(t, text)
	t.Logf("GenerateRandomKey: %s\n", text) // ONRhfKsUOHoF8iVd
}

func TestReverseString(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "正常",
			args: args{
				str: "123456789",
			},
			want: "987654321",
		},
		{
			name: "回文",
			args: args{
				str: "123321",
			},
			want: "123321",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, reverseString(tt.args.str), "ReverseString(%v)", tt.args.str)
		})
	}
}

func TestWeApiEncrypt(t *testing.T) {
	type args struct {
		object interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "正常",
			args: args{
				object: map[string]string{"phone": "188********", "ctcode": "86"},
			},
			want:    map[string]string{},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WeApiEncrypt(tt.args.object)
			assert.NoError(t, err)
			t.Logf("data:%+v\n", got)
			// assert.Equalf(t, tt.want, got, "weapi(%v)", tt.args.object)
		})
	}
}

func TestEApiDecrypt(t *testing.T) {
	type args struct {
		encode string
		data   string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test request",
			args: args{
				encode: "hex",
				data:   "1BDAA66BB859333CCCE0A53AE6D1E6E61F5C1663DE05CFFB8C87BCE2FDC6F9ECAB1F5341B2FBCB5CBBACDA665D6F1A10B007189F44A13DB2463BB3EBF2639CF10A3E14D47E97975942FF626F17CE4A658E17F19C52EDACCB199F262EA09723E644C46E3880B4754AE1A2A1F4712268C52AEA6F5D0158780D82BDC30C930756181972480BE18A2ECD68A276C68E5214491F2323B3C87ECA2AF9532A4F483D55B8C5187D558AF5699D2C2437C1D98CB5AD7B90402CCDB12DF950521A86D854646BF8422708A649C1B8B752AF70AD5B3868F939FD0E9BEAA8BAE0D05BB0D4D88BE1A6BFAA8F5BBECD6F92368480E657D2200F8ACE7740ACAAA5634297D6661704EE7F74779E833DF2241939FC60C5D92569E31285E4F4A4F737CC8E89316DE7BBC8FB99E94B87DC05C190EA228637B2C0D182152BFAC603EF671A9A0B2F907D98F30E8A4614F236B3ED78392F039EDAD3C3CE5A856EE51BCDE2173F428CD1BB0239",
			},
			want: "/api/music/partner/work/evaluate-36cd479b6b5-{\"taskId\":\"185640294\",\"workId\":\"1312207\",\"score\":\"3\",\"tags\":\"3-C-1\",\"customTags\":\"[]\",\"comment\":\"\",\"extraResource\":\"true\",\"syncYunCircle\":\"false\",\"syncComment\":\"true\",\"extraScore\":\"{\\\"1\\\":3,\\\"2\\\":2,\\\"3\\\":4}\",\"source\":\"mp-music-partner\",\"header\":\"{}\",\"e_r\":true}-36cd479b6b5-f891fb9aa53a9b84280a53c43ff84de8",
			wantErr: func(t assert.TestingT, err error, msg ...interface{}) bool {
				if err != nil {
					t.Errorf("%s", msg)
					return false
				}
				return true
			},
		},
		{
			name: "test response",
			args: args{
				encode: "hex",
				data:   "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08",
			},
			want: `{"code":200,"data":true}`,
			wantErr: func(t assert.TestingT, err error, msg ...interface{}) bool {
				if err != nil {
					t.Errorf("%s", msg)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EApiDecrypt(tt.args.data, "hex")
			if !tt.wantErr(t, err, fmt.Sprintf("EApiDecrypt(%v)", tt.args.data)) {
				return
			}
			t.Logf("data: %+v\n", string(got))
			t.Logf("abcd: %+v\n", tt.want)
			assert.Equalf(t, tt.want, string(got), "EApiDecrypt(%v)", tt.args.data)
		})
	}
}

func TestEApiEncrypt(t *testing.T) {
	type args struct {
		url    string
		object interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EApiEncrypt(tt.args.url, tt.args.object)
			if !tt.wantErr(t, err, fmt.Sprintf("EApiEncrypt(%v, %v)", tt.args.url, tt.args.object)) {
				return
			}
			assert.Equalf(t, tt.want, got, "EApiEncrypt(%v, %v)", tt.args.url, tt.args.object)
		})
	}
}

func TestHex(t *testing.T) {
	var (
		url    = "/api/sms/captcha/sent"
		data   = `{"deviceId":"4cdb39bf34a848781b89663e1e546b8b","os":"OSX","cellphone":"188********","header":"{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\"IuRPVVmc3WWul9fT\\\":{\\\"version\\\":143360,\\\"appver\\\":\\\"2.3.17\\\"}}\",\"MG-Product-Name\":\"music\"}","ctcode":"86","verifyId":1,"e_r":true}`
		target = `/api/sms/captcha/sent-36cd479b6b5-{"deviceId":"4cdb39bf34a848781b89663e1e546b8b","os":"OSX","cellphone":"188********","header":"{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\"IuRPVVmc3WWul9fT\\\":{\\\"version\\\":143360,\\\"appver\\\":\\\"2.3.17\\\"}}\",\"MG-Product-Name\":\"music\"}","ctcode":"86","verifyId":1,"e_r":true}-36cd479b6b5-6712bc8cd675b8e4059289b0b56abcbe`
	)
	message := fmt.Sprintf("nobody%suse%smd5forencrypt", url, data)
	digest := fmt.Sprintf("%x", md5.Sum([]byte(message)))
	text := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%s", url, data, digest)
	fmt.Printf("text: %s\n", text)
	fmt.Printf("equal: %v\n", text == target)
}

// func Test_digest(t *testing.T) {
// 	var (
// 		sendSMSReq = api.SendSMSReq{
// 			CtCode:    "86",
// 			Cellphone: "188********",
// 			DeviceId:  "4cdb39bf34a848781b89663e1e546b8b",
// 			Os:        "OSX",
// 			VerifyId:  1,
// 			ER:        true,
// 		}
// 		header = api.SendSMSReqHeader{
// 			Os:            "osx",
// 			AppVer:        "2.3.17",
// 			DeviceId:      "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B",
// 			RequestId:     "93487028",
// 			ClientSign:    "",
// 			OsVer:         "%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89",
// 			NmGCoreStatus: "1",
// 			MConfigInfo:   `{"IuRPVVmc3WWul9fT":{"version":143360,"appver":"2.3.17"}}`,
// 			MGProductName: "music",
// 		}
// 		url = "/api/sms/captcha/sent"
// 	)
// 	headerByte, err := json.Marshal(header)
// 	if err != nil {
// 		t.Fatalf("json: %s", err)
// 	}
// 	sendSMSReq.Header = string(headerByte)
// 	payload, err := json.Marshal(sendSMSReq)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Logf("json: %s", string(payload))
// 	t.Logf("digest: %s\n", digest(url, string(payload)))
// }

func TestGetCacheKey(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: "args123",
			want: "RFKLrid1HPwKv4hPWldxJA==",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CacheKeyEncrypt(tt.args)
			if !tt.wantErr(t, err, fmt.Sprintf("CacheKeyEncrypt(%v)", tt.args)) {
				return
			}
			t.Logf("data: %+v\n", got)
			assert.Equalf(t, tt.want, got, "CacheKeyEncrypt(%v)", tt.args)
		})
	}
}

func TestCacheKeyDecrypt(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "sample",
			args: "RFKLrid1HPwKv4hPWldxJA==",
			want: "args123",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return true
				}
				return false
			},
		},
		{
			name: "真实参数",
			args: "0cjs/PeKn8i8GZDV84eJ5IqRq/RX1Hok5Oyt1+2iwcgHfZVdOn+GbulSnnhB4gmf",
			want: "e_r=false&id=10171989900&n=3&s=0",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				if err != nil {
					t.Errorf("err: %v args: %s", err, i)
					return false
				}
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CacheKeyDecrypt(tt.args)
			if !tt.wantErr(t, err, fmt.Sprintf("CacheKeyDecrypt(%v)", tt.args)) {
				return
			}
			t.Logf("got: %+v\n", got)
			assert.Equalf(t, tt.want, got, "CacheKeyDecrypt(%v)", tt.args)
		})
	}
}
