// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
)

// fansgroup_test.go 覆盖 SPEC 9.1/9.2/9.3 中可离线验证的部分:
// 纯函数黄金向量、校验分支、聚合退出码与 task 选择器。
// 依赖真实账号的编排验证 (live) 需 NCMCTL_RUN_LIVE_TESTS=1 且不在本文件范围。

func TestParseMissionParamsPlay(t *testing.T) {
	// 播放任务黄金向量: songIds 数组 + 单值字段, 合并顺序 SongIDs→SongID→TrackIDs→TrackID
	p, err := parseMissionParams(`{"songIds":["111","222"],"songId":"333"}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"111", "222", "333"}, mergeSongIDs(p))

	// primary 为空时回退 fallback (播放任务的 IconUi.TargetUrl)
	p, err = parseMissionParams("", `{"trackIds":["444"],"trackId":"555"}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"444", "555"}, mergeSongIDs(p))

	// primary 优先于 fallback
	p, err = parseMissionParams(`{"songId":"1"}`, `{"songId":"2"}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, mergeSongIDs(p))

	// 数字形态: songId/songIds 均可为 number
	p, err = parseMissionParams(`{"songIds":[111,222],"songId":333}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"111", "222", "333"}, mergeSongIDs(p))

	// 线上真实播放任务载荷 (2026-09-04 日志解密原文): songIds 嵌套在 actionMnbParams 内,
	// 顶层 songId 为空。修复前该载荷 mergeSongIDs 返回空, 触发 runPlayMission 提前
	// taskFailed("任务参数中无可用歌曲ID"), playOnce 从未执行。
	realPlay := `{"actionType":"mnb","actionMnbName":"nm.play.playSongs",` +
		`"actionMnbParams":{"songIndex":0,"songIds":[3372978601,3357688069],` +
		`"playParams":{"playerType":"music","showUI":"true"}}}`
	p, err = parseMissionParams(realPlay, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"3372978601", "3357688069"}, mergeSongIDs(p))

	// 顶层 + 嵌套混合: 顶层优先于嵌套
	mixed := `{"songIds":["11"],"actionMnbParams":{"songIds":["22","33"]}}`
	p, err = parseMissionParams(mixed, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"11", "22", "33"}, mergeSongIDs(p))
}

func TestMergeSongIDsNested(t *testing.T) {
	// 嵌套为空时退化为纯顶层行为
	p, err := parseMissionParams(`{"songId":"1","songIds":["2"]}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"2", "1"}, mergeSongIDs(p))

	// 仅嵌套有值 (播放任务真实情形), 数字/字符串混发
	p, err = parseMissionParams(`{"actionMnbParams":{"songIds":[111,"222"],"songId":333}}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"111", "222", "333"}, mergeSongIDs(p))

	// 嵌套 trackId(s) 也并入
	p, err = parseMissionParams(`{"actionMnbParams":{"trackIds":["9"],"trackId":"8"}}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"9", "8"}, mergeSongIDs(p))

	// 嵌套内 null 项丢弃
	p, err = parseMissionParams(`{"actionMnbParams":{"songIds":["1",null],"songId":null}}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, mergeSongIDs(p))

	// 顶层与嵌套都为空
	p, err = parseMissionParams(`{"actionMnbParams":{}}`, "")
	require.NoError(t, err)
	assert.Empty(t, mergeSongIDs(p))
}

func TestIsDuplicateEventID(t *testing.T) {
	rt := &fansGroupRuntime{eventIDs: []int64{37891477077}}
	assert.True(t, isDuplicateEventID(rt, 37891477077))         // 已存在 → 服务端去重
	assert.False(t, isDuplicateEventID(rt, 37891477078))        // 新动态ID
	assert.False(t, isDuplicateEventID(rt, 0))                  // id<=0 属"成功但无ID"分支
	assert.False(t, isDuplicateEventID(rt, -1))                 // 负数同理
	assert.False(t, isDuplicateEventID(&fansGroupRuntime{}, 1)) // 空执行链
}

func TestParseMissionParamsShare(t *testing.T) {
	// 分享任务黄金向量: actionCustomParams.progressParams
	p, err := parseMissionParams(`{"actionCustomParams":{"progressParams":{"resourceId":"666","resourceType":"4"}}}`, "")
	require.NoError(t, err)
	assert.Equal(t, "666", string(p.ActionCustomParams.ProgressParams.ResourceID))
	assert.Equal(t, "4", string(p.ActionCustomParams.ProgressParams.ResourceType))

	// 线上真实载荷: resourceId 为字符串, resourceType 与 songId 为数字, fansGroupId 为 null。
	// 三者混发曾让 resourceType 声明为 string 时整个任务解析失败。
	raw := `{"actionType":"custom","actionCustomName":"SHARE_SONG_EVENT","actionCustomParams":` +
		`{"progressParams":{"resourceId":"3357361025","resourceType":4,"action":"share","fansGroupId":null}},` +
		`"songId":3357361025}`
	p, err = parseMissionParams(raw, "")
	require.NoError(t, err)
	assert.Equal(t, "3357361025", string(p.ActionCustomParams.ProgressParams.ResourceID))
	assert.Equal(t, "4", string(p.ActionCustomParams.ProgressParams.ResourceType))
	assert.Equal(t, []string{"3357361025"}, mergeSongIDs(p))
}

func TestFlexString(t *testing.T) {
	// 字符串 / 数字 / null 均可落到 flexString, 非标量快速失败
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{`"123"`, "123"},
		{`123`, "123"},
		{`0`, "0"},
		{`-1`, "-1"},
		{`"a\"b"`, `a"b`}, // 转义序列需还原
		{`null`, ""},
		{`"null"`, "null"}, // 字符串 "null" 不应被当成 null
	} {
		var v flexString
		require.NoError(t, json.Unmarshal([]byte(tc.raw), &v), "raw=%s", tc.raw)
		assert.Equal(t, tc.want, string(v), "raw=%s", tc.raw)
	}

	for _, raw := range []string{`{"a":1}`, `[1]`, `true`} {
		var v flexString
		require.Error(t, json.Unmarshal([]byte(raw), &v), "raw=%s", raw)
	}

	// 数组元素同样逐项生效
	var ids []flexString
	require.NoError(t, json.Unmarshal([]byte(`["a",1,null]`), &ids))
	assert.Equal(t, []flexString{"a", "1", ""}, ids)
}

func TestParseMissionParamsEdgeCases(t *testing.T) {
	// 字段全空: 解析成功但取值为空
	p, err := parseMissionParams("{}", "")
	require.NoError(t, err)
	assert.Empty(t, mergeSongIDs(p))
	assert.Empty(t, p.ActionCustomParams.ProgressParams.ResourceID)

	// 双空输入
	_, err = parseMissionParams("  ", "")
	require.ErrorContains(t, err, "empty")

	// 畸形 JSON: 错误信息包含原文片段, 不做正则兜底 (D8)
	_, err = parseMissionParams(`{"songIds":oops}`, "")
	require.ErrorContains(t, err, "oops")
	require.ErrorContains(t, err, "songIds")

	// 非标量值: 报错而非静默接受 (数字/字符串/null 之外的类型不兜底)
	_, err = parseMissionParams(`{"songId":{"nested":1}}`, "")
	require.ErrorContains(t, err, "flexString")

	// 数组内字符串与数字混发
	p, err = parseMissionParams(`{"songIds":["1",2,null]}`, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, mergeSongIDs(p))
}

func TestSpeedUpSongIDs(t *testing.T) {
	var m eapi.FansGroupMissionAllRespDataNormalData

	m.Button.Url = `{"songId":"111"}`
	assert.Equal(t, []string{"111"}, speedUpSongIDs(&m, nil))

	m.Button.Url = ""
	m.LogInfo = `{"trackIds":["222"]}`
	assert.Equal(t, []string{"222"}, speedUpSongIDs(&m, nil))

	m.LogInfo = ""
	m.MissionDetail = map[string]any{"songIds": []any{"333"}}
	assert.Equal(t, []string{"333"}, speedUpSongIDs(&m, nil))

	m.MissionDetail = `{"songId":"444"}`
	assert.Equal(t, []string{"444"}, speedUpSongIDs(&m, nil))

	// 各来源解析失败时回退 normal 播放任务累积的歌曲 ID (5.1.7)
	m.MissionDetail = nil
	m.LogInfo = "{bad"
	assert.Equal(t, []string{"555"}, speedUpSongIDs(&m, []string{"555"}))

	// 全空且无回退 → 空 (AC-022: 跳过而非硬编码)
	empty := eapi.FansGroupMissionAllRespDataNormalData{}
	assert.Empty(t, speedUpSongIDs(&empty, nil))
}

func TestMissionCompletedAndRemaining(t *testing.T) {
	// 完成判定以服务端为准 (FR-02/FR-03)
	assert.True(t, missionCompleted("COMPLETED", 0, 3))
	assert.True(t, missionCompleted("PROCESSING", 3, 3))
	assert.False(t, missionCompleted("INIT", 1, 3))
	assert.False(t, missionCompleted("PROCESSING", 2, 0)) // all==0 时只看状态

	// 剩余次数 = AllProgress - CurrentProgress, <=0 时按 1 次处理
	assert.Equal(t, 2, missionRemaining(1, 3))
	assert.Equal(t, 1, missionRemaining(0, 0))
	assert.Equal(t, 1, missionRemaining(3, 3))
}

func TestFansGroupNoteText(t *testing.T) {
	c := NewFansGroup(&Root{}, nil)

	title, message := c.noteText("周杰伦")
	assert.Equal(t, "周杰伦 | 今日乐迷团打卡", title)

	// 内置默认正文: 长度 >=10 Unicode 字符且带随机编号 (两次不相等)
	assert.GreaterOrEqual(t, len([]rune(message)), 10)
	assert.NotEqual(t, message, func() string { _, m := c.noteText("周杰伦"); return m }())

	title, _ = c.noteText("")
	assert.Equal(t, "今日乐迷团打卡", title)

	// --title/--message 优先 (TrimSpace 后)
	c.opts.Title = "  自定义标题  "
	c.opts.Message = "自定义正文内容"
	title, message = c.noteText("周杰伦")
	assert.Equal(t, "自定义标题", title)
	assert.Equal(t, "自定义正文内容", message)
}

func TestRunIterations(t *testing.T) {
	// 全部成功 → done
	r, err := runIterations(context.Background(), "t", 3, 0, 0, func(int, int) (bool, error) { return true, nil })
	require.NoError(t, err)
	assert.Equal(t, taskDone, r.Status)

	// 部分成功 → partial; round 下标与累计成功数按预期传入
	var rounds []int

	r, err = runIterations(context.Background(), "t", 3, 0, 0, func(round, ok int) (bool, error) {
		rounds = append(rounds, round*10+ok)
		return round != 1, nil // 第 2 轮失败
	})
	require.NoError(t, err)
	assert.Equal(t, taskPartial, r.Status)
	assert.Equal(t, []int{0, 11, 21}, rounds)

	// 全部失败 → failed
	r, err = runIterations(context.Background(), "t", 2, 0, 0, func(int, int) (bool, error) { return false, nil })
	require.NoError(t, err)
	assert.Equal(t, taskFailed, r.Status)

	// 致命错误 (ctx 取消) → 透传 error 立即中止, 结果为零值
	ctx, cancel := context.WithCancel(context.Background())
	r, err = runIterations(ctx, "t", 5, 0, 0, func(round, _ int) (bool, error) {
		if round == 1 {
			cancel()
		}
		return true, nil
	})
	assert.Empty(t, r)
	require.ErrorIs(t, err, context.Canceled)
}

func TestFetchAudioSampleInvalidLimit(t *testing.T) {
	// limit<=0 直接 fail-fast, 不发起请求 (否则会构造非法 Range 头 bytes=0--1)
	err := fetchAudioSample(context.Background(), "http://127.0.0.1:1/a.mp3", 0, time.Second)
	require.ErrorContains(t, err, "limit must be positive")
}

func TestFansGroupValidate(t *testing.T) {
	c := NewFansGroup(&Root{}, nil)

	// --group-id 必须为非空纯数字 (AC-006)
	c.opts.GroupID = []string{"abc"}
	require.ErrorContains(t, c.validate(), "numeric")
	c.opts.GroupID = []string{"123", ""}
	require.ErrorContains(t, c.validate(), "numeric")
	c.opts.GroupID = []string{"123"}

	// status 与写 flag 互斥 (AC-004)
	c.status = true
	c.opts.Delete = true
	require.ErrorContains(t, c.validate(), "read-only")
	c.opts.Delete = false
	c.opts.Title = "x"
	require.ErrorContains(t, c.validate(), "read-only")
	c.opts.Title = ""
	c.opts.Message = "x"
	require.ErrorContains(t, c.validate(), "read-only")
	c.opts.Message = ""
	c.opts.Image = "whatever.png"
	require.ErrorContains(t, c.validate(), "read-only")
	c.opts.Image = ""

	// status 与 --group-id 兼容 (AC-005)
	require.NoError(t, c.validate())

	// 任务模式: --title TrimSpace 后非空
	c.status = false
	c.opts.Title = "   "
	require.ErrorContains(t, c.validate(), "title")
	c.opts.Title = ""

	// --message TrimSpace 后 >=10 Unicode 字符
	c.opts.Message = "一二三四五六七八九"
	require.ErrorContains(t, c.validate(), "10 Unicode")
	c.opts.Message = "一二三四五六七八九十"
	require.NoError(t, c.validate())
	c.opts.Message = ""

	// --image 非符号链接、非空常规文件
	dir := t.TempDir()
	target := filepath.Join(dir, "real.img")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))

	link := filepath.Join(dir, "link.img")
	require.NoError(t, os.Symlink(target, link))
	c.opts.Image = link
	require.ErrorContains(t, c.validate(), "symlink")
	c.opts.Image = filepath.Join(dir, "empty.img")
	require.NoError(t, os.WriteFile(c.opts.Image, nil, 0o600))
	require.Error(t, c.validate())
	c.opts.Image = target
	require.NoError(t, c.validate())
}

func TestTaskSelectionFansGroup(t *testing.T) {
	c := NewTask(&Root{}, nil)

	// 空选择报错信息含 --fansgroup (AC-027)
	_, err := c.taskSelection()
	require.ErrorContains(t, err, "--fansgroup")

	// --fansgroup 单选 (AC-024)
	c.opts.FansGroup = true
	sel, err := c.taskSelection()
	require.NoError(t, err)
	assert.True(t, sel.FansGroup)
	assert.False(t, sel.SignIn)
	assert.False(t, sel.SongShare)

	// --runAll 五项全选 (AC-026)
	c.opts.RunAll = true
	sel, err = c.taskSelection()
	require.NoError(t, err)
	assert.True(t, sel.SignIn && sel.Partner && sel.Scrobble && sel.SongShare && sel.FansGroup)
}

func TestTaskValidateFansGroup(t *testing.T) {
	c := NewTask(&Root{}, nil)
	c.opts.FansGroup = true

	// 非法 --fansgroup.cron 在启动 cron 前失败 (AC-028)
	c.opts.FansGroupOptsCrontab = "not-a-cron"
	require.ErrorContains(t, c.validate(), "ParseStandard")

	// 合法 cron + 非法 group-id
	c.opts.FansGroupOptsCrontab = "30 10 * * *"
	c.opts.GroupID = []string{"abc"}
	require.ErrorContains(t, c.validate(), "numeric")

	// 全部合法
	c.opts.GroupID = []string{"1872529203038486609"}
	require.NoError(t, c.validate())
}

func TestIsNumericString(t *testing.T) {
	assert.True(t, isNumericString("1872529203038486609"))
	assert.False(t, isNumericString(""))
	assert.False(t, isNumericString("12a"))
	assert.False(t, isNumericString("-1"))
	assert.False(t, isNumericString(" 1"))
}

func TestToInt64SongIDs(t *testing.T) {
	assert.Equal(t, []int64{1, 2}, toInt64SongIDs([]string{"1", "2"}))
	assert.Empty(t, toInt64SongIDs([]string{"abc", ""}))
	assert.Empty(t, toInt64SongIDs(nil))

	// 非正数 ID 跳过, 不进入播放/红心链路
	assert.Equal(t, []int64{7}, toInt64SongIDs([]string{"-5", "7"}))
	assert.Empty(t, toInt64SongIDs([]string{"-5", "0"}))
}
