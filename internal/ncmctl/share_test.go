// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
)

func TestDailySongShareValidateMessageAndDelete(t *testing.T) {
	c := NewDailySongShare(&Root{}, nil)
	c.opts.Message = "一二三四五六七八九"
	require.ErrorContains(t, c.validateFlags(true), "10 Unicode")

	c.opts.Message = "一二三四五六七八九十"
	c.opts.Delete = true
	c.opts.Draw = false
	assert.ErrorContains(t, c.validateFlags(true), "draw enabled")
}

func TestClassifyDailySongShareGuide(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		already         bool
		activity, cycle int64
		want            dailySongState
	}{
		{"completed", "registered", true, 1, 1, stateCompleted},
		{"not registered NOREGISTER", "NOREGISTER", false, 1, 1, stateNotRegistered},
		{"registered REGISTER", "REGISTER", false, 1, 1, stateIsRegister},
		{"unknown status", "其他", false, 1, 1, dailySongState("unknow status: 其他")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &eapi.DailySongShareRegistrationGuideResp{}
			g.Data.RegisterStatus = tt.status
			g.Data.ActivityId, g.Data.ActivityCycleId = tt.activity, tt.cycle
			g.Data.RegisteredGuide.AlreadyPubEvent = tt.already
			assert.Equal(t, tt.want, classifyGuide(g))
		})
	}
}
