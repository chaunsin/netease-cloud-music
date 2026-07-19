// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

func TestPartnerWeek(t *testing.T) {
	resp, err := cli.PartnerWeek(ctx, &PartnerWeekReq{Period: "MMD-1617552000000-37-1"})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerPeriod(t *testing.T) {
	resp, err := cli.PartnerPeriod(ctx, &PartnerPeriodReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerPeriodUserinfo(t *testing.T) {
	resp, err := cli.PartnerUserinfo(ctx, &PartnerUserinfoReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerLatest(t *testing.T) {
	resp, err := cli.PartnerLatest(ctx, &PartnerLatestReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerHome(t *testing.T) {
	resp, err := cli.PartnerHome(ctx, &PartnerHomeReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerTask(t *testing.T) {
	resp, err := cli.PartnerDailyTask(ctx, &PartnerTaskReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerEvaluate(t *testing.T) {
	resp, err := cli.PartnerEvaluate(ctx, &PartnerEvaluateReq{
		ReqCommon:     types.ReqCommon{CSRFToken: ""},
		TaskId:        "101398359",
		WorkId:        "1328062",
		Score:         "3",
		Tags:          ThreeDOnePartnerTags,
		CustomTags:    "",
		Comment:       "",
		SyncYunCircle: false,
		SyncComment:   true,
		Source:        "mp-music-partner",
	})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}
