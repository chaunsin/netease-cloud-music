// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

// Upload 工具函数 — NOS 文件上传 (内部使用, 不对外暴露)
// 提供 getUploadNode 和 rawUpload 方法供 event.go 使用

package eapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// uploadLbsResp 上传节点 LBS 响应
type uploadLbsResp struct {
	Upload []string `json:"upload"`
}

// GetUploadNode 获取 NOS 上传节点地址
// 接口: http://wanproxy.127.net/lbs?version=1.0&bucketname=<bucket>
// 返回一个可用的上传节点 URL
func (a *Api) GetUploadNode(ctx context.Context, bucket string) (string, error) {
	url := fmt.Sprintf("http://wanproxy.127.net/lbs?version=1.0&bucketname=%s", bucket)
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
		return "", fmt.Errorf("no upload nodes available")
	}
	return lbs.Upload[0], nil
}

// rawUpload 原始文件上传 (PUT)
// 将文件字节以 PUT 方式上传到 NOS 存储
// 注意: 不使用 client.Upload() 因为它强制使用 POST
func (a *Api) rawUpload(ctx context.Context, url string, headers map[string]string, data []byte) error {
	resp, err := a.client.NewRequest().
		SetContext(ctx).
		SetHeaders(headers).
		SetBody(data).
		Put(url)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", url, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("PUT %s returned status %d: %s", url, resp.StatusCode(), string(resp.Body()))
	}
	return nil
}
