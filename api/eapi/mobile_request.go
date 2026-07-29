// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// todo: 考虑迁移到types包中得EApiReqCommon或api包中得Options

// DailySongShareBaseReq contains the common encrypted mobile request fields.
type DailySongShareBaseReq struct {
	Header         any    `json:"header"` // eg: '{}'
	ER             bool   `json:"e_r"`
	DeviceId       string `json:"deviceId,omitempty"`
	OS             string `json:"os,omitempty"`
	VerifyId       int    `json:"verifyId,omitempty"`
	AntiCheatToken string `json:"X-antiCheatToken,omitempty"`
}

func (r *DailySongShareBaseReq) fill() {
	r.ER = true

	if r.mobileHeaderEmpty() {
		r.Header = "{}"
	}
}

func (r *DailySongShareBaseReq) mobileHeaderEmpty() bool {
	switch value := r.Header.(type) {
	case nil:
		return true
	case string:
		value = strings.TrimSpace(value)
		return value == "" || value == "{}"
	case map[string]any:
		return len(value) == 0
	default:
		return false
	}
}

func randomDailySongSessionID() (string, error) {
	value, err := randomDailySongHex(6, false)
	if err != nil {
		return "", err
	}
	return value[:8] + "-" + value[8:11], nil
}

func randomDailySongHex(size int, upper bool) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate daily song random ID: %w", err)
	}

	value := hex.EncodeToString(data)
	if upper {
		value = strings.ToUpper(value)
	}
	return value, nil
}
