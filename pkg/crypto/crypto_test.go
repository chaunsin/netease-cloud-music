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
			name: "",
			args: args{
				object: map[string]string{"phone": "188********", "ctcode": "86"},
			},
			want:    nil,
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
			name: "test1",
			args: args{
				encode: "hex",
				data:   "7FBF4FE377777244A3D313B184D84F8941EE1778F2B44810DAD85F1F8B8A46EF184EDD7948FBEDDB27E75B828A8AE6BBFE2BD0C267A13A275D76EFF0E65737C3609159C29A2355FE00CF82EF0767F4CD0772E4D3574DEA27888B0FB8896A5036B0458C73077A26075E969802234A217EF39CEB3B26DBF89C3A11884BDA07AC84B049BED3EBE48B44EDD18059B4729A5A1BF57BB93FAD866F623CFD9DE305CCB433C6786E963C82C72294E2A5E547A1F7B799919C91BF9C4CAF1EAE5A6EAF1E1F088A96F790CB77E4B43C944C8ACA59293F1E32CF0BD8CC04459BFA592D3E090AE3C06FCA7B536883DDCC7A3C7DCDB60E7F47E1B5D858AEEC15A98109793AB18F5C01318CE3CDF75C4E338DD84574D22A0ABD6005FE0F48ED060E3340691B5FDDC506BADB478CFE3CB82C41FFB953BCBD57361A7B3E5350A5E3D8843561377FFF338C1CF70FEB2434BB12CD18E0EF291CEE75DFFA68E700F393B8919EECC71C8C7F981D7034EFD607C90EBCA4532C9A567976C3AE9E8B9A500C7632E312399589AF63CD2E94EE8CD7B84D10B256E9F94A63DDA55E4002ABDA38DB02003187C60F00AD20CEAF107D9E6A69EDCEA227FE894E4AC27ACF86AB5B4682DA89DEF2295FA6574E46EBE916862881C844BF498CE1800B4638AF58BF67588F5807A992FB94394D9D38610A8D83E81F0C824DC64AB3695539FD273D5D7B03D39226E96FB36C28678B4293909E128BB1941A6CD7D7FFBF1D0732709A948421A8E25ECAC9D5A73220D3E7620DD2592D83DE90C2D9A8F014F8A82459FFC0408A2F1B448F606E3B42A987E3464540E1DE6E4B3B414165F20E7B29DA724028600BF1EF1B2BDC3E53D7D9120418D51E69C66D806DBAAD9A254B8E525C928B604347843F3E053EAB50",
			},
			want: `/api/sms/captcha/sent-36cd479b6b5-{"deviceId":"4cdb39bf34a848781b89663e1e546b8b","os":"OSX","cellphone":"18846766926","header":"{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\"IuRPVVmc3WWul9fT\\\":{\\\"version\\\":143360,\\\"appver\\\":\\\"2.3.17\\\"}}\",\"MG-Product-Name\":\"music\"}","ctcode":"86","verifyId":1,"e_r":true}-36cd479b6b5-6712bc8cd675b8e4059289b0b56abcbe`,
			wantErr: func(t assert.TestingT, err error, msg ...interface{}) bool {
				if err != nil {
					t.Errorf("%s", msg)
					return false
				}
				return true
			},
		},
		{
			name: "test2",
			args: args{
				encode: "hex",
				data:   "7FBF4FE377777244A3D313B184D84F8941EE1778F2B44810DAD85F1F8B8A46EF184EDD7948FBEDDB27E75B828A8AE6BBFE2BD0C267A13A275D76EFF0E65737C3609159C29A2355FE00CF82EF0767F4CD0772E4D3574DEA27888B0FB8896A5036B0458C73077A26075E969802234A217EF39CEB3B26DBF89C3A11884BDA07AC84B049BED3EBE48B44EDD18059B4729A5A1BF57BB93FAD866F623CFD9DE305CCB433C6786E963C82C72294E2A5E547A1F7B799919C91BF9C4CAF1EAE5A6EAF1E1F088A96F790CB77E4B43C944C8ACA59293F1E32CF0BD8CC04459BFA592D3E090AE3C06FCA7B536883DDCC7A3C7DCDB60E7F47E1B5D858AEEC15A98109793AB18F5C01318CE3CDF75C4E338DD84574D22A0ABD6005FE0F48ED060E3340691B5FDDC506BADB478CFE3CB82C41FFB953BCBD57361A7B3E5350A5E3D8843561377FFF338C1CF70FEB2434BB12CD18E0EF291CEE75DFFA68E700F393B8919EECC71C8C7F981D7034EFD607C90EBCA4532C9A567976C3AE9E8B9A500C7632E312399589AF63CD2E94EE8CD7B84D10B256E9F94A63DDA55E4002ABDA38DB02003187C60F00AD20CEAF107D9E6A69EDCEA227FE894E4AC27ACF86AB5B4682DA89DEF2295FA6574E46EBE916862881C844BF498CE1800B4638AF58BF67588F5807A992FB94394D9D38610A8D83E81F0C824DC64AB3BB78759C081321BD8F0AC96B781A690128678B4293909E128BB1941A6CD7D7FFBF1D0732709A948421A8E25ECAC9D5A73220D3E7620DD2592D83DE90C2D9A8F014F8A82459FFC0408A2F1B448F606E3B42A987E3464540E1DE6E4B3B414165F2BEB4F0465EB5AD5CB522A2E120C18D3CF7A81C3646A87C8AD842C75F5AD2678123642A1428D7F5E002601E3D193C59C2",
			},
			want: `/api/sms/captcha/sent-36cd479b6b5-{"deviceId":"4cdb39bf34a848781b89663e1e546b8b","os":"OSX","cellphone":"188********","header":"{\"os\":\"osx\",\"appver\":\"2.3.17\",\"deviceId\":\"7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B\",\"requestId\":\"93487028\",\"clientSign\":\"\",\"osver\":\"%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89\",\"Nm-GCore-Status\":\"1\",\"MConfig-Info\":\"{\\\"IuRPVVmc3WWul9fT\\\":{\\\"version\\\":143360,\\\"appver\\\":\\\"2.3 17\\\"}}\",\"MG-Product-Name\":\"music\"}","ctcode":"86","verifyId":1,"e_r":true}-36cd479b6b5-64b33cf9c2023bca3d9ecce5fef8f493`,
			wantErr: func(t assert.TestingT, err error, msg ...interface{}) bool {
				if err != nil {
					t.Errorf("%s", msg)
					return false
				}
				return true
			},
		},
		{
			name: "test3",
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
