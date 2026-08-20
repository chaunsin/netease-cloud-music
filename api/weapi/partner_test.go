// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

func TestPartnerWeek(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerWeek(t.Context(), &PartnerWeekReq{Period: "MMD-1617552000000-37-1"})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerPeriod(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerPeriod(t.Context(), &PartnerPeriodReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerPeriodUserinfo(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerUserinfo(t.Context(), &PartnerUserinfoReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerLatest(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerLatest(t.Context(), &PartnerLatestReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerHome(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerHome(t.Context(), &PartnerHomeReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerTask(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerDailyTask(t.Context(), &PartnerTaskReq{})
	require.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerEvaluate(t *testing.T) {
	cli := newLiveWEAPI(t)

	resp, err := cli.PartnerEvaluate(t.Context(), &PartnerEvaluateReq{
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
