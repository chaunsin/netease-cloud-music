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

package weapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPartnerPeriod(t *testing.T) {
	resp, err := cli.PartnerPeriod(ctx, &PartnerPeriodReq{})
	assert.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerPeriodUserinfo(t *testing.T) {
	resp, err := cli.PartnerPeriodUserinfo(ctx, &PartnerPeriodUserinfoReq{})
	assert.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerLatest(t *testing.T) {
	resp, err := cli.PartnerLatest(ctx, &PartnerLatestReq{})
	assert.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerHome(t *testing.T) {
	resp, err := cli.PartnerHome(ctx, &PartnerHomeReq{})
	assert.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerTask(t *testing.T) {
	resp, err := cli.PartnerTask(ctx, &PartnerTaskReq{})
	assert.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}

func TestPartnerEvaluate(t *testing.T) {
	resp, err := cli.PartnerEvaluate(ctx, &PartnerEvaluateReq{
		CsrfToken:     "",
		TaskId:        0,
		WorkId:        0,
		Score:         3,
		Tags:          "",
		CustomTags:    "",
		Comment:       "",
		SyncYunCircle: false,
		Source:        "",
	})
	assert.NoError(t, err)
	t.Logf("resp: %+v\n", resp)
}
