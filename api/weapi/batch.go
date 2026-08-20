// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Headers 自定义 Headers 数据类型 (仅对于非 eapi 有效).
type Headers []struct {
	Name  string
	Value string
}

// RequestData 传入请求数据类型.
type RequestData struct {
	Cookies []*http.Cookie
	Headers Headers
	Body    string
}

// EapiOption eapi 请求所需要的参数.
type EapiOption struct {
	Json string
	Path string
	Url  string
}

// Batch 批处理 APi.
type Batch struct {
	API    map[string]any
	Result string
	Header http.Header
	Error  error
}

// BatchAPI 被批处理的 API.
type BatchAPI struct {
	Key  string
	Json string
}

// NewBatch 新建 Batch 对象.
// url: testdata/har/12.har
func NewBatch(apis ...BatchAPI) *Batch {
	b := &Batch{
		API: make(map[string]any),
	}
	for _, api := range apis {
		b.API[api.Key] = api.Json
	}
	return b
}

// Add 添加 API.
func (b *Batch) Add(apis ...BatchAPI) *Batch {
	for _, api := range apis {
		b.API[api.Key] = api.Json
	}
	return b
}

// Do 请求批处理 API.
func (b *Batch) Do(_ RequestData) *Batch {
	b.Error = errors.New("batch request is not implemented")
	return b
}

// Parse 解析 Batch 的 Json 数据.
func (b *Batch) Parse() (*Batch, map[string]string) {
	jsonData := make(map[string]any)
	jsonMap := make(map[string]string)

	if err := json.Unmarshal([]byte(b.Result), &jsonData); err != nil {
		b.Error = fmt.Errorf("parse batch json error: %w", err)
	}

	for k, v := range jsonData {
		jsonStr, err := json.Marshal(v)
		if err != nil {
			b.Error = fmt.Errorf("marshal batch result %q: %w", k, err)
			return b, nil
		}

		jsonMap[k] = string(jsonStr)
	}
	return b, jsonMap
}
