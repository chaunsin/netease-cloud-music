// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"context"
	"crypto/aes"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	ncmcookie "github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type signFlowTransport struct {
	responses [][]byte
	paths     []string
}

func (t *signFlowTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, err
	}

	t.paths = append(t.paths, request.URL.Path)
	body := t.responses[len(t.paths)-1]
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}, nil
}

func TestExecuteYunBeiSignWithoutAutomaticOnlySigns(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"sign":false}}`),
	})

	var output bytes.Buffer

	command := &SignIn{cmd: &cobra.Command{}}
	command.cmd.SetOut(&output)

	err := command.executeYunBeiSign(context.Background(), weapi.New(client))
	require.NoError(t, err)
	assert.Equal(t, []string{"/weapi/pointmall/user/sign"}, transport.paths)
	assert.Equal(t, "云贝已签到\n", output.String())
}

func TestExecuteYunBeiSignAutomaticPreservesRewardFlow(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"sign":true}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":3,"baseGrant":{"name":"3云贝"},"baseLotteryId":123},{"signDay":7,"baseGrant":{"name":"未领取"}}]}}`),
		[]byte(`{"code":200,"data":true}`),
		[]byte(`{"code":200,"data":[{"completed":false},{"completed":true,"depositCode":1304,"period":1,"taskName":"分享歌曲","taskPoint":2,"userTaskId":456}]}`),
		[]byte(`{"code":200,"data":true}`),
	})
	logger := log.New(&log.Config{Level: "error"})

	t.Cleanup(func() { require.NoError(t, logger.Close()) })

	var output bytes.Buffer

	command := &SignIn{
		cmd:  &cobra.Command{},
		l:    logger,
		opts: SignInOpts{Automatic: true},
	}
	command.cmd.SetOut(&output)

	err := command.executeYunBeiSign(context.Background(), weapi.New(client))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
		"/weapi/pointmall/user/sign/lottery/get",
		"/weapi/usertool/task/todo/query",
		"/weapi/usertool/task/point/receive",
	}, transport.paths)
	assert.Equal(t, "云贝签到成功\n"+
		"云贝连续签到天数=3,奖励内容=3云贝 领取成功\n"+
		"云贝 [分享歌曲] 任务完成获得云贝数量 2\n", output.String())
}

func TestExecuteVipSignAtMaxLevelSkipsReward(t *testing.T) {
	responses := append(successfulVipSignResponses(t, true), []byte(`{"code":200}`))
	client, transport := newSignFlowClient(t, responses)

	var output bytes.Buffer

	command := &SignIn{
		cmd:  &cobra.Command{},
		opts: SignInOpts{Automatic: true},
	}
	command.cmd.SetOut(&output)

	err := command.executeVipSign(
		context.Background(),
		weapi.New(client),
		eapi.New(client),
		1785913200098,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/weapi/vipnewcenter/app/level/growhpoint/basic",
		"/eapi/vip-center-bff/task/sign",
		"/eapi/vipnewcenter/app/level/user/checkin/history/detail",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/weapi/login/token/refresh",
	}, transport.paths)
	assert.Equal(t, "VIP 等级: 已满级\n"+
		"黑胶乐签: 成功\n"+
		"  今日歌曲: Locked Out of Heaven - Bruno Mars\n"+
		"  8月黑胶乐签: 已签 3 天, 再打卡4天得3天高清臻音\n", output.String())
}

func TestExecuteVipSignClaimsRewardAfterFlow(t *testing.T) {
	responses := append(
		successfulVipSignResponses(t, false),
		[]byte(`{"code":200,"data":{"result":true}}`),
		[]byte(`{"code":200}`),
	)
	client, transport := newSignFlowClient(t, responses)

	var output bytes.Buffer

	command := &SignIn{
		cmd:  &cobra.Command{},
		opts: SignInOpts{Automatic: true},
	}
	command.cmd.SetOut(&output)

	err := command.executeVipSign(
		context.Background(),
		weapi.New(client),
		eapi.New(client),
		1785913200098,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/weapi/vipnewcenter/app/level/growhpoint/basic",
		"/eapi/vip-center-bff/task/sign",
		"/eapi/vipnewcenter/app/level/user/checkin/history/detail",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/weapi/vipnewcenter/app/level/task/reward/getall",
		"/weapi/login/token/refresh",
	}, transport.paths)
	assert.Equal(t, "黑胶乐签: 成功\n"+
		"  今日歌曲: Locked Out of Heaven - Bruno Mars\n"+
		"  8月黑胶乐签: 已签 3 天, 再打卡4天得3天高清臻音\n"+
		"VIP 成长值: 领取成功\n", output.String())
}

func TestExecuteVipSignPrintsServerMessageWhenNotCompleted(t *testing.T) {
	responses := successfulVipSignResponses(t, false)
	responses[1] = encryptSignTestResponse(t, []byte(`{"code":200,"data":false,"message":"今日已乐签"}`))
	responses = append(responses, []byte(`{"code":200}`))
	client, _ := newSignFlowClient(t, responses)

	var output bytes.Buffer

	command := &SignIn{cmd: &cobra.Command{}}
	command.cmd.SetOut(&output)

	err := command.executeVipSign(context.Background(), weapi.New(client), eapi.New(client), 1785913200098)
	require.NoError(t, err)
	assert.Equal(t, "黑胶乐签: 今日已乐签\n"+
		"  今日歌曲: Locked Out of Heaven - Bruno Mars\n"+
		"  8月黑胶乐签: 已签 3 天, 再打卡4天得3天高清臻音\n", output.String())
}

func TestExecuteVipSignStopsWhenDetailRefreshFails(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		vipGrowPointResponse(false),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":true,"message":""}`)),
		encryptSignTestResponse(t, []byte(`{"code":500,"data":{},"message":"failed"}`)),
	})
	command := &SignIn{cmd: &cobra.Command{}}

	err := command.executeVipSign(context.Background(), weapi.New(client), eapi.New(client), 1785913200098)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VipCheckinHistoryDetail after VipTaskSign")
	assert.Equal(t, []string{
		"/weapi/vipnewcenter/app/level/growhpoint/basic",
		"/eapi/vip-center-bff/task/sign",
		"/eapi/vipnewcenter/app/level/user/checkin/history/detail",
	}, transport.paths)
}

func TestExecuteVipSignWithoutVipContinuesSignFlow(t *testing.T) {
	responses := successfulVipSignResponses(t, false)
	responses[0] = []byte(`{"code":200,"data":{"userLevel":{"latestVipStatus":0}}}`)
	responses = append(responses, []byte(`{"code":200}`))
	client, transport := newSignFlowClient(t, responses)

	var output bytes.Buffer

	command := &SignIn{
		cmd:  &cobra.Command{},
		opts: SignInOpts{Automatic: true},
	}
	command.cmd.SetOut(&output)

	err := command.executeVipSign(context.Background(), weapi.New(client), eapi.New(client), 1785913200098)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/weapi/vipnewcenter/app/level/growhpoint/basic",
		"/eapi/vip-center-bff/task/sign",
		"/eapi/vipnewcenter/app/level/user/checkin/history/detail",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/weapi/login/token/refresh",
	}, transport.paths)
	assert.Equal(t, "VIP 权益: 暂无, 仅执行乐签\n"+
		"黑胶乐签: 成功\n"+
		"  今日歌曲: Locked Out of Heaven - Bruno Mars\n"+
		"  8月黑胶乐签: 已签 3 天, 再打卡4天得3天高清臻音\n", output.String())
}

func newSignFlowClient(t *testing.T, responses [][]byte) (*api.Client, *signFlowTransport) {
	t.Helper()

	home := t.TempDir()
	logger := log.New(&log.Config{Level: "error"})
	client, err := api.NewClient(&api.Config{
		Timeout: time.Second,
		HomeDir: home,
		Cookie: ncmcookie.Config{
			Filepath: filepath.Join(home, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	})

	transport := &signFlowTransport{responses: responses}
	client.GetClient().Transport = transport
	return client, transport
}

func successfulVipSignResponses(t *testing.T, maxLevel bool) [][]byte {
	t.Helper()

	return [][]byte{
		vipGrowPointResponse(maxLevel),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":true,"message":""}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":{"songInfo":{"songName":"Locked Out of Heaven","artistName":"Bruno Mars"},"monthCheckInTotalDay":3},"message":""}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":{"text":null,"subText":"黑胶乐签 再打卡4天有惊喜"},"message":""}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":{"text":"8月黑胶乐签","subText":"再打卡4天得3天高清臻音"},"message":""}`)),
	}
}

func vipGrowPointResponse(maxLevel bool) []byte {
	if maxLevel {
		return []byte(`{"code":200,"data":{"userLevel":{"latestVipStatus":1,"maxLevel":true}}}`)
	}
	return []byte(`{"code":200,"data":{"userLevel":{"latestVipStatus":1,"maxLevel":false}}}`)
}

func encryptSignTestResponse(t *testing.T, plaintext []byte) []byte {
	t.Helper()

	block, err := aes.NewCipher([]byte("e82ckenh8dichen8"))
	require.NoError(t, err)

	padding := block.BlockSize() - len(plaintext)%block.BlockSize()
	padded := append(append([]byte(nil), plaintext...), bytes.Repeat([]byte{byte(padding)}, padding)...)

	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], padded[i:i+block.BlockSize()])
	}
	return ciphertext
}
