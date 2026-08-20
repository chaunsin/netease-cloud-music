// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

// Upload 工具函数 — NOS 文件上传 (内部使用, 不对外暴露)
// 提供 getUploadNode 和 rawUpload 方法供 event.go 使用

package eapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// uploadLbsResp 上传节点 LBS 响应.
type uploadLbsResp struct {
	Upload []string `json:"upload"`
}

// GetUploadNode 获取 NOS 上传节点地址
// 接口: http://wanproxy.127.net/lbs?version=1.0&bucketname=<bucket>
// 返回一个可用的上传节点 URL.
func (a *Api) GetUploadNode(ctx context.Context, bucket string) (string, error) {
	url := "http://wanproxy.127.net/lbs?version=1.0&bucketname=" + bucket

	resp, err := a.client.
		NewRequest().
		SetContext(ctx).
		Get(url)
	if err != nil {
		return "", fmt.Errorf("request lbs: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("lbs returned status %d", resp.StatusCode())
	}

	var lbs uploadLbsResp
	if err := json.Unmarshal(resp.Body(), &lbs); err != nil {
		return "", fmt.Errorf("parse lbs: %w", err)
	}

	if len(lbs.Upload) == 0 {
		return "", errors.New("no upload nodes available")
	}
	return lbs.Upload[0], nil
}
